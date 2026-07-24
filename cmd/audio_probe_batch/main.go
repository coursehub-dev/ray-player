package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ray-player1/internal/analysis"
	"ray-player1/internal/emotion"
	"ray-player1/internal/onnx"
)

// ProbeConfig is a standalone offline probe settings file.
// The CLI auto-creates this file on first run when it is missing.
type ProbeConfig struct {
	ModelsDir              string `json:"modelsDir"`
	ONNXRuntimePath        string `json:"onnxRuntimePath"`
	FFmpegPath             string `json:"ffmpegPath"`
	MelMode                string `json:"melMode"`
	IncludeRawVectors      bool   `json:"includeRawVectors"`
	IncludePatchRows       bool   `json:"includePatchRows"`
	MaxPatchRows           int    `json:"maxPatchRows"`
	IncludeEmbeddings      bool   `json:"includeEmbeddings"`
	IncludeGenrePatchDebug bool   `json:"includeGenrePatchDebug"`
	IncludeTempoDebug      bool   `json:"includeTempoDebug"`
}

type BatchAudioProbeReport struct {
	Tool        string                 `json:"tool"`
	Version     string                 `json:"version"`
	GeneratedAt string                 `json:"generatedAt"`
	ConfigPath  string                 `json:"configPath"`
	Config      ProbeConfig            `json:"config"`
	AudioDir    string                 `json:"audioDir"`
	OutPath     string                 `json:"outPath"`
	FilesFound  int                    `json:"filesFound"`
	FilesOK     int                    `json:"filesOk"`
	FilesError  int                    `json:"filesError"`
	Tracks      []TrackProbeReport     `json:"tracks"`
	Summary     onnx.BatchProbeSummary `json:"summary"`
}

type TrackProbeReport struct {
	FilePath      string                   `json:"filePath"`
	FileName      string                   `json:"fileName"`
	OK            bool                     `json:"ok"`
	Error         string                   `json:"error,omitempty"`
	Audio         onnx.AudioDecodeProbe    `json:"audio"`
	Tempo         onnx.TempoProbe          `json:"tempo"`
	Essentia      onnx.EssentiaProbeReport `json:"essentia"`
	FinalFeatures onnx.FinalFeatureReport  `json:"finalFeatures"`
	Timing        onnx.ProbeTiming         `json:"timing,omitempty"`
	Warnings      []string                 `json:"warnings,omitempty"`
}

type ShortBatchAudioProbeReport struct {
	Tool           string                  `json:"tool"`
	Version        string                  `json:"version"`
	Mode           string                  `json:"mode"`
	GeneratedAt    string                  `json:"generatedAt"`
	AudioDir       string                  `json:"audioDir"`
	FilesFound     int                     `json:"filesFound"`
	FilesOK        int                     `json:"filesOk"`
	FilesError     int                     `json:"filesError"`
	Tracks         []ShortTrackProbeReport `json:"tracks"`
	Summary        ShortBatchSummary       `json:"summary"`
	FormulaVersion string                  `json:"formulaVersion,omitempty"`
}

type ShortTrackProbeReport struct {
	File            string             `json:"file"`
	Title           string             `json:"title,omitempty"`
	OK              bool               `json:"ok"`
	Error           string             `json:"error,omitempty"`
	Dur             float64            `json:"dur"`
	AnalyzedSec     float64            `json:"analyzedSec,omitempty"`
	BPM             float64            `json:"bpm"`
	PBPM            float64            `json:"pbpm,omitempty"`
	TConf           float64            `json:"tConf"`
	TStab           float64            `json:"tStab"`
	TTrust          float64            `json:"tTrust,omitempty"`
	TSrc            string             `json:"tSrc,omitempty"`
	Genre           string             `json:"genre,omitempty"`
	Expected        string             `json:"expected,omitempty"`
	Match           bool               `json:"match,omitempty"`
	CompatibleMatch bool               `json:"compatibleMatch,omitempty"`
	GScore          float64            `json:"gScore,omitempty"`
	GMargin         float64            `json:"gMargin,omitempty"`
	F               ShortFeatureValues `json:"f"`
	Audio           ShortAudioTexture  `json:"audio"`
	Heads           ShortHeadValues    `json:"heads,omitempty"`
	Audio2          ShortAudioTexture2 `json:"audio2,omitempty"`
	Audio2Quality   ShortAudio2Quality `json:"audio2q,omitempty"`
	Timing          onnx.ProbeTiming   `json:"timing,omitempty"`
	Norm2           ShortNormValues2   `json:"norm2,omitempty"`
	Basis           ShortMoodBasis     `json:"basis,omitempty"`
	Basis3          ShortBasis3        `json:"basis3,omitempty"`
	Basis3Debug     Basis3Debug        `json:"basis3debug,omitempty"`
	Norm            ShortNormValues    `json:"norm,omitempty"`
	Warn            []string           `json:"warn,omitempty"`
}

type ShortFeatureValues struct {
	Energy   float64 `json:"e"`
	Dance    float64 `json:"dance"`
	Val      float64 `json:"val"`
	Happy    float64 `json:"happy"`
	Sad      float64 `json:"sad"`
	Relax    float64 `json:"relax"`
	Party    float64 `json:"party"`
	Aggr     float64 `json:"aggr"`
	Acoustic float64 `json:"ac"`
	Elec     float64 `json:"el"`
	Instr    float64 `json:"instr"`
	Vocal    float64 `json:"vocal"`
	Mel      float64 `json:"mel"`
	Soft     float64 `json:"soft"`
	Heavy    float64 `json:"heavy"`
	Dream    float64 `json:"dream"`
	Emo      float64 `json:"emo"`
	Bright   float64 `json:"bright"`
	Tonal    float64 `json:"tonal"`
	Approach float64 `json:"approach,omitempty"`
	Engage   float64 `json:"engage,omitempty"`
}

type ShortAudioTexture struct {
	Loudness float64 `json:"loud,omitempty"`
	Centroid float64 `json:"centroid,omitempty"`
	ZCR      float64 `json:"zcr,omitempty"`
	RMS      float64 `json:"rms,omitempty"`
}

type ShortAudioTexture2 struct {
	Flatness     float64 `json:"flat,omitempty"`
	Rolloff85    float64 `json:"roll85,omitempty"`
	Flux         float64 `json:"flux,omitempty"`
	OnsetRate    float64 `json:"onset,omitempty"`
	DynamicRange float64 `json:"dyn,omitempty"`
	LowBand      float64 `json:"low,omitempty"`
	MidBand      float64 `json:"mid,omitempty"`
	HighBand     float64 `json:"high,omitempty"`
}

type ShortAudio2Quality struct {
	FlatOK     bool `json:"flatOk,omitempty"`
	RollOK     bool `json:"rollOk,omitempty"`
	FluxOK     bool `json:"fluxOk,omitempty"`
	BandsOK    bool `json:"bandsOk,omitempty"`
	CentroidOK bool `json:"centroidOk,omitempty"`
}

type ShortNormValues2 struct {
	ZCR         float64 `json:"zcrN,omitempty"`
	Centroid    float64 `json:"centN,omitempty"`
	Flatness    float64 `json:"flatN,omitempty"`
	Rolloff85   float64 `json:"rollN,omitempty"`
	Flux        float64 `json:"fluxN,omitempty"`
	Onset       float64 `json:"onsetN,omitempty"`
	Density     float64 `json:"density,omitempty"`
	GrooveTempo float64 `json:"grooveTempo,omitempty"`
	SprintTempo float64 `json:"sprintTempo,omitempty"`
	TempoTrust  float64 `json:"tempoTrust,omitempty"`
}

type ShortBasis3 struct {
	Motion      float64 `json:"motion,omitempty"`
	Pulse       float64 `json:"pulse,omitempty"`
	Density     float64 `json:"density,omitempty"`
	Roughness   float64 `json:"rough,omitempty"`
	Brightness  float64 `json:"bright,omitempty"`
	Smoothness  float64 `json:"smooth,omitempty"`
	Impact      float64 `json:"impact,omitempty"`
	Pressure    float64 `json:"pressure,omitempty"`
	Joy         float64 `json:"joy,omitempty"`
	Melancholy  float64 `json:"melancholy,omitempty"`
	Serenity    float64 `json:"serenity,omitempty"`
	Swagger     float64 `json:"swagger,omitempty"`
	Combat      float64 `json:"combat,omitempty"`
	Sprint      float64 `json:"sprint,omitempty"`
	SprintClean float64 `json:"sprintClean,omitempty"`
	Label       string  `json:"label,omitempty"`
}

type ShortHeadValues struct {
	DanceSupport    float64 `json:"danceSup,omitempty"`
	PartySupport    float64 `json:"partySup,omitempty"`
	RelaxSupport    float64 `json:"relaxSup,omitempty"`
	ElecSupport     float64 `json:"elecSup,omitempty"`
	AcousticSupport float64 `json:"acSup,omitempty"`
	AggrSupport     float64 `json:"aggrSup,omitempty"`
	SadSupport      float64 `json:"sadSup,omitempty"`
	HappySupport    float64 `json:"happySup,omitempty"`
}

type ShortMoodBasis struct {
	Motion   float64 `json:"motion,omitempty"`
	Pressure float64 `json:"pressure,omitempty"`
	Edge     float64 `json:"edge,omitempty"`
	Calm     float64 `json:"calm,omitempty"`
	Cool     float64 `json:"cool,omitempty"`
	Warm     float64 `json:"warm,omitempty"`
	Texture  float64 `json:"texture,omitempty"`
	Label    string  `json:"label,omitempty"`
	Hue      float64 `json:"hue,omitempty"`
}

type ShortNormValues struct {
	Dance       float64 `json:"dance,omitempty"`
	Party       float64 `json:"party,omitempty"`
	Relax       float64 `json:"relax,omitempty"`
	Elec        float64 `json:"el,omitempty"`
	ZCR         float64 `json:"zcrN,omitempty"`
	Centroid    float64 `json:"centN,omitempty"`
	Density     float64 `json:"density,omitempty"`
	GrooveTempo float64 `json:"grooveTempo,omitempty"`
	SpeedTempo  float64 `json:"speedTempo,omitempty"`
	TempoTrust  float64 `json:"tempoTrust,omitempty"`
}

type ShortBatchSummary struct {
	FeatureStats     map[string]ShortStat      `json:"featureStats,omitempty"`
	LabelCounts      map[string]int            `json:"labelCounts,omitempty"`
	Warnings         map[string]int            `json:"warnings,omitempty"`
	SelfCheck        []string                  `json:"selfCheck,omitempty"`
	Errors           []string                  `json:"errors,omitempty"`
	ExpectedAccuracy float64                   `json:"expectedAccuracy,omitempty"`
	Confusion        map[string]map[string]int `json:"confusion,omitempty"`
	Calibration      ShortCalibrationSummary   `json:"calibration,omitempty"`
}

type ShortCalibrationSummary struct {
	ExpectedTotal      int                        `json:"expectedTotal,omitempty"`
	ExactMatches       int                        `json:"exactMatches,omitempty"`
	CompatibleMatches  int                        `json:"compatibleMatches,omitempty"`
	ExactAccuracy      float64                    `json:"exactAccuracy,omitempty"`
	CompatibleAccuracy float64                    `json:"compatibleAccuracy,omitempty"`
	Confusion          map[string]map[string]int  `json:"confusion,omitempty"`
	Mismatches         []ShortCalibrationMismatch `json:"mismatches,omitempty"`
}

type ShortCalibrationMismatch struct {
	File       string `json:"file"`
	Expected   string `json:"expected,omitempty"`
	Got        string `json:"got,omitempty"`
	Compatible bool   `json:"compatible,omitempty"`
}

type ShortStat struct {
	Count int     `json:"n"`
	Mean  float64 `json:"mean"`
	P05   float64 `json:"p05"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

type LabelScore struct {
	Label  string  `json:"label"`
	Score  float64 `json:"score"`
	Passed bool    `json:"passed"`
	Reason string  `json:"reason,omitempty"`
}

type Basis3Debug struct {
	CleanBright    float64               `json:"cleanBright,omitempty"`
	CleanParty     float64               `json:"cleanParty,omitempty"`
	Intimacy       float64               `json:"intimacy,omitempty"`
	RoughRaw       float64               `json:"roughRaw,omitempty"`
	CombatRaw      float64               `json:"combatRaw,omitempty"`
	JoyRaw         float64               `json:"joyRaw,omitempty"`
	JoyCombatCut   float64               `json:"joyCombatCut,omitempty"`
	JoyRoughCut    float64               `json:"joyRoughCut,omitempty"`
	JoyPressureCut float64               `json:"joyPressureCut,omitempty"`
	JoyEdgeCut     float64               `json:"joyEdgeCut,omitempty"`
	JoyDirtyCut    float64               `json:"joyDirtyCut,omitempty"`
	JoyCleanBoost  float64               `json:"joyCleanBoost,omitempty"`
	MelancholyRaw  float64               `json:"melancholyRaw,omitempty"`
	SerenityRaw    float64               `json:"serenityRaw,omitempty"`
	DirtyElectro   float64               `json:"dirtyElectro,omitempty"`
	HeavyDrive     float64               `json:"heavyDrive,omitempty"`
	TensePressure  float64               `json:"tensePressure,omitempty"`
	WarmGroove     float64               `json:"warmGroove,omitempty"`
	SereneBright   float64               `json:"sereneBright,omitempty"`
	JoyConfidence  float64               `json:"joyConfidence,omitempty"`
	VocalGrief     float64               `json:"vocalGrief,omitempty"`
	DramaticArc    float64               `json:"dramaticArc,omitempty"`
	Decision       emotion.DecisionAudit `json:"decision,omitempty"`
	TopLabels      []LabelScore          `json:"topLabels,omitempty"`
}

func main() {
	configPath := flag.String("config", "./audio_probe_config.json", "path to JSON config")
	audioDir := flag.String("audio-dir", "", "folder with mp3 files, non-recursive")
	outPath := flag.String("out", "/tmp/audio_probe_report.json", "output JSON report")
	mode := flag.String("mode", "full", "output mode: full, short or ultrashort")
	expectedPath := flag.String("expected", "", "optional expected labels JSON for short mode")
	pretty := flag.Bool("pretty", true, "pretty-print JSON")
	quiet := flag.Bool("quiet", false, "suppress verbose model/probe logs")
	flag.Parse()
	if !*quiet {
		fmt.Printf("argv=%q\n", os.Args)
		fmt.Printf("parsed expected=%q mode=%q out=%q audioDir=%q\n", *expectedPath, *mode, *outPath, *audioDir)
	}
	if strings.TrimSpace(*expectedPath) == "" {
		if fallback, ok := parseExpectedPath(os.Args[1:]); ok {
			*expectedPath = fallback
			if !*quiet {
				fmt.Printf("expected fallback parsed=%q\n", *expectedPath)
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "full", "short", "ultrashort":
	default:
		must(fmt.Errorf("invalid -mode %q, expected full, short or ultrashort", *mode))
	}

	cfg, created, err := ensureConfig(*configPath)
	must(err)
	if created {
		fmt.Println("config file created:", *configPath)
		fmt.Println("edit it and run again")
		return
	}
	if strings.TrimSpace(*audioDir) == "" {
		must(errors.New("-audio-dir is required"))
	}

	files, err := findMP3Files(*audioDir)
	must(err)

	ctx := context.Background()
	analysis.SetFFmpegPath(cfg.FFmpegPath)

	ess, err := onnx.NewEssentiaEngine(cfg.ONNXRuntimePath, cfg.ModelsDir)
	must(err)
	defer ess.Close()

	var tempoEngine *onnx.TempoEngine
	if strings.EqualFold(*mode, "full") || strings.EqualFold(*mode, "short") || strings.EqualFold(*mode, "ultrashort") {
		tempoEngine, err = onnx.NewTempoEngine(cfg.ONNXRuntimePath, cfg.ModelsDir)
		if err != nil {
			fmt.Printf("WARN tempo engine init failed: %v\n", err)
		} else if tempoEngine == nil {
			fmt.Println("WARN tempo engine is nil")
		} else {
			fmt.Printf("tempo engine ready=%v\n", tempoEngine.Ready())
		}
		must(err)
		if tempoEngine != nil {
			defer tempoEngine.Close()
		}
	}

	fullTracks := make([]TrackProbeReport, 0, len(files))
	filesOK, filesError := 0, 0
	shortLike := strings.EqualFold(*mode, "short") || strings.EqualFold(*mode, "ultrashort")
	useTempoEngine := strings.EqualFold(*mode, "full") || shortLike
	for _, path := range files {
		tr := analyzeOne(ctx, path, ess, tempoEngine, cfg, shortLike, useTempoEngine, *quiet)
		fullTracks = append(fullTracks, tr)
		if tr.OK {
			filesOK++
		} else {
			filesError++
		}
	}

	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "short", "ultrashort":
		shortReport := ShortBatchAudioProbeReport{
			Tool:           "audio_probe_batch",
			Version:        "1",
			Mode:           "short",
			GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
			AudioDir:       *audioDir,
			FilesFound:     len(files),
			FilesOK:        filesOK,
			FilesError:     filesError,
			FormulaVersion: emotion.DefaultTuning().Version,
			Tracks:         make([]ShortTrackProbeReport, 0, len(fullTracks)),
		}
		expectedLookup := loadExpectedLabels(*expectedPath)
		if !*quiet {
			fmt.Printf("expected lookup loaded exact=%d norm=%d path=%q\n", len(expectedLookup.exact), len(expectedLookup.norm), *expectedPath)
		}
		for _, tr := range fullTracks {
			shortReport.Tracks = append(shortReport.Tracks, toShortTrack(tr))
		}
		stats := buildShortCalibrationStats(shortReport.Tracks)
		for i := range shortReport.Tracks {
			shortReport.Tracks[i].Norm2, shortReport.Tracks[i].Basis3, shortReport.Tracks[i].Basis3Debug = computeBasis3(shortReport.Tracks[i], stats)
			if i == 0 && !*quiet {
				if exp0, ok0 := expectedLookup.Lookup(shortReport.Tracks[i].File); ok0 {
					fmt.Printf("expected first lookup file=%q exp=%q\n", shortReport.Tracks[i].File, exp0)
				} else {
					fmt.Printf("expected first lookup file=%q MISS\n", shortReport.Tracks[i].File)
				}
			}
			if exp, ok := expectedLookup.Lookup(shortReport.Tracks[i].File); ok {
				shortReport.Tracks[i].Expected = exp
				shortReport.Tracks[i].Match = exp != "" && exp == shortReport.Tracks[i].Basis3.Label
				shortReport.Tracks[i].CompatibleMatch = exp != "" && isCompatibleLabel(exp, shortReport.Tracks[i].Basis3.Label)
			} else {
				shortReport.Tracks[i].Warn = append(shortReport.Tracks[i].Warn, "expected_missing_for_file")
			}
		}
		shortReport.Summary = buildShortSummary(shortReport.Tracks, expectedLookup)
		shortReport.Summary.SelfCheck = append(shortReport.Summary.SelfCheck, validateShortReport(shortReport)...)
		if strings.EqualFold(strings.TrimSpace(*mode), "ultrashort") {
			writeAnyReport(*outPath, toUltraShortReport(shortReport), *pretty)
		} else {
			writeAnyReport(*outPath, shortReport, *pretty)
		}
	default:
		fullReport := BatchAudioProbeReport{
			Tool:        "audio_probe_batch",
			Version:     "1",
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			ConfigPath:  *configPath,
			Config:      cfg,
			AudioDir:    *audioDir,
			OutPath:     *outPath,
			FilesFound:  len(files),
			FilesOK:     filesOK,
			FilesError:  filesError,
			Tracks:      fullTracks,
		}
		fullReport.Summary = buildBatchSummary(fullTracks)
		writeAnyReport(*outPath, fullReport, *pretty)
	}

	fmt.Printf("written: %s\\n", *outPath)
	fmt.Printf("files: %d ok: %d error: %d\\n", len(files), filesOK, filesError)
}

func analyzeOne(ctx context.Context, path string, ess *onnx.EssentiaEngine, tempoEngine *onnx.TempoEngine, cfg ProbeConfig, shortAnalysis bool, useTempoEngine bool, quiet bool) TrackProbeReport {
	tr := TrackProbeReport{FilePath: path, FileName: filepath.Base(path)}
	probe, err := ess.ProbeAudioFileFull(ctx, path, onnx.ProbeOptions{
		MelMode:                cfg.MelMode,
		IncludeRawVectors:      cfg.IncludeRawVectors,
		IncludePatchRows:       cfg.IncludePatchRows,
		MaxPatchRows:           cfg.MaxPatchRows,
		IncludeEmbeddings:      cfg.IncludeEmbeddings,
		IncludeGenrePatchDebug: cfg.IncludeGenrePatchDebug,
		IncludeTempoDebug:      cfg.IncludeTempoDebug,
		ShortAnalysis:          shortAnalysis,
		AnalysisMaxSec:         45,
		PreferCenterWindow:     true,
	})
	if err != nil {
		tr.OK = false
		tr.Error = err.Error()
		tr.Warnings = append(tr.Warnings, "probe_error")
		return tr
	}

	tr.OK = true
	tr.Audio = probe.Audio
	tr.Tempo = probe.Tempo
	tr.Essentia = probe.Essentia
	tr.FinalFeatures = probe.Features
	tr.Warnings = append(tr.Warnings, probe.Warnings...)

	if useTempoEngine {
		if tempoEngine == nil {
			tr.Warnings = append(tr.Warnings, "tempo_engine_nil")
		} else if !tempoEngine.Ready() {
			tr.Warnings = append(tr.Warnings, "tempo_engine_not_ready")
		} else if tempoRes, err := tempoEngine.AnalyzePath(ctx, path); err == nil {
			tr.Tempo.BPM = tempoRes.BPM
			tr.Tempo.BPMPerceived = tempoRes.BPMPerceived
			tr.Tempo.Confidence = tempoRes.Confidence
			tr.Tempo.Stability = tempoRes.Stability
			tr.Tempo.Source = tempoRes.Source
			tr.Tempo.Patches = len(tempoRes.LocalBPM)
		} else {
			tr.Tempo.Error = err.Error()
			tr.Warnings = append(tr.Warnings, "tempo_error")
		}
	}

	if !quiet {
		fmt.Printf(
			"tempo final file=%q bpm=%.2f pbpm=%.2f conf=%.3f stab=%.3f src=%q err=%q\n",
			tr.FileName,
			tr.Tempo.BPM,
			tr.Tempo.BPMPerceived,
			tr.Tempo.Confidence,
			tr.Tempo.Stability,
			tr.Tempo.Source,
			tr.Tempo.Error,
		)
	}

	return tr
}

func toShortTrack(tr TrackProbeReport) ShortTrackProbeReport {
	out := ShortTrackProbeReport{
		File:  tr.FileName,
		OK:    tr.OK,
		Error: tr.Error,
		Warn:  compactWarnings(tr.Warnings),
	}
	if !tr.OK {
		return out
	}

	out.Dur = r2(tr.Audio.DurationSec)
	out.AnalyzedSec = r2(tr.Audio.DurationSec)
	out.BPM = r2(tr.Tempo.BPM)
	out.PBPM = r2(tr.Tempo.BPMPerceived)
	out.TConf = r3(tr.Tempo.Confidence)
	out.TStab = r3(tr.Tempo.Stability)
	out.TSrc = tr.Tempo.Source
	out.TTrust = r3(shortTempoTrust(out.BPM, out.TConf, out.TStab, out.TSrc))

	out.Genre = compactGenre(tr.Essentia.Genre)
	out.GScore = r3(tr.Essentia.Genre.Score)
	out.GMargin = r3(tr.Essentia.Genre.Margin)

	f := tr.FinalFeatures
	out.F = ShortFeatureValues{
		Energy:   r3(f.Energy),
		Dance:    r3(f.Danceability),
		Val:      r3(f.Valence),
		Happy:    r3(f.Happy),
		Sad:      r3(f.Sad),
		Relax:    r3(f.Relaxed),
		Party:    r3(f.Party),
		Aggr:     r3(f.Aggressive),
		Acoustic: r3(f.Acoustic),
		Elec:     r3(f.Electronic),
		Instr:    r3(f.Instrumental),
		Vocal:    r3(f.Vocal),
		Mel:      r3(f.Melodic),
		Soft:     r3(f.Soft),
		Heavy:    r3(f.Heavy),
		Dream:    r3(f.Dream),
		Emo:      r3(f.Emotional),
		Bright:   r3(f.Brightness),
		Tonal:    r3(f.Tonality),
		Approach: r3(f.Approachability),
		Engage:   r3(f.Engagement),
	}

	out.Audio = ShortAudioTexture{
		Loudness: r2(f.Loudness),
		Centroid: r2(f.SpectralCentroid),
		ZCR:      r3(f.ZeroCrossingRate),
		RMS:      r3(f.RMS),
	}
	out.Audio2 = ShortAudioTexture2{
		Flatness:     r3(f.SpectralFlatness),
		Rolloff85:    r2(f.SpectralRolloff85),
		Flux:         r3(f.SpectralFlux),
		OnsetRate:    r3(f.OnsetRate),
		DynamicRange: r3(f.DynamicRange),
		LowBand:      r3(f.LowBandRatio),
		MidBand:      r3(f.MidBandRatio),
		HighBand:     r3(f.HighBandRatio),
	}
	out.Audio2Quality = ShortAudio2Quality{
		FlatOK:     f.SpectralFlatness > 0 && f.SpectralFlatness < 0.999,
		RollOK:     f.SpectralRolloff85 >= 80 && f.SpectralRolloff85 <= 8000,
		FluxOK:     f.SpectralFlux > 0,
		BandsOK:    f.LowBandRatio+f.MidBandRatio+f.HighBandRatio > 0.95,
		CentroidOK: f.SpectralCentroid > 50,
	}
	out.Timing = tr.Timing
	out.Heads = shortHeadValues(tr.Essentia.Heads)
	out.Norm = shortNormValues(out)
	// basis3 is computed after batch stats are available
	out.Basis3Debug = Basis3Debug{}

	if out.BPM > 0 && out.TConf <= 0 {
		out.Warn = append(out.Warn, "tempo_conf_zero")
	}
	if out.TSrc == "" && out.BPM > 0 {
		out.Warn = append(out.Warn, "tempo_source_empty")
	}

	return out
}

func shortTempoTrust(bpm, conf, stab float64, src string) float64 {
	if bpm <= 0 {
		return 0
	}
	trust := 0.35
	if strings.EqualFold(src, "tempocnn") {
		trust = 0.55
	}
	if conf > 0 {
		trust = math.Max(trust, conf)
	}
	if stab > 0 {
		trust = 0.65*trust + 0.35*stab
	}
	if conf <= 0 && stab <= 0 {
		trust = math.Min(trust, 0.35)
	}
	return clamp01(trust)
}

func shortTrackToEmotionInputs(t ShortTrackProbeReport) emotion.Inputs {
	return emotion.Inputs{
		Dance:             blendSupport(t.F.Dance, t.Heads.DanceSupport),
		Valence:           t.F.Val,
		Happy:             t.F.Happy,
		Sad:               t.F.Sad,
		Relaxed:           blendSupport(t.F.Relax, t.Heads.RelaxSupport),
		Party:             blendSupport(t.F.Party, t.Heads.PartySupport),
		Aggressive:        blendSupport(t.F.Aggr, t.Heads.AggrSupport),
		Acousticness:      t.F.Acoustic,
		Electronicness:    blendSupport(t.F.Elec, t.Heads.ElecSupport),
		Instrumentalness:  t.F.Instr,
		Vocalness:         t.F.Vocal,
		Brightness:        t.F.Bright,
		Tonality:          t.F.Tonal,
		Melodicness:       t.F.Mel,
		Softness:          t.F.Soft,
		Heaviness:         t.F.Heavy,
		Dreaminess:        t.F.Dream,
		Emotionality:      t.F.Emo,
		BPM:               t.BPM,
		BPMPerceived:      t.PBPM,
		TempoConfidence:   t.TConf,
		TempoStability:    t.TStab,
		Loudness:          t.Audio.Loudness,
		RMS:               t.Audio.RMS,
		ZeroCrossingRate:  t.Audio.ZCR,
		SpectralCentroid:  t.Audio.Centroid,
		SpectralFlatness:  t.Audio2.Flatness,
		SpectralRolloff85: t.Audio2.Rolloff85,
		SpectralFlux:      t.Audio2.Flux,
		OnsetRate:         t.Audio2.OnsetRate,
		DynamicRange:      t.Audio2.DynamicRange,
		LowBandRatio:      t.Audio2.LowBand,
		MidBandRatio:      t.Audio2.MidBand,
		HighBandRatio:     t.Audio2.HighBand,
	}
}

type shortStatsNormalizer struct{ st ShortCalibrationStats }

func (n shortStatsNormalizer) Norm(name string, value float64) float64 {
	switch name {
	case "ZeroCrossingRate":
		return normStat(value, n.st.ZCR)
	case "SpectralCentroid":
		return normStat(value, n.st.Centroid)
	case "SpectralFlatness":
		return normStat(value, n.st.Flatness)
	case "SpectralRolloff85":
		return normStat(value, n.st.Rolloff85)
	case "SpectralFlux":
		return normStat(value, n.st.Flux)
	case "OnsetRate":
		return normStat(value, n.st.Onset)
	case "Loudness":
		return normStat(value, n.st.Loudness)
	case "RMS":
		return normStat(value, n.st.RMS)
	default:
		return fallbackShortNorm(name, value)
	}
}

func (n shortStatsNormalizer) NormWeighted(name string, value float64) float64 {
	if normalized, ok := emotion.NormalizeSemanticFeature(name, value); ok {
		return normalized
	}
	return n.Norm(name, value)
}
func (n shortStatsNormalizer) Reliability(name string) float64 {
	if trust, ok := emotion.SemanticFeatureTrust(name); ok {
		return trust
	}
	return 1
}

func fallbackShortNorm(name string, v float64) float64 {
	switch name {
	case "Loudness":
		return clamp01((v + 60.0) / 35.0)
	case "RMS":
		return clamp01((v - 0.05) / 0.45)
	case "ZeroCrossingRate":
		return clamp01(v / 0.22)
	case "SpectralCentroid":
		return clamp01((v - 200.0) / 5000.0)
	case "SpectralRolloff85":
		return clamp01((v - 100.0) / 6000.0)
	case "SpectralFlatness":
		return clamp01(v / 0.08)
	case "SpectralFlux":
		return clamp01(v / 0.45)
	case "OnsetRate":
		return clamp01(v / 3.0)
	case "DynamicRange", "LowBandRatio", "MidBandRatio", "HighBandRatio":
		return clamp01(v)
	default:
		return clamp01(v)
	}
}

func emotionDebugToShortDebug(d emotion.Debug) Basis3Debug {
	out := Basis3Debug{
		CleanBright:    r3(d.CleanBright),
		CleanParty:     r3(d.CleanParty),
		Intimacy:       r3(d.Intimacy),
		RoughRaw:       r3(d.RoughRaw),
		CombatRaw:      r3(d.CombatRaw),
		JoyRaw:         r3(d.JoyRaw),
		JoyCombatCut:   r3(d.JoyCombatCut),
		JoyRoughCut:    r3(d.JoyRoughCut),
		JoyPressureCut: r3(d.JoyPressureCut),
		JoyEdgeCut:     r3(d.JoyEdgeCut),
		JoyDirtyCut:    r3(d.JoyDirtyCut),
		JoyCleanBoost:  r3(d.JoyCleanBoost),
		MelancholyRaw:  r3(d.MelancholyRaw),
		SerenityRaw:    r3(d.SerenityRaw),
		DirtyElectro:   r3(d.DirtyElectro),
		HeavyDrive:     r3(d.HeavyDrive),
		TensePressure:  r3(d.TensePressure),
		WarmGroove:     r3(d.WarmGroove),
		SereneBright:   r3(d.SereneBright),
		JoyConfidence:  r3(d.JoyConfidence),
		VocalGrief:     r3(d.VocalGrief),
		DramaticArc:    r3(d.DramaticArc),
		Decision:       d.Decision,
	}
	out.TopLabels = make([]LabelScore, 0, len(d.TopLabels))
	for _, it := range d.TopLabels {
		out.TopLabels = append(out.TopLabels, LabelScore{Label: it.Label, Score: r3(it.Score), Passed: it.Passed, Reason: it.Reason})
	}
	return out
}

func shortNormValues(t ShortTrackProbeReport) ShortNormValues {
	dent := 0.0
	if t.Audio.Centroid > 50 {
		dent = clamp01((t.Audio.Centroid - 1000.0) / 2600.0)
	}
	return ShortNormValues{
		Dance:       r3(clamp01(t.F.Dance)),
		Party:       r3(clamp01(t.F.Party)),
		Relax:       r3(clamp01(t.F.Relax)),
		Elec:        r3(clamp01(t.F.Elec)),
		ZCR:         r3(clamp01(t.Audio.ZCR / 0.20)),
		Centroid:    r3(dent),
		Density:     r3(clamp01(0.62*t.F.Energy + 0.22*clamp01(t.Audio.RMS*2.0) + 0.16*clamp01((60+t.Audio.Loudness)/60))),
		GrooveTempo: r3(clamp01(t.TTrust * tempoPulseShort(t.BPM, t.TConf))),
		SpeedTempo:  r3(clamp01((t.BPM - 115.0) / 20.0)),
		TempoTrust:  r3(clamp01(t.TTrust)),
	}
}

func compactGenre(g onnx.GenreProbeReport) string {
	primary := strings.TrimSpace(g.Primary)
	detail := strings.TrimSpace(g.Detail)
	label := strings.TrimSpace(g.Label)
	if primary != "" {
		if detail != "" {
			return primary + " / " + detail
		}
		return primary
	}
	if label != "" {
		return label
	}
	return ""
}

func compactWarnings(ws []string) []string {
	if len(ws) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		keep := false
		switch w {
		case "genre_weak", "tempo_error", "probe_error", "tempo_engine_nil", "tempo_engine_not_ready", "tempo_conf_zero", "tempo_source_empty", "regression_out_of_range", "approach_saturated", "engage_saturated", "sad_saturated", "aggressive_underfires", "binary_like", "near_zero_saturated", "near_one_saturated":
			keep = true
		default:
			if strings.Contains(w, "genre") || strings.Contains(w, "tempo") || strings.Contains(w, "regression") || strings.Contains(w, "saturated") {
				keep = true
			}
		}
		if keep && !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

func shortHeadValues(heads []onnx.HeadProbeReport) ShortHeadValues {
	var out ShortHeadValues
	for _, h := range heads {
		name := strings.ToLower(h.Name)
		v := r3(h.PositiveStats.Support50)
		if v == 0 {
			v = r3(h.PositiveStats.Support30)
		}
		switch {
		case strings.Contains(name, "danceability"):
			out.DanceSupport = v
		case strings.Contains(name, "mood_party"):
			out.PartySupport = v
		case strings.Contains(name, "mood_relaxed"):
			out.RelaxSupport = v
		case strings.Contains(name, "mood_electronic"):
			out.ElecSupport = v
		case strings.Contains(name, "mood_acoustic"):
			out.AcousticSupport = v
		case strings.Contains(name, "mood_aggressive"):
			out.AggrSupport = v
		case strings.Contains(name, "mood_sad"):
			out.SadSupport = v
		case strings.Contains(name, "mood_happy"):
			out.HappySupport = v
		}
	}
	return out
}

func computeShortBasisFromFinalFeatures(t ShortTrackProbeReport) ShortMoodBasis {
	f := t.F
	n := t.Norm
	if n.TempoTrust == 0 && t.TTrust > 0 {
		n = shortNormValues(t)
	}

	motion := clamp01(
		0.36*f.Dance +
			0.22*n.GrooveTempo +
			0.14*f.Party +
			0.12*f.Elec +
			0.08*(1.0-f.Soft) +
			0.08*n.TempoTrust,
	)

	edge := clamp01(
		0.34*n.ZCR +
			0.28*n.Centroid +
			0.10*f.Bright +
			0.08*f.Elec +
			0.08*(1.0-f.Soft) +
			0.06*(1.0-f.Mel) +
			0.04*f.Aggr +
			0.02*f.Heavy,
	)

	impact := clamp01(
		0.42*n.Centroid +
			0.28*n.ZCR +
			0.18*f.Aggr +
			0.12*edge,
	)

	pressure := clamp01(
		0.28*impact +
			0.26*edge +
			0.18*motion +
			0.14*n.Density +
			0.08*n.SpeedTempo +
			0.06*(1.0-f.Relax),
	)

	calm := clamp01(
		0.34*f.Relax +
			0.22*f.Soft +
			0.18*(1.0-motion) +
			0.18*(1.0-pressure) +
			0.08*(1.0-edge),
	)

	cool := clamp01(
		0.30*(1.0-f.Bright) +
			0.18*(1.0-f.Val) +
			0.16*n.Density +
			0.16*f.Relax +
			0.10*(1.0-f.Party) +
			0.10*(1.0-impact),
	)

	warm := clamp01(
		0.34*f.Val +
			0.24*f.Bright +
			0.16*f.Happy +
			0.14*f.Party +
			0.12*(1.0-cool),
	)

	b := ShortMoodBasis{Motion: r3(motion), Pressure: r3(pressure), Edge: r3(edge), Calm: r3(calm), Cool: r3(cool), Warm: r3(warm), Texture: r3(n.Density)}
	b.Label = shortMoodLabel(b)
	b.Hue = r2(shortHue(b))
	return b
}

func normStat(x float64, st ShortStat) float64 {
	if st.P95 <= st.P05+1e-9 {
		return 0.5
	}
	return clamp01((x - st.P05) / (st.P95 - st.P05))
}

func boolWeight(ok bool) float64 {
	if ok {
		return 1.0
	}
	return 0.0
}
func missingNeutral(ok bool, value float64) float64 {
	if !ok {
		return 0.5
	}
	return clamp01(value)
}

func computeBasis3(t ShortTrackProbeReport, st ShortCalibrationStats) (ShortNormValues2, ShortBasis3, Basis3Debug) {
	res := emotion.ComputeFromInputs(shortTrackToEmotionInputs(t), shortStatsNormalizer{st: st})
	b := res.Basis
	d := res.Debug
	bpm := t.PBPM
	if bpm <= 0 {
		bpm = t.BPM
	}
	sprintTempoN := 0.0
	for _, x := range tempoCandidatesShort(bpm) {
		sprintTempoN = math.Max(sprintTempoN, clamp01(1.0-math.Abs(x-138.0)/42.0))
	}
	sprintTempoN *= clamp(t.TTrust, 0.35, 1.0)
	norm := ShortNormValues2{
		ZCR:         r3(normStat(t.Audio.ZCR, st.ZCR)),
		Centroid:    r3(normStat(t.Audio.Centroid, st.Centroid)),
		Flatness:    r3(normStat(t.Audio2.Flatness, st.Flatness)),
		Rolloff85:   r3(normStat(t.Audio2.Rolloff85, st.Rolloff85)),
		Flux:        r3(normStat(t.Audio2.Flux, st.Flux)),
		Onset:       r3(normStat(t.Audio2.OnsetRate, st.Onset)),
		Density:     r3(b.Density),
		GrooveTempo: r3(b.Pulse),
		SprintTempo: r3(sprintTempoN),
		TempoTrust:  r3(t.TTrust),
	}
	basis := ShortBasis3{
		Motion:      r3(b.Motion),
		Pulse:       r3(b.Pulse),
		Density:     r3(b.Density),
		Roughness:   r3(b.Roughness),
		Brightness:  r3(b.Brightness),
		Smoothness:  r3(b.Smoothness),
		Impact:      r3(b.Impact),
		Pressure:    r3(b.Pressure),
		Joy:         r3(b.Joy),
		Melancholy:  r3(b.Melancholy),
		Serenity:    r3(b.Serenity),
		Swagger:     r3(b.Swagger),
		Combat:      r3(b.Combat),
		Sprint:      r3(b.Sprint),
		SprintClean: r3(b.SprintClean),
		Label:       b.Label,
	}
	return norm, basis, emotionDebugToShortDebug(d)
}

func shortBasisToEmotionBasis(b ShortBasis3) emotion.Basis {
	return emotion.Basis{
		Motion:      b.Motion,
		Pulse:       b.Pulse,
		Density:     b.Density,
		Roughness:   b.Roughness,
		Brightness:  b.Brightness,
		Smoothness:  b.Smoothness,
		Impact:      b.Impact,
		Pressure:    b.Pressure,
		Joy:         b.Joy,
		Melancholy:  b.Melancholy,
		Serenity:    b.Serenity,
		Swagger:     b.Swagger,
		Combat:      b.Combat,
		Sprint:      b.Sprint,
		SprintClean: b.SprintClean,
		Label:       b.Label,
	}
}

func basis3DebugFromTrackShort(cleanBright, intimacy, roughRaw, combatRaw, joyRaw, melancholyRaw, serenity, dirtyElectro, tensePressure, warmGroove, sereneBright, joyConfidence, vocalGrief, dramaticArc float64) Basis3Debug {
	return Basis3Debug{CleanBright: cleanBright, Intimacy: intimacy, RoughRaw: roughRaw, CombatRaw: combatRaw, JoyRaw: joyRaw, MelancholyRaw: melancholyRaw, SerenityRaw: serenity, DirtyElectro: dirtyElectro, TensePressure: tensePressure, WarmGroove: warmGroove, SereneBright: sereneBright, JoyConfidence: joyConfidence, VocalGrief: vocalGrief, DramaticArc: dramaticArc}
}

func topLabelScores(b ShortBasis3, d Basis3Debug) []LabelScore {
	items := emotion.TopLabelScoresWithDebug(shortBasisToEmotionBasis(b), shortDebugToEmotionDebug(d))
	out := make([]LabelScore, 0, len(items))
	for _, it := range items {
		out = append(out, LabelScore{Label: it.Label, Score: it.Score, Passed: it.Passed, Reason: it.Reason})
	}
	return out
}

func basis3Label(b ShortBasis3, d Basis3Debug) string {
	return emotion.LabelWithDebug(shortBasisToEmotionBasis(b), shortDebugToEmotionDebug(d))
}

func shortDebugToEmotionDebug(d Basis3Debug) emotion.Debug {
	return emotion.Debug{
		CleanBright:    d.CleanBright,
		CleanParty:     d.CleanParty,
		Intimacy:       d.Intimacy,
		RoughRaw:       d.RoughRaw,
		CombatRaw:      d.CombatRaw,
		JoyRaw:         d.JoyRaw,
		JoyCombatCut:   d.JoyCombatCut,
		JoyRoughCut:    d.JoyRoughCut,
		JoyPressureCut: d.JoyPressureCut,
		JoyEdgeCut:     d.JoyEdgeCut,
		JoyDirtyCut:    d.JoyDirtyCut,
		JoyCleanBoost:  d.JoyCleanBoost,
		MelancholyRaw:  d.MelancholyRaw,
		SerenityRaw:    d.SerenityRaw,
		DirtyElectro:   d.DirtyElectro,
		TensePressure:  d.TensePressure,
		WarmGroove:     d.WarmGroove,
		SereneBright:   d.SereneBright,
		JoyConfidence:  d.JoyConfidence,
		VocalGrief:     d.VocalGrief,
		DramaticArc:    d.DramaticArc,
	}
}

func blendSupport(a, b float64) float64 {
	if b > 0 {
		return (a + b) / 2
	}
	return a
}
func densityFromAudio(loudness, rms float64) float64 {
	return clamp01(0.62*clamp01((60+loudness)/60) + 0.38*clamp01(rms*2.0))
}
func grooveTempo(bpm, trust float64) float64 {
	if bpm <= 0 {
		return 0.5
	}
	return clamp01(trust * tempoPulseShort(bpm, trust))
}
func sprintTempo(bpm, trust float64) float64 {
	if bpm <= 0 {
		return 0
	}
	return clamp01(((bpm - 115.0) / 20.0) * trust)
}

type ShortCalibrationStats struct{ ZCR, Centroid, Flatness, Rolloff85, Flux, Onset, Loudness, RMS ShortStat }

func buildShortCalibrationStats(tracks []ShortTrackProbeReport) ShortCalibrationStats {
	var st ShortCalibrationStats
	collect := func(ok func(ShortTrackProbeReport) bool, get func(ShortTrackProbeReport) float64) ShortStat {
		xs := make([]float64, 0)
		for _, tr := range tracks {
			if tr.OK && ok(tr) {
				xs = append(xs, get(tr))
			}
		}
		return shortStat(xs)
	}
	st.ZCR = collect(func(tr ShortTrackProbeReport) bool { return true }, func(tr ShortTrackProbeReport) float64 { return tr.Audio.ZCR })
	st.Centroid = collect(func(tr ShortTrackProbeReport) bool { return tr.Audio.Centroid > 50 }, func(tr ShortTrackProbeReport) float64 { return tr.Audio.Centroid })
	st.Flatness = collect(func(tr ShortTrackProbeReport) bool { return tr.Audio2.Flatness > 0 }, func(tr ShortTrackProbeReport) float64 { return tr.Audio2.Flatness })
	st.Rolloff85 = collect(func(tr ShortTrackProbeReport) bool { return tr.Audio2.Rolloff85 > 50 }, func(tr ShortTrackProbeReport) float64 { return tr.Audio2.Rolloff85 })
	st.Flux = collect(func(tr ShortTrackProbeReport) bool { return tr.Audio2.Flux > 0 }, func(tr ShortTrackProbeReport) float64 { return tr.Audio2.Flux })
	st.Onset = collect(func(tr ShortTrackProbeReport) bool { return tr.Audio2.OnsetRate > 0 }, func(tr ShortTrackProbeReport) float64 { return tr.Audio2.OnsetRate })
	st.Loudness = collect(func(tr ShortTrackProbeReport) bool { return true }, func(tr ShortTrackProbeReport) float64 { return tr.Audio.Loudness })
	st.RMS = collect(func(tr ShortTrackProbeReport) bool { return true }, func(tr ShortTrackProbeReport) float64 { return tr.Audio.RMS })
	return st
}

func tempoPulseShort(bpm, conf float64) float64 {
	if bpm <= 0 || conf < 0.35 {
		return 0.5
	}
	center := 118.0
	width := 38.0
	x := 1.0 - math.Abs(bpm-center)/width
	return clamp01(0.35 + 0.65*x)
}

func tempoCandidatesShort(bpm float64) []float64 {
	if bpm <= 0 {
		return nil
	}
	out := []float64{bpm}
	if bpm < 95 {
		out = append(out, bpm*2)
	}
	if bpm > 150 {
		out = append(out, bpm/2)
	}
	return out
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func shortMoodLabel(m ShortMoodBasis) string {
	if m.Edge > 0.70 && m.Pressure > 0.66 {
		return "combat_drive"
	}
	if m.Edge > 0.58 && m.Pressure > 0.56 {
		return "dirty_drive"
	}
	if m.Motion > 0.65 && m.Cool > 0.52 && m.Pressure < 0.66 {
		return "street_groove"
	}
	if m.Cool > 0.58 && m.Edge < 0.52 && m.Pressure < 0.62 {
		return "night_smooth"
	}
	if m.Motion > 0.74 && m.Pressure > 0.66 {
		return "speed_drive"
	}
	if m.Calm > 0.62 && m.Pressure < 0.55 {
		return "calm_flow"
	}
	return "steady_groove"
}

func shortHue(m ShortMoodBasis) float64 {
	switch shortMoodLabel(m) {
	case "combat_drive":
		return 16
	case "dirty_drive":
		return 24
	case "speed_drive":
		return 30
	case "street_groove":
		return 250
	case "night_smooth":
		return 255
	case "calm_flow":
		return 225
	default:
		return 230
	}
}

func writeAnyReport(path string, report any, pretty bool) {
	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(report, "", "  ")
	} else {
		data, err = json.Marshal(report)
	}
	must(err)
	must(os.WriteFile(path, data, 0o644))
}

type expectedLabelLookup struct {
	exact map[string]string
	norm  map[string]string
}

func (l expectedLabelLookup) Lookup(file string) (string, bool) {
	if len(l.exact) == 0 && len(l.norm) == 0 {
		return "", false
	}
	base := filepath.Base(file)
	if exp := strings.TrimSpace(l.exact[file]); exp != "" {
		return exp, true
	}
	if exp := strings.TrimSpace(l.exact[base]); exp != "" {
		return exp, true
	}
	norm := normalizeProbeName(base)
	if norm == "" {
		norm = normalizeProbeName(file)
	}
	if norm == "" {
		return "", false
	}
	if exp := strings.TrimSpace(l.norm[norm]); exp != "" {
		return exp, true
	}
	for k, exp := range l.norm {
		if k == norm {
			return exp, true
		}
		if strings.HasPrefix(k, norm) || strings.HasPrefix(norm, k) {
			return exp, true
		}
		if strings.Contains(k, norm) || strings.Contains(norm, k) {
			return exp, true
		}
	}
	return "", false
}

func loadExpectedLabels(path string) expectedLabelLookup {
	path = strings.TrimSpace(path)
	if path == "" {
		return expectedLabelLookup{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return expectedLabelLookup{}
	}
	rawMap := map[string]string{}
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return expectedLabelLookup{}
	}
	lookup := expectedLabelLookup{exact: map[string]string{}, norm: map[string]string{}}
	for k, v := range rawMap {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		lookup.exact[k] = v
		lookup.exact[filepath.Base(k)] = v
		for _, nk := range []string{normalizeProbeName(k), normalizeProbeName(filepath.Base(k))} {
			if nk != "" {
				lookup.norm[nk] = v
			}
		}
	}
	return lookup
}

func parseExpectedPath(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-expected" && i+1 < len(args) {
			return args[i+1], true
		}
		if strings.HasPrefix(arg, "-expected=") {
			return strings.TrimPrefix(arg, "-expected="), true
		}
	}
	return "", false
}

func normalizeProbeName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, filepath.Ext(s))
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "(", " ")
	s = strings.ReplaceAll(s, ")", " ")
	s = strings.ReplaceAll(s, "[", " ")
	s = strings.ReplaceAll(s, "]", " ")
	s = strings.ReplaceAll(s, "{", " ")
	s = strings.ReplaceAll(s, "}", " ")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, ".", " ")
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		switch p {
		case "from", "www", "lightaudio", "ru", "mp3":
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}

func validateShortReport(r ShortBatchAudioProbeReport) []string {
	var out []string
	ok := 0
	tempoConfZero := 0
	centroidMissing := 0
	energySat := 0
	for _, tr := range r.Tracks {
		if !tr.OK {
			continue
		}
		ok++
		if tr.BPM > 0 && tr.TConf <= 0 {
			tempoConfZero++
		}
		if tr.Audio.Centroid <= 50 {
			centroidMissing++
		}
		if tr.F.Energy >= 0.999 {
			energySat++
		}
	}
	if ok == 0 {
		return append(out, "no_ok_tracks")
	}
	if float64(tempoConfZero)/float64(ok) > 0.5 {
		out = append(out, "batch_tempo_conf_mostly_zero")
	}
	if float64(centroidMissing)/float64(ok) > 0.2 {
		out = append(out, "batch_centroid_many_missing")
	}
	if float64(energySat)/float64(ok) > 0.8 {
		out = append(out, "batch_energy_saturated")
	}
	return out
}

var compatibleLabels = map[string][]string{
	"joy_party":            {"joy_funk", "street_swagger"},
	"joy_funk":             {"joy_party", "street_swagger"},
	"serene_calm":          {"serene_warm_groove", "serene_bright", "melancholy_calm"},
	"melancholy_calm":      {"melancholy_grief", "serene_calm"},
	"combat_force":         {"dirty_electro_combat", "tense_pressure"},
	"dirty_electro_combat": {"combat_force", "dirty_drive"},
	"night_smooth":         {"serene_warm_groove", "serene_calm"},
	"uplift_drive":         {"street_swagger", "joy_party", "speed_flight"},
}

func isCompatibleLabel(expected, got string) bool {
	if expected == "" || got == "" {
		return false
	}
	if expected == got {
		return true
	}
	for _, label := range compatibleLabels[expected] {
		if label == got {
			return true
		}
	}
	return false
}

func buildShortSummary(tracks []ShortTrackProbeReport, expected expectedLabelLookup) ShortBatchSummary {
	features := map[string][]float64{}
	warnings := map[string]int{}
	labels := map[string]int{}
	confusion := map[string]map[string]int{}
	errorsList := []string{}
	expectedTotal := 0
	mismatches := make([]ShortCalibrationMismatch, 0)
	for _, tr := range tracks {
		if !tr.OK {
			errorsList = append(errorsList, tr.File)
			continue
		}
		features["bpm"] = append(features["bpm"], tr.BPM)
		features["energy"] = append(features["energy"], tr.F.Energy)
		features["dance"] = append(features["dance"], tr.F.Dance)
		features["party"] = append(features["party"], tr.F.Party)
		features["relax"] = append(features["relax"], tr.F.Relax)
		features["edge"] = append(features["edge"], tr.Basis.Edge)
		features["pressure"] = append(features["pressure"], tr.Basis.Pressure)
		features["motion"] = append(features["motion"], tr.Basis.Motion)
		features["cool"] = append(features["cool"], tr.Basis.Cool)
		features["calm"] = append(features["calm"], tr.Basis.Calm)
		features["centroid"] = append(features["centroid"], tr.Audio.Centroid)
		features["zcr"] = append(features["zcr"], tr.Audio.ZCR)
		features["loudness"] = append(features["loudness"], tr.Audio.Loudness)
		features["rough"] = append(features["rough"], tr.Basis3.Roughness)
		if tr.Audio2Quality.FlatOK && tr.Audio2.Flatness >= 0.999 {
			warnings["flatness_exact_one"]++
		}
		if tr.Audio2Quality.RollOK && (tr.Audio2.Rolloff85 < 50 || tr.Audio2.Rolloff85 > 8000) {
			warnings["rolloff_suspicious"]++
		}
		if tr.Audio2Quality.BandsOK {
			bandSum := tr.Audio2.LowBand + tr.Audio2.MidBand + tr.Audio2.HighBand
			if math.Abs(bandSum-1.0) > 0.08 {
				warnings["band_ratio_sum_bad"]++
			}
		}
		if !tr.Audio2Quality.FlatOK {
			warnings["flatness_missing"]++
		}
		if !tr.Audio2Quality.RollOK {
			warnings["rolloff_missing"]++
		}
		if !tr.Audio2Quality.FluxOK {
			warnings["flux_missing"]++
		}
		if !tr.Audio2Quality.BandsOK {
			warnings["bands_missing"]++
		}
		features["bright"] = append(features["bright"], tr.Basis3.Brightness)
		features["smooth"] = append(features["smooth"], tr.Basis3.Smoothness)
		features["impact"] = append(features["impact"], tr.Basis3.Impact)
		features["pressure3"] = append(features["pressure3"], tr.Basis3.Pressure)
		features["joy"] = append(features["joy"], tr.Basis3.Joy)
		features["melancholy"] = append(features["melancholy"], tr.Basis3.Melancholy)
		features["serenity"] = append(features["serenity"], tr.Basis3.Serenity)
		features["swagger"] = append(features["swagger"], tr.Basis3.Swagger)
		features["combat"] = append(features["combat"], tr.Basis3.Combat)
		features["sprintClean"] = append(features["sprintClean"], tr.Basis3.SprintClean)
		features["flatness"] = append(features["flatness"], tr.Audio2.Flatness)
		features["rolloff85"] = append(features["rolloff85"], tr.Audio2.Rolloff85)
		features["flux"] = append(features["flux"], tr.Audio2.Flux)
		features["onset"] = append(features["onset"], tr.Audio2.OnsetRate)
		features["dyn"] = append(features["dyn"], tr.Audio2.DynamicRange)
		features["lowBand"] = append(features["lowBand"], tr.Audio2.LowBand)
		features["highBand"] = append(features["highBand"], tr.Audio2.HighBand)
		labels[tr.Basis3.Label]++
		if exp, ok := expected.Lookup(tr.File); ok {
			expectedTotal++
			if confusion[exp] == nil {
				confusion[exp] = map[string]int{}
			}
			confusion[exp][tr.Basis3.Label]++
		}
		for _, w := range tr.Warn {
			warnings[w]++
		}
	}
	out := ShortBatchSummary{FeatureStats: map[string]ShortStat{}, LabelCounts: labels, Warnings: warnings, Errors: errorsList, Confusion: confusion}
	for k, xs := range features {
		out.FeatureStats[k] = shortStat(xs)
	}
	if expectedTotal > 0 {
		exactMatches := 0
		compatibleMatches := 0
		for _, tr := range tracks {
			if !tr.OK {
				continue
			}
			exp, ok := expected.Lookup(tr.File)
			if !ok || strings.TrimSpace(exp) == "" {
				continue
			}
			exp = strings.TrimSpace(exp)
			if exp == tr.Basis3.Label {
				exactMatches++
			}
			if isCompatibleLabel(exp, tr.Basis3.Label) {
				compatibleMatches++
			} else {
				mismatches = append(mismatches, ShortCalibrationMismatch{File: tr.File, Expected: exp, Got: tr.Basis3.Label, Compatible: false})
			}
		}
		exactAccuracy := float64(exactMatches) / float64(expectedTotal)
		compatibleAccuracy := float64(compatibleMatches) / float64(expectedTotal)
		out.ExpectedAccuracy = exactAccuracy
		out.Calibration = ShortCalibrationSummary{
			ExpectedTotal:      expectedTotal,
			ExactMatches:       exactMatches,
			CompatibleMatches:  compatibleMatches,
			ExactAccuracy:      exactAccuracy,
			CompatibleAccuracy: compatibleAccuracy,
			Confusion:          confusion,
			Mismatches:         mismatches,
		}
	}
	if out.FeatureStats["joy"].P95-out.FeatureStats["joy"].P05 < 0.10 {
		out.SelfCheck = append(out.SelfCheck, "joy_no_spread")
	}
	if out.FeatureStats["combat"].P95-out.FeatureStats["combat"].P05 < 0.10 {
		out.SelfCheck = append(out.SelfCheck, "combat_no_spread")
	}
	if out.FeatureStats["melancholy"].P95-out.FeatureStats["melancholy"].P05 < 0.10 {
		out.SelfCheck = append(out.SelfCheck, "melancholy_no_spread")
	}
	if out.FeatureStats["serenity"].P95-out.FeatureStats["serenity"].P05 < 0.10 {
		out.SelfCheck = append(out.SelfCheck, "serenity_no_spread")
	}
	if labels["steady_groove"] > len(tracks)/2 {
		out.SelfCheck = append(out.SelfCheck, "basis3_too_many_steady_groove")
	}
	if warnings["flatness_missing"] > len(tracks)/4 {
		out.SelfCheck = append(out.SelfCheck, "flatness_many_missing")
	}
	if warnings["rolloff_missing"] > len(tracks)/4 {
		out.SelfCheck = append(out.SelfCheck, "rolloff_many_missing")
	}
	if warnings["flux_missing"] > len(tracks)/4 {
		out.SelfCheck = append(out.SelfCheck, "flux_many_missing")
	}
	if warnings["band_ratio_sum_bad"] > len(tracks)/10 {
		out.SelfCheck = append(out.SelfCheck, "band_ratio_sum_many_bad")
	}
	return out
}

func shortStat(xs []float64) ShortStat {
	if len(xs) == 0 {
		return ShortStat{}
	}
	clean := append([]float64{}, xs...)
	sort.Float64s(clean)
	sum := 0.0
	for _, x := range clean {
		sum += x
	}
	mean := sum / float64(len(clean))
	return ShortStat{
		Count: len(clean),
		Mean:  r3(mean),
		P05:   r3(percentileSorted(clean, 0.05)),
		P50:   r3(percentileSorted(clean, 0.50)),
		P95:   r3(percentileSorted(clean, 0.95)),
		Min:   r3(clean[0]),
		Max:   r3(clean[len(clean)-1]),
	}
}

func ensureConfig(path string) (ProbeConfig, bool, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg := defaultConfig()
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return cfg, true, err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return cfg, true, err
		}
		return cfg, true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProbeConfig{}, false, err
	}
	var cfg ProbeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ProbeConfig{}, false, err
	}
	return cfg, false, nil
}

func defaultConfig() ProbeConfig {
	return ProbeConfig{
		ModelsDir:              "/Users/user/Dev/wails_test1/ray-player1/assets/models/essentia",
		ONNXRuntimePath:        "/Users/user/Dev/wails_test1/bm25-search1/assets/lib/libonnxruntime/macos-arm64/libonnxruntime.dylib",
		FFmpegPath:             "ffmpeg",
		MelMode:                "official",
		IncludeRawVectors:      false,
		IncludePatchRows:       true,
		MaxPatchRows:           8,
		IncludeEmbeddings:      false,
		IncludeGenrePatchDebug: true,
		IncludeTempoDebug:      true,
	}
}

func findMP3Files(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".mp3") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func buildBatchSummary(tracks []TrackProbeReport) onnx.BatchProbeSummary {
	features := map[string][]float64{}
	out := onnx.BatchProbeSummary{FeatureStats: map[string]onnx.Stat{}, HeadWarnings: map[string]int{}}
	for _, tr := range tracks {
		if !tr.OK {
			out.TracksWithErrors = append(out.TracksWithErrors, tr.FileName)
			continue
		}
		f := tr.FinalFeatures
		features["danceability"] = append(features["danceability"], f.Danceability)
		features["energy"] = append(features["energy"], f.Energy)
		features["valence"] = append(features["valence"], f.Valence)
		features["happy"] = append(features["happy"], f.Happy)
		features["sad"] = append(features["sad"], f.Sad)
		features["relaxed"] = append(features["relaxed"], f.Relaxed)
		features["party"] = append(features["party"], f.Party)
		features["aggressive"] = append(features["aggressive"], f.Aggressive)
		features["heavy"] = append(features["heavy"], f.Heavy)
		features["dream"] = append(features["dream"], f.Dream)
		features["emotional"] = append(features["emotional"], f.Emotional)
		features["brightness"] = append(features["brightness"], f.Brightness)
		features["tonality"] = append(features["tonality"], f.Tonality)
		features["approachability"] = append(features["approachability"], f.Approachability)
		features["engagement"] = append(features["engagement"], f.Engagement)
		for _, h := range tr.Essentia.Heads {
			for _, w := range h.Warnings {
				out.HeadWarnings[h.Name+":"+w]++
			}
		}
	}
	for name, xs := range features {
		out.FeatureStats[name] = stat(xs)
	}
	return out
}

func stat(xs []float64) onnx.Stat {
	if len(xs) == 0 {
		return onnx.Stat{}
	}
	clean := append([]float64{}, xs...)
	sort.Float64s(clean)
	mean := 0.0
	for _, x := range clean {
		mean += x
	}
	mean /= float64(len(clean))
	std := 0.0
	for _, x := range clean {
		d := x - mean
		std += d * d
	}
	std = math.Sqrt(std / float64(len(clean)))
	return onnx.Stat{Count: len(clean), Mean: mean, Median: percentileSorted(clean, 0.50), Trimmed10: trimmedMeanSorted(clean, 0.10), Min: clean[0], Max: clean[len(clean)-1], Std: std, P05: percentileSorted(clean, 0.05), P25: percentileSorted(clean, 0.25), P75: percentileSorted(clean, 0.75), P95: percentileSorted(clean, 0.95), Support30: supportRatio(clean, 0.30), Support50: supportRatio(clean, 0.50), Support70: supportRatio(clean, 0.70), NearZeroRatio: nearZeroRatio(clean), NearOneRatio: nearOneRatio(clean), Binaryness: binaryness(clean)}
}

func percentileSorted(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	if p <= 0 {
		return xs[0]
	}
	if p >= 1 {
		return xs[len(xs)-1]
	}
	pos := p * float64(len(xs)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return xs[lo]
	}
	f := pos - float64(lo)
	return xs[lo]*(1-f) + xs[hi]*f
}

func trimmedMeanSorted(xs []float64, trim float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	start := int(float64(len(xs)) * trim)
	end := len(xs) - start
	if start >= end {
		start = 0
		end = len(xs)
	}
	sum := 0.0
	for _, x := range xs[start:end] {
		sum += x
	}
	return sum / float64(end-start)
}

func supportRatio(xs []float64, threshold float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	n := 0
	for _, x := range xs {
		if x >= threshold {
			n++
		}
	}
	return float64(n) / float64(len(xs))
}

func nearZeroRatio(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	n := 0
	for _, x := range xs {
		if x <= 0.001 {
			n++
		}
	}
	return float64(n) / float64(len(xs))
}

func nearOneRatio(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	n := 0
	for _, x := range xs {
		if x >= 0.999 {
			n++
		}
	}
	return float64(n) / float64(len(xs))
}

func binaryness(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	n := 0
	for _, x := range xs {
		if x <= 0.001 || x >= 0.999 {
			n++
		}
	}
	return float64(n) / float64(len(xs))
}

func r3(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	return math.Round(x*1000) / 1000
}

func r2(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	return math.Round(x*100) / 100
}

func clamp01(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
