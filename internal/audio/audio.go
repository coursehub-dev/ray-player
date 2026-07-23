package audio

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ray-player1/internal/appstate"
	"ray-player1/internal/events"
	"ray-player1/internal/library"
	"ray-player1/internal/logx"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/speaker"
)

var audioLog = logx.New("audio")

type PlaybackEndReason string

const (
	PlaybackEndNatural      PlaybackEndReason = "natural_end"
	PlaybackEndUserSkip     PlaybackEndReason = "user_skip"
	PlaybackEndDecodeFailed PlaybackEndReason = "decode_failed"
	PlaybackEndStreamError  PlaybackEndReason = "stream_error"
	PlaybackEndEmptyStream  PlaybackEndReason = "empty_stream"
	PlaybackEndInterrupted  PlaybackEndReason = "interrupted"
	PlaybackEndSeekFailed   PlaybackEndReason = "seek_failed"
)

type Service struct {
	mu             sync.Mutex
	state          *appstate.Store
	events         *events.Service
	onEnded        func(library.Track, PlaybackEndReason)
	onStarted      func(library.Track, string)
	initialized    bool
	mixer          *beep.Mixer
	ctrl           *beep.Ctrl
	volume         *effects.Volume
	current        *playbackStream
	format         beep.Format
	commonSR       beep.SampleRate
	currentPath    string
	currentTrack   library.Track
	currentBackend string
	playToken      uint64
	seekGeneration uint64

	normalizePodcasts bool

	preparedMu sync.Mutex
	prepared   *preparedPlayback
	prepareSeq uint64
}

type preparedPlayback struct {
	track   library.Track
	stream  beep.StreamSeekCloser
	format  beep.Format
	closer  io.Closer
	backend string
}

type playbackStream struct {
	mu             sync.Mutex
	owner          *Service
	token          uint64
	stream         beep.StreamSeekCloser
	closed         bool
	samplesPlayed  int64
	firstSampleAt  time.Time
	err            error
	ended          bool
	started        bool
	queuedAt       time.Time
	seekGeneration uint64
}

func (p *playbackStream) Stream(samples [][2]float64) (n int, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			p.mu.Lock()
			p.err = fmt.Errorf("stream panic: %v", r)
			p.mu.Unlock()
			n = 0
			ok = false
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || atomic.LoadUint64(&p.owner.playToken) != p.token {
		return 0, false
	}

	n, ok = p.stream.Stream(samples)
	if n > 0 {
		p.samplesPlayed += int64(n)

		if p.firstSampleAt.IsZero() {
			p.firstSampleAt = time.Now()

			p.owner.mu.Lock()
			track := p.owner.currentTrack
			backend := p.owner.currentBackend
			onStarted := p.owner.onStarted
			p.owner.mu.Unlock()

			audioLog.I(
				"first samples path=%q track=%s backend=%s waitMs=%d samples=%d",
				track.Path,
				track.ID,
				backend,
				time.Since(p.queuedAt).Milliseconds(),
				n,
			)

			if !p.started {
				p.started = true
				if onStarted != nil {
					go onStarted(track, backend)
				}
			}
		}
	}

	if !ok {
		currentSeekGeneration := atomic.LoadUint64(
			&p.owner.seekGeneration,
		)
		if currentSeekGeneration != p.seekGeneration {
			p.seekGeneration = currentSeekGeneration
			return n, true
		}

		p.ended = true
		if err := p.stream.Err(); err != nil {
			p.err = err
			audioLog.I("stream error token=%d path=%q samples=%d err=%v", p.token, p.owner.currentPath, p.samplesPlayed, err)
		}
		return n, false
	}

	if atomic.LoadUint64(&p.owner.playToken) != p.token {
		return n, false
	}
	return n, true
}

func (p *playbackStream) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	if p.closed {
		return nil
	}
	return p.stream.Err()
}

func (p *playbackStream) Stats() (samples int64, firstSampleAt time.Time, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.samplesPlayed, p.firstSampleAt, p.err
}

func (p *playbackStream) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0
	}
	return p.stream.Len()
}

func (p *playbackStream) Position() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0
	}
	return p.stream.Position()
}

func (p *playbackStream) Seek(pos int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	return p.stream.Seek(pos)
}

func (p *playbackStream) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	stream := p.stream
	p.mu.Unlock()
	return stream.Close()
}

func NewService(state *appstate.Store, events *events.Service) *Service {
	return &Service{
		state:    state,
		events:   events,
		commonSR: playbackSampleRate,
		mixer:    &beep.Mixer{},
	}
}

func (s *Service) SetOnEnded(fn func(library.Track, PlaybackEndReason)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEnded = fn
}

func (s *Service) SetOnStarted(
	fn func(library.Track, string),
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onStarted = fn
}

func (s *Service) Warmup() error {
	s.mu.Lock()
	if s.initialized {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	if err := speaker.Init(
		s.commonSR,
		s.commonSR.N(time.Second/8),
	); err != nil {
		return err
	}

	speaker.Play(s.mixer)

	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	audioLog.I(
		"speaker warmup ok sampleRate=%d bufferMs=%d",
		s.commonSR,
		125,
	)
	return nil
}

func (s *Service) Prepare(track library.Track) {
	if strings.TrimSpace(track.Path) == "" {
		return
	}

	s.preparedMu.Lock()
	s.prepareSeq++
	seq := s.prepareSeq
	s.preparedMu.Unlock()

	go func() {
		startedAt := time.Now()

		stream, format, closer, backend, err := safeDecode(
			func() (
				beep.StreamSeekCloser,
				beep.Format,
				io.Closer,
				string,
				error,
			) {
				return DecodeForPlayback(track.Path)
			},
		)
		if err != nil {
			audioLog.I(
				"prepare failed track=%s path=%q err=%v",
				track.ID,
				track.Path,
				err,
			)
			return
		}
		if closer == nil {
			closer = stream
		}

		next := &preparedPlayback{
			track:   track,
			stream:  stream,
			format:  format,
			closer:  closer,
			backend: backend,
		}

		s.preparedMu.Lock()
		if seq != s.prepareSeq {
			s.preparedMu.Unlock()
			_ = closer.Close()
			return
		}

		previous := s.prepared
		s.prepared = next
		s.preparedMu.Unlock()

		if previous != nil && previous.closer != nil {
			_ = previous.closer.Close()
		}

		audioLog.I(
			"prepare ok track=%s backend=%s ms=%d",
			track.ID,
			backend,
			time.Since(startedAt).Milliseconds(),
		)
	}()
}

func (s *Service) takePrepared(
	track library.Track,
) (
	beep.StreamSeekCloser,
	beep.Format,
	io.Closer,
	string,
	bool,
) {
	s.preparedMu.Lock()
	defer s.preparedMu.Unlock()

	if s.prepared == nil ||
		s.prepared.track.ID != track.ID ||
		s.prepared.track.Path != track.Path {
		return nil, beep.Format{}, nil, "", false
	}

	prepared := s.prepared
	s.prepared = nil

	return prepared.stream,
		prepared.format,
		prepared.closer,
		prepared.backend,
		true
}

func (s *Service) Play(track library.Track) error {
	return s.playWithOptions(track, false, false)
}

func (s *Service) PlayFresh(track library.Track) error {
	return s.playWithOptions(track, true, false)
}

func (s *Service) play(track library.Track, restart bool) error {
	return s.playWithOptions(track, restart, false)
}

func (s *Service) playWithOptions(track library.Track, restart bool, normalize bool) error {
	audioLog.I("open start track=%s title=%q path=%s restart=%t", track.ID, track.Title, track.Path, restart)

	s.mu.Lock()
	if !restart && s.currentPath == track.Path && s.ctrl != nil {
		ctrl := s.ctrl
		volume := s.volume
		s.currentTrack = track
		s.mu.Unlock()
		speaker.Lock()
		ctrl.Paused = false
		if volume != nil {
			volume.Silent = false
		}
		speaker.Unlock()
		return nil
	}
	oldCurrent := s.current
	initialized := s.initialized
	s.mu.Unlock()

	if _, err := os.Stat(track.Path); err != nil {
		audioLog.I("file stat failed path=%s err=%v", track.Path, err)
		return err
	}

	openStartedAt := time.Now()

	stream, format, closer, backend, prepared :=
		s.takePrepared(track)

	var err error
	if !prepared {
		stream, format, closer, backend, err =
			safeDecode(func() (
				beep.StreamSeekCloser,
				beep.Format,
				io.Closer,
				string,
				error,
			) {
				return DecodeForPlaybackWithOptions(track.Path, normalize)
			})
	} else {
		audioLog.I(
			"prepared stream hit track=%s backend=%s",
			track.ID,
			backend,
		)
	}
	if err != nil {
		audioLog.I("decode failed path=%q track=%s err=%v", track.Path, track.ID, err)
		return err
	}
	if closer == nil {
		closer = stream
	}

	token := atomic.AddUint64(&s.playToken, 1)

	if token != atomic.LoadUint64(&s.playToken) {
		_ = closer.Close()
		return nil
	}

	audioLog.I(
		"decode ok path=%q track=%s backend=%s openMs=%d sr=%d channels=%d precision=%d len=%d",
		track.Path, track.ID, backend,
		time.Since(openStartedAt).Milliseconds(),
		format.SampleRate, format.NumChannels, format.Precision, stream.Len(),
	)

	if !initialized {
		if err := s.Warmup(); err != nil {
			_ = closer.Close()
			return err
		}
	}

	guarded := &playbackStream{
		owner:    s,
		token:    token,
		stream:   stream,
		queuedAt: time.Now(),
		seekGeneration: atomic.LoadUint64(
			&s.seekGeneration,
		),
	}
	var str beep.Streamer = guarded
	if format.SampleRate != s.commonSR {
		str = beep.Resample(3, format.SampleRate, s.commonSR, str)
	}
	ctrl := &beep.Ctrl{Streamer: str, Paused: false}
	volume := &effects.Volume{Streamer: ctrl, Base: 2, Volume: volumeToGain(s.state.Get().Volume), Silent: false}
	streamer := beep.Seq(volume, beep.Callback(func() {
		go s.handlePlaybackEnded(track, token, guarded)
	}))

	s.mu.Lock()
	if token != atomic.LoadUint64(&s.playToken) {
		s.mu.Unlock()
		_ = closer.Close()
		return nil
	}
	s.ctrl = ctrl
	s.volume = volume
	s.current = guarded
	s.format = format
	s.currentPath = track.Path
	s.currentTrack = track
	s.currentBackend = backend
	s.mu.Unlock()

	if oldCurrent != nil {
		_ = oldCurrent.Close()
	}

	audioLog.I(
		"playback queued track=%s path=%q backend=%s sr=%d len=%d",
		track.ID,
		track.Path,
		backend,
		format.SampleRate,
		stream.Len(),
	)

	speaker.Lock()
	s.mixer.Add(streamer)
	speaker.Unlock()
	return nil
}

func (s *Service) handlePlaybackEnded(track library.Track, token uint64, ps *playbackStream) {
	samples, firstSampleAt, streamErr := ps.Stats()

	s.mu.Lock()
	currentToken := atomic.LoadUint64(&s.playToken)
	format := s.format
	path := s.currentPath
	backend := s.currentBackend
	onEnded := s.onEnded
	if token != currentToken || path != track.Path {
		s.mu.Unlock()
		return
	}
	s.current = nil
	s.ctrl = nil
	s.volume = nil
	s.currentPath = ""
	s.currentTrack = library.Track{}
	s.currentBackend = ""
	s.mu.Unlock()

	playedMs := 0
	if format.SampleRate > 0 {
		playedMs = int((time.Second * time.Duration(samples) / time.Duration(format.SampleRate)) / time.Millisecond)
	}

	reason := classifyPlaybackEnd(samples, playedMs, !firstSampleAt.IsZero(), streamErr != nil)

	audioLog.I(
		"playback ended path=%q track=%s reason=%s playedMs=%d samples=%d streamErr=%v backend=%s",
		track.Path, track.ID, reason, playedMs, samples, streamErr, backend,
	)

	if reason == PlaybackEndNatural {
		_ = s.events.MarkComplete(track)
		if onEnded != nil {
			go onEnded(track, reason)
		}
		return
	}

	audioLog.I(
		"technical skip track=%s path=%q reason=%s backend=%s playedMs=%d err=%v",
		track.ID, track.Path, reason, backend, playedMs, streamErr,
	)
	_ = s.events.MarkTechnicalSkip(track, string(reason), playedMs)
	if streamErr != nil {
		_ = s.events.MarkPlaybackFailed(track, string(reason), streamErr, playedMs)
	} else {
		_ = s.events.MarkPlaybackFailed(track, string(reason), nil, playedMs)
	}
	if onEnded != nil {
		go onEnded(track, reason)
	}
}

func (s *Service) TogglePause() bool {
	s.mu.Lock()
	ctrl := s.ctrl
	s.mu.Unlock()
	if ctrl == nil {
		return s.state.Get().Playing
	}
	speaker.Lock()
	ctrl.Paused = !ctrl.Paused
	speaker.Unlock()
	return !ctrl.Paused
}

func (s *Service) HasActiveStream() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.current != nil && s.ctrl != nil
}

func (s *Service) Pause() bool {
	s.mu.Lock()
	control := s.ctrl
	s.mu.Unlock()
	if control == nil {
		return false
	}

	speaker.Lock()
	control.Paused = true
	speaker.Unlock()

	return true
}

func (s *Service) Resume() bool {
	s.mu.Lock()
	control := s.ctrl
	s.mu.Unlock()
	if control == nil {
		return false
	}

	speaker.Lock()
	control.Paused = false
	speaker.Unlock()

	return true
}

func (s *Service) SetVolume(v float64) {
	s.mu.Lock()
	vol := s.volume
	s.mu.Unlock()
	if vol == nil {
		return
	}

	// Плавный рамп ~35 ms: 10 шагов по ~3.5 ms.
	// Убирает щелчок при резком mute/unmute.
	const steps = 10
	const stepDelay = 3500 * time.Microsecond

	speaker.Lock()
	startGain := vol.Volume
	speaker.Unlock()

	targetGain := volumeToGain(v)
	targetSilent := v <= 0.001

	if startGain == targetGain {
		speaker.Lock()
		vol.Silent = targetSilent
		speaker.Unlock()
		return
	}

	go func() {
		for i := 1; i <= steps; i++ {
			frac := float64(i) / float64(steps)
			gain := startGain + (targetGain-startGain)*frac
			silent := i == steps && targetSilent

			s.mu.Lock()
			currentVol := s.volume
			s.mu.Unlock()
			if currentVol != vol {
				// Трек сменился — прекращаем рамп.
				return
			}

			speaker.Lock()
			vol.Volume = gain
			if silent {
				vol.Silent = true
			} else if i == 1 {
				vol.Silent = false
			}
			speaker.Unlock()

			time.Sleep(stepDelay)
		}
	}()
}

// SetPodcastNormalization включает/выключает FFmpeg audio filter для подкастов.
func (s *Service) SetPodcastNormalization(enabled bool) {
	s.mu.Lock()
	s.normalizePodcasts = enabled
	s.mu.Unlock()
}

// PodcastNormalizationEnabled возвращает текущее состояние нормализации.
func (s *Service) PodcastNormalizationEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.normalizePodcasts
}

// PlayFreshPodcast запускает подкаст с учётом настройки нормализации.
func (s *Service) PlayFreshPodcast(track library.Track) error {
	var normalize bool
	s.mu.Lock()
	normalize = s.normalizePodcasts
	s.mu.Unlock()
	return s.playWithOptions(track, true, normalize)
}

func (s *Service) Seek(positionMs int) error {
	s.mu.Lock()
	current := s.current
	format := s.format
	path := s.currentPath
	s.mu.Unlock()

	audioLog.I("seek request path=%s positionMs=%d", path, positionMs)
	if current == nil {
		return nil
	}
	if format.SampleRate <= 0 {
		return errors.New("cannot seek: invalid sample rate")
	}

	samples := format.SampleRate.N(time.Duration(positionMs) * time.Millisecond)

	atomic.AddUint64(&s.seekGeneration, 1)

	speaker.Lock()
	err := current.Seek(samples)
	speaker.Unlock()

	if err != nil {
		audioLog.I(
			"seek failed path=%q positionMs=%d err=%v",
			path,
			positionMs,
			err,
		)
		return err
	}

	current.mu.Lock()
	current.seekGeneration = atomic.LoadUint64(&s.seekGeneration)
	current.mu.Unlock()

	audioLog.I(
		"seek applied path=%q positionMs=%d samples=%d",
		path,
		positionMs,
		samples,
	)
	return err
}

func (s *Service) GetPositionMs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil || s.format.SampleRate <= 0 {
		return 0
	}
	return int((time.Second * time.Duration(s.current.Position()) / time.Duration(s.format.SampleRate)) / time.Millisecond)
}

func (s *Service) GetDurationMs() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil || s.format.SampleRate <= 0 {
		return 0
	}
	length := s.current.Len()
	if length <= 0 {
		return 0
	}
	return int(time.Second * time.Duration(length) /
		time.Duration(s.format.SampleRate) / time.Millisecond)
}

func volumeToGain(v float64) float64 {
	if v <= 0.001 {
		return -8
	}
	return (v - 1) * 4
}
