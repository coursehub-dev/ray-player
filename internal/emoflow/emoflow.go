package emoflow

import (
	"fmt"
	"math"
	"strings"

	"ray-player1/internal/library"
	"ray-player1/internal/rays"
)

type Vector struct {
	Energy             float64 `json:"energy"`
	Valence            float64 `json:"valence"`
	Brightness         float64 `json:"brightness"`
	Darkness           float64 `json:"darkness"`
	Calmness           float64 `json:"calmness"`
	Aggression         float64 `json:"aggression"`
	Movement           float64 `json:"movement"`
	TempoBPM           float64 `json:"tempoBpm"`
	TempoConfidence    float64 `json:"tempoConfidence"`
	RhythmicPulse      float64 `json:"rhythmicPulse"`
	Drive              float64 `json:"drive"`
	Melancholy         float64 `json:"melancholy"`
	Intensity          float64 `json:"intensity"`
	Pulse              float64 `json:"pulse"`
	ClubPressure       float64 `json:"clubPressure"`
	MechanicalPressure float64 `json:"mechanicalPressure"`
	Atmosphere         float64 `json:"atmosphere"`
	Melodicness        float64 `json:"melodicness"`
	Softness           float64 `json:"softness"`
	Heaviness          float64 `json:"heaviness"`
	Dreaminess         float64 `json:"dreaminess"`
	Acousticness       float64 `json:"acousticness"`
	Electronicness     float64 `json:"electronicness"`
	Instrumentalness   float64 `json:"instrumentalness"`
	Vocalness          float64 `json:"vocalness"`
	TimbreBrightness   float64 `json:"timbreBrightness"`
	Tonality           float64 `json:"tonality"`
	Approachability    float64 `json:"approachability"`
	Engagement         float64 `json:"engagement"`
}

type Direction string

const (
	Stable    Direction = "stable"
	WarmUp    Direction = "warm_up"
	CoolDown  Direction = "cool_down"
	Darken    Direction = "darken"
	Brighten  Direction = "brighten"
	Deepen    Direction = "deepen"
	Intensify Direction = "intensify"
	Explore   Direction = "explore"
)

type Palette struct {
	Accent     string `json:"accent"`
	AccentSoft string `json:"accentSoft"`
	AccentHot  string `json:"accentHot"`
	Background string `json:"background"`
	Surface    string `json:"surface"`
	Glow       string `json:"glow"`
	GlowSoft   string `json:"glowSoft"`
	Ring       string `json:"ring"`
	Progress   string `json:"progress"`
	Icon       string `json:"icon"`
	AccentOn   string `json:"accentOn"`
}

type TrackState struct {
	TrackID   string    `json:"trackId"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist"`
	Vector    Vector    `json:"vector"`
	Palette   Palette   `json:"palette"`
	Intensity float64   `json:"intensity"`
	Heat      float64   `json:"heat"`
	Cool      float64   `json:"cool"`
	Tension   float64   `json:"tension"`
	Reason    string    `json:"reason"`
	Dominant  string    `json:"dominant"`
	Secondary string    `json:"secondary"`
	Direction Direction `json:"direction"`
}

type TransitionVisualState struct {
	MoodDistance float64 `json:"moodDistance"`
	EnergyDelta  float64 `json:"energyDelta"`
	AggroDelta   float64 `json:"aggroDelta"`
	Direction    string  `json:"direction"`
	Reason       string  `json:"reason"`
}

type UIState struct {
	TrackID    string                `json:"trackId"`
	Current    TrackState            `json:"current"`
	Previous   *TrackState           `json:"previous,omitempty"`
	Next       *TrackState           `json:"next,omitempty"`
	Vector     Vector                `json:"vector"`
	Direction  Direction             `json:"direction"`
	Intensity  float64               `json:"intensity"`
	Heat       float64               `json:"heat"`
	Cool       float64               `json:"cool"`
	Tension    float64               `json:"tension"`
	Palette    Palette               `json:"palette"`
	Reason     string                `json:"reason"`
	Transition TransitionVisualState `json:"transition"`
}

type UISettings struct {
	Enabled              bool    `json:"enabled"`
	Intensity            float64 `json:"intensity"`
	AnimateDuringTrack   bool    `json:"animateDuringTrack"`
	RespectReducedMotion bool    `json:"respectReducedMotion"`
}

type oklchColor struct {
	L float64
	C float64
	H float64
}

type weightedMood struct {
	Key    string
	Weight float64
}

type EmoFlowConfig struct {
	UsePerceptualBasisV3 bool
}

type PerceptualBasis struct {
	Motion      float64 `json:"motion"`
	Density     float64 `json:"density"`
	Roughness   float64 `json:"roughness"`
	Brightness  float64 `json:"brightness"`
	Smoothness  float64 `json:"smoothness"`
	Impact      float64 `json:"impact"`
	Pressure    float64 `json:"pressure"`
	Joy         float64 `json:"joy"`
	Melancholy  float64 `json:"melancholy"`
	Serenity    float64 `json:"serenity"`
	Swagger     float64 `json:"swagger"`
	Combat      float64 `json:"combat"`
	Sprint      float64 `json:"sprint"`
	SprintClean float64 `json:"sprintClean"`
	Label       string  `json:"label"`
}

type MoodBasis struct {
	Motion   float64
	Pressure float64
	Calmness float64
	Coolness float64
	Warmth   float64
	Texture  float64
	Edge     float64
	Valence  float64
	Vocality float64
}

var moodAnchors = map[string]oklchColor{
	"happy":      {L: 0.76, C: 0.20, H: 48},
	"party":      {L: 0.70, C: 0.22, H: 32},
	"aggressive": {L: 0.63, C: 0.24, H: 22},
	"sad":        {L: 0.61, C: 0.18, H: 272},
	"relaxed":    {L: 0.68, C: 0.16, H: 225},
	"acoustic":   {L: 0.70, C: 0.13, H: 250},
	"electronic": {L: 0.68, C: 0.22, H: 220},
	"dreamy":     {L: 0.70, C: 0.20, H: 300},
	"dark":       {L: 0.55, C: 0.18, H: 255},
	"soft":       {L: 0.74, C: 0.12, H: 288},
	"default":    {L: 0.62, C: 0.08, H: 230},
}

func DefaultSettings() UISettings {
	return UISettings{Enabled: true, Intensity: 1.0, AnimateDuringTrack: true, RespectReducedMotion: true}
}

func NormalizeSettings(s UISettings) UISettings {
	s.Intensity = clamp01(s.Intensity)
	if s.Intensity == 0 && !s.Enabled {
		return s
	}
	if s.Intensity == 0 {
		s.Intensity = 1.0
	}
	return s
}

func FromTrack(t library.Track) Vector {
	tempoPulse := tempoToPulse(t.BPMPerceived, t.TempoConfidence)
	movement := clamp01(0.46*t.Danceability + 0.20*tempoPulse + 0.14*t.Party + 0.10*t.Electronicness + 0.10*(1.0-t.Softness))
	clubPressure := clamp01(0.36*t.Danceability + 0.22*t.Party + 0.16*t.Vocalness + 0.12*tempoPulse + 0.08*(1.0-t.Softness) + 0.06*t.Energy)
	mechanicalPressure := clamp01(0.34*t.Electronicness + 0.24*t.Danceability + 0.16*tempoPulse + 0.12*(1.0-t.Softness) + 0.08*t.TimbreBrightness + 0.06*t.Instrumentalness)
	roughAudio := clamp01(0.38*clamp01(t.ZeroCrossingRate/0.22) + 0.28*clamp01(t.SpectralFlatness/0.08) + 0.20*clamp01(t.SpectralFlux/0.45) + 0.14*clamp01(t.OnsetRate/3.0))
	drive := clamp01(0.36*movement + 0.18*clubPressure + 0.18*mechanicalPressure + 0.16*t.Energy + 0.12*tempoPulse)
	intensity := clamp01(0.30*drive + 0.24*mechanicalPressure + 0.18*clubPressure + 0.12*(1.0-t.Softness) + 0.08*t.TimbreBrightness + 0.06*t.Aggressive + 0.04*t.Heaviness)
	valence := clamp01(maxFloat(t.Valence, 0.58*t.Happy+0.14*(1.0-t.Sad)+0.12*t.Relaxed+0.10*t.Danceability))
	brightness := clamp01(0.34*t.Happy + 0.26*t.TimbreBrightness + 0.18*math.Max(0, valence-0.50)*2.0 + 0.12*t.Melodicness + 0.10*t.Electronicness)
	darkness := clamp01(0.34*(1.0-valence) + 0.22*(1.0-t.TimbreBrightness) + 0.20*t.Sad + 0.14*intensity + 0.10*(1.0-t.Happy))
	calmnessBase := clamp01(0.30*t.Relaxed + 0.24*t.Softness + 0.18*(1.0-drive) + 0.16*(1.0-intensity) + 0.12*(1.0-clubPressure))
	calmness := clamp01(calmnessBase - 0.20*drive*intensity - 0.16*clubPressure - 0.12*mechanicalPressure)
	aggression := clamp01(0.34*roughAudio + 0.22*t.Aggressive + 0.18*t.Heaviness + 0.14*mechanicalPressure + 0.08*(1.0-t.Softness) + 0.04*drive - 0.18*valence)
	melodicness := clamp01(maxFloat(t.Melodicness, melodicnessFallback(t)))
	softness := clamp01(t.Softness)
	heaviness := clamp01(t.Heaviness)
	dreaminess := clamp01(t.Dreaminess)
	atmosphere := clamp01(0.34*calmness + 0.24*t.Instrumentalness + 0.18*t.Electronicness + 0.14*dreaminess + 0.10*softness)
	instrumental := clamp01(t.Instrumentalness)
	vocal := clamp01(maxFloat(t.Vocalness, 1.0-instrumental))
	melancholy := clamp01(0.30*darkness + 0.24*(1.0-valence) + 0.18*calmness + 0.12*softness - 0.18*drive)
	pulse := clamp01(0.55*tempoPulse + 0.25*drive + 0.20*movement)
	return Vector{Energy: t.Energy, Valence: valence, Brightness: brightness, Darkness: darkness, Calmness: calmness, Aggression: aggression, Movement: movement, TempoBPM: t.BPMPerceived, TempoConfidence: clamp01(t.TempoConfidence), RhythmicPulse: tempoPulse, Drive: drive, Melancholy: melancholy, Intensity: intensity, Pulse: pulse, ClubPressure: clubPressure, MechanicalPressure: mechanicalPressure, Atmosphere: atmosphere, Melodicness: melodicness, Softness: softness, Heaviness: heaviness, Dreaminess: dreaminess, Acousticness: clamp01(t.Acousticness), Electronicness: clamp01(t.Electronicness), Instrumentalness: instrumental, Vocalness: vocal, TimbreBrightness: clamp01(t.TimbreBrightness), Tonality: clamp01(t.Tonality), Approachability: clamp01(t.Approachability), Engagement: clamp01(t.Engagement)}
}

func BuildState(current library.Track, previous *library.Track, next *library.Track, queue []rays.QueueItem, settings UISettings) UIState {
	settings = NormalizeSettings(settings)
	curVector := FromTrack(current)
	direction := computeDirection(current, next, queue)
	curTrack := buildTrackState(current, curVector, direction, settings)
	state := UIState{TrackID: current.ID, Current: curTrack, Vector: curVector, Direction: direction, Intensity: curTrack.Intensity, Heat: curTrack.Heat, Cool: curTrack.Cool, Tension: curTrack.Tension, Palette: curTrack.Palette, Reason: curTrack.Reason}
	if previous != nil {
		prevVec := FromTrack(*previous)
		prevDir := computeDirection(*previous, &current, queue)
		prevState := buildTrackState(*previous, prevVec, prevDir, settings)
		state.Previous = &prevState
	}
	if next != nil {
		nextVec := FromTrack(*next)
		nextDir := computeDirection(current, next, queue)
		nextState := buildTrackState(*next, nextVec, nextDir, settings)
		state.Next = &nextState
		state.Transition = buildTransition(curVector, nextVec)
		state.Transition.Direction = string(direction)
		state.Transition.Reason = explainDirection(curVector, nextVec, direction)
		if state.Reason == "" {
			state.Reason = state.Transition.Reason
		}
	} else {
		state.Transition = TransitionVisualState{Direction: string(direction), Reason: curTrack.Reason}
	}
	return state
}

func buildTrackState(track library.Track, v Vector, direction Direction, settings UISettings) TrackState {
	mood := visualMood(v)
	dominant, secondary := dominantEmotion(mood)
	heat := clamp01(0.60*v.Energy + 0.40*v.Aggression)
	cool := clamp01(0.55*v.Calmness + 0.25*v.Softness + 0.20*v.Acousticness)
	tension := clamp01(0.55*v.Aggression + 0.25*v.Darkness + 0.20*(1.0-v.Approachability))
	intensity := clamp01((0.34*heat + 0.20*tension + 0.18*v.Brightness + 0.14*v.Dreaminess + 0.14*v.Melodicness) * maxFloat(settings.Intensity, 0.18))
	reason := explainMood(v, direction)
	return TrackState{TrackID: track.ID, Title: track.Title, Artist: track.Artist, Vector: v, Palette: paletteForMood(v, direction, settings), Intensity: intensity, Heat: heat, Cool: cool, Tension: tension, Reason: reason, Dominant: dominant, Secondary: secondary, Direction: direction}
}

func buildTransition(cur, next Vector) TransitionVisualState {
	return TransitionVisualState{MoodDistance: round2(moodDistance(cur, next)), EnergyDelta: round2(next.Energy - cur.Energy), AggroDelta: round2(next.Aggression - cur.Aggression)}
}

func computeDirection(current library.Track, next *library.Track, queue []rays.QueueItem) Direction {
	if inferred := directionFromQueueInsight(current, next, queue); inferred != "" {
		return inferred
	}
	if next == nil {
		if inferred := inferExplore(queue); inferred != "" {
			return inferred
		}
		return Stable
	}
	cur := FromTrack(current)
	nxt := FromTrack(*next)
	energyDelta := nxt.Energy - cur.Energy
	aggroDelta := nxt.Aggression - cur.Aggression
	calmDelta := nxt.Calmness - cur.Calmness
	melodicDelta := nxt.Melodicness - cur.Melodicness
	darkDelta := nxt.Darkness - cur.Darkness
	brightDelta := nxt.Brightness - cur.Brightness
	textureShift := math.Abs(nxt.Electronicness-cur.Electronicness) + math.Abs(nxt.Acousticness-cur.Acousticness)
	switch {
	case aggroDelta > 0.10 || (energyDelta > 0.14 && nxt.Aggression > cur.Aggression):
		return Intensify
	case energyDelta > 0.15 || (aggroDelta > 0.10 && nxt.Movement > cur.Movement):
		return WarmUp
	case calmDelta > 0.18 && aggroDelta <= 0.02 || melodicDelta > 0.18 || (cur.Aggression-nxt.Aggression) > 0.14:
		return CoolDown
	case darkDelta > 0.16:
		return Darken
	case brightDelta > 0.16:
		return Brighten
	case melodicDelta > 0.10 || (nxt.Dreaminess-cur.Dreaminess) > 0.12:
		return Deepen
	case textureShift > 0.55 || inferExplore(queue) == Explore:
		return Explore
	default:
		return Stable
	}
}

func directionFromQueueInsight(current library.Track, next *library.Track, queue []rays.QueueItem) Direction {
	targetID := ""
	if next != nil {
		targetID = next.ID
	} else {
		targetID = current.ID
	}
	if targetID == "" {
		return ""
	}
	for _, item := range queue {
		if item.TrackID != targetID {
			continue
		}
		if d := parseDirection(item.Insight.EnergyDirection); d != "" {
			return d
		}
		if d := parseDirection(item.Insight.Transition); d != "" {
			return d
		}
	}
	return ""
}

func parseDirection(value string) Direction {
	switch strings.TrimSpace(value) {
	case "intensify":
		return Intensify
	case "warm_up":
		return WarmUp
	case "cool_down", "soften":
		return CoolDown
	case "brighten":
		return Brighten
	case "darken":
		return Darken
	case "deepen":
		return Deepen
	default:
		return ""
	}
}

func inferExplore(queue []rays.QueueItem) Direction {
	for _, item := range queue {
		if strings.TrimSpace(item.Insight.Mode) == "explore" {
			return Explore
		}
	}
	return ""
}

func explainDirection(cur, next Vector, direction Direction) string {
	switch direction {
	case Intensify:
		return "плавно тяжелее и напряжённее"
	case WarmUp:
		return "разогревается и становится энергичнее"
	case CoolDown:
		return "успокаивается и смягчается"
	case Darken:
		return "темнеет и уходит глубже"
	case Brighten:
		return "светлеет и открывается"
	case Deepen:
		return "становится глубже и мелодичнее"
	case Explore:
		return "исследует соседний вайб без резкого слома"
	default:
		if next.Aggression > cur.Aggression+0.12 {
			return "чуть тяжелее"
		}
		if next.Calmness > cur.Calmness+0.10 {
			return "чуть спокойнее"
		}
		return "сохраняет текущее настроение"
	}
}

func tempoToPulse(bpm, confidence float64) float64 {
	if bpm <= 0 || confidence <= 0 {
		return 0
	}
	x := clamp01((bpm - 60.0) / 120.0)
	return x * clamp01(confidence)
}

func audioEdge(t library.Track) float64 {
	zcr := clamp01(t.ZeroCrossingRate / 0.18)
	centroid := clamp01((t.SpectralCentroid - 600.0) / 1800.0)
	bright := clamp01(t.TimbreBrightness)
	electronic := clamp01(t.Electronicness)
	softInv := 1.0 - clamp01(t.Softness)
	melodicInv := 1.0 - clamp01(t.Melodicness)
	return clamp01(0.34*zcr + 0.30*centroid + 0.12*bright + 0.08*electronic + 0.10*softInv + 0.06*melodicInv)
}

func smoothnessDamping(t library.Track, edge float64) float64 {
	lowCentroid := 1.0 - clamp01((t.SpectralCentroid-400.0)/1600.0)
	lowZCR := 1.0 - clamp01(t.ZeroCrossingRate/0.18)
	relaxed := clamp01(t.Relaxed)
	soft := clamp01(t.Softness)
	electronic := clamp01(t.Electronicness)
	return clamp01(0.30*lowCentroid + 0.22*lowZCR + 0.20*relaxed + 0.12*soft + 0.10*electronic + 0.06*(1.0-edge))
}

func tempoPushForMood(t library.Track) float64 {
	bpm := t.BPMPerceived
	if bpm <= 0 {
		bpm = t.Tempo
	}
	if bpm <= 0 || t.TempoConfidence < 0.35 {
		return 0
	}
	return clamp01((bpm - 115.0) / 20.0)
}

func BuildPerceptualBasis(t library.Track) PerceptualBasis {
	m := moodBasisFromTrack(t)
	return PerceptualBasis{Motion: m.Motion, Density: clamp01(t.Energy), Roughness: clamp01(audioEdge(t)), Brightness: clamp01(t.TimbreBrightness), Smoothness: clamp01(1 - audioEdge(t)), Impact: clamp01(t.Energy), Pressure: m.Pressure, Joy: clamp01(t.Valence), Melancholy: clamp01(1 - t.Valence), Serenity: m.Calmness, Swagger: clamp01(t.Danceability), Combat: clamp01(t.Aggressive), Sprint: clamp01(t.BPMPerceived / 200), SprintClean: clamp01(t.BPMPerceived / 200), Label: moodLabel(m)}
}

func moodBasisFromTrack(t library.Track) MoodBasis {
	pulse := tempoToPulse(t.BPMPerceived, t.TempoConfidence)
	tempoPush := tempoPushForMood(t)
	edge := audioEdge(t)
	smooth := smoothnessDamping(t, edge)
	motion := clamp01(0.42*t.Danceability + 0.24*pulse + 0.16*t.Energy + 0.10*t.Party + 0.08*(1.0-t.Softness))
	pressureRaw := clamp01(0.28*motion + 0.22*t.Energy + 0.20*edge + 0.12*t.Party + 0.10*(1.0-t.Relaxed) + 0.05*t.Electronicness + 0.03*(1.0-t.Softness) + 0.01*tempoPush)
	pressure := clamp01(pressureRaw - 0.22*smooth)
	calmBase := clamp01(0.34*t.Relaxed + 0.22*t.Softness + 0.18*(1.0-motion) + 0.18*(1.0-pressure) + 0.08*smooth)
	calmness := clamp01(calmBase - 0.18*edge - 0.16*motion*pressure)
	texture := clamp01(0.56*t.Electronicness + 0.24*(1.0-clamp01(t.Acousticness)) + 0.20*t.TimbreBrightness)
	coolness := clamp01(0.30*(1.0*t.TimbreBrightness) + 0.20*(1.0*t.Valence) + 0.16*texture + 0.14*t.Relaxed + 0.12*(1.0*t.Party) + 0.08*smooth)
	warmth := clamp01(0.34*t.Valence + 0.26*t.TimbreBrightness + 0.16*t.Happy + 0.12*t.Party + 0.12*(1.0-coolness))
	return MoodBasis{Motion: motion, Pressure: pressure, Calmness: calmness, Coolness: coolness, Warmth: warmth, Texture: texture, Edge: edge, Valence: clamp01(t.Valence), Vocality: clamp01(t.Vocalness)}
}

func moodBasisFromVector(v Vector) MoodBasis {
	motion := clamp01(0.52*v.Movement + 0.22*v.Pulse + 0.16*v.Energy + 0.10*(1.0-v.Softness))
	edge := clamp01(0.28*(1.0-v.Melodicness) + 0.22*v.TimbreBrightness + 0.20*v.Electronicness + 0.14*(1.0-v.Softness) + 0.10*v.MechanicalPressure + 0.06*(1.0-v.Atmosphere))
	pressure := clamp01(0.38*v.Intensity + 0.24*v.Drive + 0.20*v.MechanicalPressure + 0.18*v.ClubPressure)
	calmness := clamp01(0.40*v.Calmness + 0.20*v.Softness + 0.16*(1.0-motion) + 0.16*(1.0-pressure) + 0.08*v.Atmosphere - 0.20*edge)
	coolness := clamp01(0.34*(1.0*v.Brightness) + 0.24*(1.0*v.Valence) + 0.18*v.Atmosphere + 0.12*v.Softness + 0.12*(1.0-motion))
	warmth := clamp01(0.34*v.Brightness + 0.28*v.Valence + 0.14*v.Energy + 0.12*v.Movement + 0.12*(1.0-coolness))
	texture := clamp01(0.60*v.Electronicness + 0.20*v.MechanicalPressure + 0.20*(1.0*v.Acousticness))
	return MoodBasis{Motion: motion, Pressure: pressure, Calmness: calmness, Coolness: coolness, Warmth: warmth, Texture: texture, Edge: edge, Valence: clamp01(v.Valence), Vocality: clamp01(v.Vocalness)}
}

func moodBasisDistance(a, b MoodBasis) float64 {
	d := 0.0
	d += math.Abs(a.Motion-b.Motion) * 0.18
	d += math.Abs(a.Pressure-b.Pressure) * 0.20
	d += math.Abs(a.Edge-b.Edge) * 0.18
	d += math.Abs(a.Coolness-b.Coolness) * 0.14
	d += math.Abs(a.Calmness-b.Calmness) * 0.12
	d += math.Abs(a.Texture-b.Texture) * 0.10
	d += math.Abs(a.Valence-b.Valence) * 0.05
	d += math.Abs(a.Vocality-b.Vocality) * 0.03
	return clamp01(d)
}

func moodLabel(m MoodBasis) string {
	if m.Motion > 0.72 && m.Pressure > 0.58 {
		return "скоростной драйв"
	}
	if m.Coolness > 0.58 && m.Pressure < 0.68 {
		return "ночной грув"
	}
	if m.Texture > 0.44 && m.Motion > 0.60 {
		return "электронный драйв"
	}
	if m.Calmness > 0.52 && m.Pressure < 0.52 {
		return "спокойный поток"
	}
	if m.Warmth > 0.60 && m.Coolness < 0.50 {
		return "тёплый вайб"
	}
	if m.Motion > 0.58 {
		return "ровный кач"
	}
	return "нейтральный поток"
}

func accentFromMoodBasis(m MoodBasis) oklchColor {
	if m.Motion > 0.72 && m.Pressure > 0.58 {
		return oklchColor{L: 0.62, C: 0.22, H: 22}
	}
	if m.Coolness > 0.58 && m.Pressure < 0.68 {
		return oklchColor{L: 0.56, C: 0.17, H: 245}
	}
	if m.Texture > 0.44 && m.Motion > 0.60 {
		return oklchColor{L: 0.60, C: 0.18, H: 210}
	}
	if m.Calmness > 0.52 && m.Pressure < 0.52 {
		return oklchColor{L: 0.62, C: 0.14, H: 225}
	}
	if m.Warmth > 0.60 && m.Coolness < 0.50 {
		return oklchColor{L: 0.69, C: 0.17, H: 48}
	}
	if m.Motion > 0.58 {
		return oklchColor{L: 0.62, C: 0.17, H: 32}
	}
	return oklchColor{L: 0.60, C: 0.07, H: 230}
}

func atmosphereHint(v Vector) float64 {
	return clamp01(0.34*v.Calmness + 0.24*v.Instrumentalness + 0.18*v.Electronicness + 0.14*v.Dreaminess + 0.10*v.Softness)
}

func explainMood(v Vector, direction Direction) string {
	switch direction {
	case Intensify, WarmUp, CoolDown, Darken, Brighten, Deepen, Explore:
		return explainDirection(v, v, direction)
	}
	basis := moodBasisFromVector(v)
	return moodLabel(basis)
}

func paletteForMood(v Vector, direction Direction, settings UISettings) Palette {
	influence := clamp01(settings.Intensity)
	moodAccent := computeMoodAccent(v)
	accent := mixOKLCH(moodAnchors["default"], moodAccent, influence)
	accent = adjustColorByDirection(accent, v, direction)
	accent = polishAccent(accent, v, influence)
	soft := polishVariant(oklchColor{L: clamp(accent.L+0.06, 0.50, 0.86), C: clamp(accent.C*0.75, 0.08, 0.20), H: accent.H}, 0.08, 0.20)
	hot := polishVariant(oklchColor{L: clamp(accent.L+0.04, 0.50, 0.82), C: clamp(accent.C*1.18, 0.15, 0.28), H: accent.H}, 0.15, 0.28)
	icon := polishVariant(oklchColor{L: clamp(accent.L+0.08, 0.52, 0.84), C: clamp(accent.C*1.10, 0.12, 0.26), H: accent.H}, 0.12, 0.26)
	progress := polishVariant(oklchColor{L: clamp(accent.L+0.02, 0.50, 0.84), C: clamp(accent.C*1.04, 0.10, 0.24), H: accent.H}, 0.10, 0.24)
	background := oklchColor{L: clamp(0.10+accent.L*0.03, 0.08, 0.14), C: clamp(accent.C*0.08, 0.02, 0.05), H: accent.H}
	surface := oklchColor{L: clamp(0.14+accent.L*0.04, 0.12, 0.18), C: clamp(accent.C*0.10, 0.02, 0.06), H: accent.H}
	glowOpacity := clamp(0.08+v.Energy*0.10+v.Movement*0.06, 0.08, 0.28)
	if settings.RespectReducedMotion && !settings.AnimateDuringTrack {
		glowOpacity = math.Min(glowOpacity, 0.18)
	}
	accentOn := "white"
	if accent.L > 0.68 {
		accentOn = "rgba(0,0,0,.92)"
	}
	return Palette{Accent: oklchCSS(accent, 1), AccentSoft: oklchCSS(soft, 1), AccentHot: oklchCSS(hot, 1), Background: oklchCSS(background, 1), Surface: oklchCSS(surface, 1), Glow: oklchCSS(hot, glowOpacity), GlowSoft: oklchCSS(accent, math.Min(0.18, glowOpacity*0.65)), Ring: oklchCSS(accent, math.Min(0.16, glowOpacity*0.55)), Progress: oklchCSS(progress, 1), Icon: oklchCSS(icon, 1), AccentOn: accentOn}
}

func computeMoodAccent(v Vector) oklchColor {
	return accentFromMoodBasis(moodBasisFromVector(v))
}

func adjustColorByDirection(c oklchColor, v Vector, direction Direction) oklchColor {
	switch direction {
	case WarmUp:
		c = mixOKLCH(c, moodAnchors["party"], 0.14)
		c.C += 0.01
	case CoolDown:
		c = mixOKLCH(c, moodAnchors["relaxed"], 0.16)
		c.C -= 0.01
	case Brighten:
		c.L += 0.04
		c.C += 0.01
	case Darken:
		c = mixOKLCH(c, moodAnchors["dark"], 0.22)
		c.L -= 0.05
		c.C = math.Max(c.C, 0.12)
	case Intensify:
		hot := moodAnchors["aggressive"]
		if v.Electronicness > 0.62 {
			hot = moodAnchors["electronic"]
		}
		c = mixOKLCH(c, hot, 0.26)
		c.C += 0.03
	case Deepen:
		c = mixOKLCH(c, moodAnchors["dreamy"], 0.16)
		c.L -= 0.02
		c.C += 0.01
	case Explore:
		if v.Electronicness >= v.Acousticness {
			c = mixOKLCH(c, moodAnchors["electronic"], 0.14)
		} else {
			c = mixOKLCH(c, moodAnchors["dreamy"], 0.14)
		}
	}
	return c
}

func mixWeightedAnchors(items []weightedMood) oklchColor {
	var total, l, c, x, y float64
	for _, item := range items {
		weight := math.Max(0, item.Weight)
		if weight == 0 {
			continue
		}
		anchor, ok := moodAnchors[item.Key]
		if !ok {
			continue
		}
		rad := anchor.H * math.Pi / 180
		total += weight
		l += anchor.L * weight
		c += anchor.C * weight
		x += math.Cos(rad) * anchor.C * weight
		y += math.Sin(rad) * anchor.C * weight
	}
	if total == 0 {
		return moodAnchors["default"]
	}
	return oklchColor{L: l / total, C: clamp(c/total, 0.10, 0.26), H: math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)}
}

func mixOKLCH(a, b oklchColor, t float64) oklchColor {
	t = clamp01(t)
	delta := math.Mod((b.H-a.H)+540, 360) - 180
	return oklchColor{L: a.L + (b.L-a.L)*t, C: a.C + (b.C-a.C)*t, H: math.Mod(a.H+delta*t+360, 360)}
}

func polishAccent(c oklchColor, v Vector, influence float64) oklchColor {
	energyBoost := 0.02 * clamp01(v.Energy)
	movementBoost := 0.015 * clamp01(v.Movement)
	calmReduce := 0.02 * clamp01(v.Calmness)
	return oklchColor{L: clamp(c.L, 0.50, 0.82), C: clamp(c.C+energyBoost+movementBoost-calmReduce+0.01*influence, 0.10, 0.24), H: c.H}
}

func clampHueAround(c oklchColor, lower, upper float64) oklchColor {
	if c.H < lower {
		c.H = lower
	}
	if c.H > upper {
		c.H = upper
	}
	return c
}

func polishVariant(c oklchColor, minC, maxC float64) oklchColor {
	c.C = clamp(c.C, minC, maxC)
	c.L = clamp(c.L, 0.58, 0.88)
	return c
}

func oklchCSS(c oklchColor, alpha float64) string {
	if alpha >= 1 {
		return fmt.Sprintf("oklch(%d%% %.3f %d)", int(math.Round(c.L*100)), c.C, int(math.Round(c.H)))
	}
	return fmt.Sprintf("oklch(%d%% %.3f %d / %.3f)", int(math.Round(c.L*100)), c.C, int(math.Round(c.H)), clamp01(alpha))
}

func visualMood(v Vector) map[string]float64 {
	b := moodBasisFromVector(v)
	return map[string]float64{
		"aggressive": clamp01(0.62*b.Pressure + 0.22*v.Intensity + 0.16*(1-b.Calmness)),
		"calm":       clamp01(0.62*b.Calmness + 0.18*v.Softness + 0.20*v.Atmosphere),
		"sad":        clamp01(0.42*v.Darkness + 0.30*(1-b.Valence) + 0.18*v.Melodicness),
		"happy":      clamp01(0.42*b.Warmth + 0.22*b.Motion + 0.18*b.Valence + 0.18*v.Brightness),
		"electronic": clamp01(0.56*b.Texture + 0.20*v.Electronicness + 0.24*v.MechanicalPressure),
		"melodic":    clamp01(0.50*v.Melodicness + 0.24*v.Softness + 0.12*v.Tonality + 0.14*v.Atmosphere),
	}
}

func dominantEmotion(m map[string]float64) (string, string) {
	bestKey, secondKey := "calm", "melodic"
	bestVal, secondVal := -1.0, -1.0
	for key, value := range m {
		if value > bestVal {
			secondKey, secondVal = bestKey, bestVal
			bestKey, bestVal = key, value
			continue
		}
		if value > secondVal {
			secondKey, secondVal = key, value
		}
	}
	return bestKey, secondKey
}

func moodDistance(a, b Vector) float64 {
	return moodBasisDistance(moodBasisFromVector(a), moodBasisFromVector(b))
}

func melodicnessFallback(t library.Track) float64 {
	return clamp01((1-clamp01(t.ZeroCrossingRate*5))*0.35 + clamp01(1-math.Abs(t.SpectralCentroid-1800)/1800)*0.25 + clamp01(t.Vocalness)*0.20 + clamp01(t.Acousticness)*0.20)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
