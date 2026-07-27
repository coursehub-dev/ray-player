package onnx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"ray-player1/internal/analysis"

	ort "ray-player1/internal/onnx/ortshim"
)

type ProbeOptions struct {
	MelMode                string
	IncludeRawVectors      bool
	IncludePatchRows       bool
	MaxPatchRows           int
	IncludeEmbeddings      bool
	IncludeGenrePatchDebug bool
	IncludeTempoDebug      bool
	ShortAnalysis          bool
	AnalysisMaxSec         float64
	PreferCenterWindow     bool
}

type ProbeTiming struct {
	DecodeFullMS  int64 `json:"decodeFullMs,omitempty"`
	DecodeShortMS int64 `json:"decodeShortMs,omitempty"`
	EssentiaMS    int64 `json:"essentiaMs,omitempty"`
	Audio2MS      int64 `json:"audio2Ms,omitempty"`
	TempoMS       int64 `json:"tempoMs,omitempty"`
	TotalMS       int64 `json:"totalMs,omitempty"`
}

type AudioDecodeProbe struct {
	SampleRate  int     `json:"sampleRate"`
	Channels    int     `json:"channels"`
	DurationSec float64 `json:"durationSec"`
	Samples     int     `json:"samples"`
}

type TempoProbe struct {
	BPM          float64 `json:"bpm"`
	BPMPerceived float64 `json:"bpmPerceived"`
	Confidence   float64 `json:"confidence"`
	Stability    float64 `json:"stability"`
	Source       string  `json:"source"`
	Error        string  `json:"error,omitempty"`
	Patches      int     `json:"patches,omitempty"`
}

type Stat struct {
	Count         int     `json:"count"`
	Mean          float64 `json:"mean"`
	Median        float64 `json:"median"`
	Trimmed10     float64 `json:"trimmed10"`
	Min           float64 `json:"min"`
	Max           float64 `json:"max"`
	Std           float64 `json:"std"`
	P05           float64 `json:"p05"`
	P25           float64 `json:"p25"`
	P75           float64 `json:"p75"`
	P95           float64 `json:"p95"`
	Support30     float64 `json:"support30"`
	Support50     float64 `json:"support50"`
	Support70     float64 `json:"support70"`
	NearZeroRatio float64 `json:"nearZeroRatio"`
	NearOneRatio  float64 `json:"nearOneRatio"`
	Binaryness    float64 `json:"binaryness"`
}

type OutputProbe struct {
	Name  string  `json:"name"`
	Shape []int64 `json:"shape"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
	Std   float64 `json:"std"`
}

type MelProbeReport struct {
	InputShape   []int64     `json:"inputShape"`
	ValidPatches int         `json:"validPatches"`
	PatchStats   []PatchStat `json:"patchStats,omitempty"`
}

type PatchStat struct {
	Index int     `json:"index"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
	RMS   float64 `json:"rms"`
}

type BaseModelReport struct {
	ModelName string        `json:"modelName"`
	Outputs   []OutputProbe `json:"outputs"`
}

type EmbeddingReport struct {
	Dim    int       `json:"dim"`
	Min    float64   `json:"min"`
	Max    float64   `json:"max"`
	Mean   float64   `json:"mean"`
	RMS    float64   `json:"rms"`
	Norm   float64   `json:"norm"`
	Vector []float64 `json:"vector,omitempty"`
}

type AggregationReport struct {
	ChosenValue float64 `json:"chosenValue"`
	ChosenMode  string  `json:"chosenMode"`
	Mean        float64 `json:"mean"`
	Median      float64 `json:"median"`
	Trimmed10   float64 `json:"trimmed10"`
	Support30   float64 `json:"support30"`
	Support50   float64 `json:"support50"`
	Support70   float64 `json:"support70"`
	Binaryness  float64 `json:"binaryness"`
}

type HeadProbeReport struct {
	Name          string            `json:"name"`
	ModelPath     string            `json:"modelPath,omitempty"`
	MetadataPath  string            `json:"metadataPath,omitempty"`
	InputName     string            `json:"inputName"`
	OutputName    string            `json:"outputName"`
	Classes       []string          `json:"classes,omitempty"`
	Shape         []int64           `json:"shape"`
	PositiveLabel string            `json:"positiveLabel,omitempty"`
	PositiveIndex int               `json:"positiveIndex,omitempty"`
	StatsByClass  map[string]Stat   `json:"statsByClass,omitempty"`
	PositiveStats Stat              `json:"positiveStats"`
	Aggregation   AggregationReport `json:"aggregation"`
	FirstRows     [][]float64       `json:"firstRows,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
}

type GenreClassScore struct {
	Index int     `json:"index"`
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type GenreGroupScore struct {
	Label          string  `json:"label"`
	Score          float64 `json:"score"`
	Support        int     `json:"support"`
	SumScore       float64 `json:"sumScore"`
	BestSubLabel   string  `json:"bestSubLabel"`
	BestSubScore   float64 `json:"bestSubScore"`
	SecondSubScore float64 `json:"secondSubScore,omitempty"`
}

type GenrePatchTop struct {
	Patch int               `json:"patch"`
	Top   []GenreClassScore `json:"top"`
}

type GenreProbeReport struct {
	ModelName  string            `json:"modelName"`
	OutputName string            `json:"outputName"`
	Shape      []int64           `json:"shape"`
	Primary    string            `json:"primary"`
	Label      string            `json:"label"`
	Detail     string            `json:"detail"`
	Score      float64           `json:"score"`
	Margin     float64           `json:"margin"`
	TopClasses []GenreClassScore `json:"topClasses"`
	Groups     []GenreGroupScore `json:"groups"`
	PatchTop   []GenrePatchTop   `json:"patchTop,omitempty"`
	Warnings   []string          `json:"warnings,omitempty"`
}

type FinalFeatureReport struct {
	Danceability      float64 `json:"danceability"`
	Energy            float64 `json:"energy"`
	Valence           float64 `json:"valence"`
	Loudness          float64 `json:"loudness,omitempty"`
	SpectralCentroid  float64 `json:"spectralCentroid,omitempty"`
	ZeroCrossingRate  float64 `json:"zeroCrossingRate,omitempty"`
	RMS               float64 `json:"rms,omitempty"`
	SpectralFlatness  float64 `json:"spectralFlatness,omitempty"`
	SpectralRolloff85 float64 `json:"spectralRolloff85,omitempty"`
	SpectralFlux      float64 `json:"spectralFlux,omitempty"`
	OnsetRate         float64 `json:"onsetRate,omitempty"`
	DynamicRange      float64 `json:"dynamicRange,omitempty"`
	LowBandRatio      float64 `json:"lowBandRatio,omitempty"`
	MidBandRatio      float64 `json:"midBandRatio,omitempty"`
	HighBandRatio     float64 `json:"highBandRatio,omitempty"`
	Happy             float64 `json:"happy"`
	Sad               float64 `json:"sad"`
	Relaxed           float64 `json:"relaxed"`
	Party             float64 `json:"party"`
	Aggressive        float64 `json:"aggressive"`
	Acoustic          float64 `json:"acoustic"`
	Electronic        float64 `json:"electronic"`
	Instrumental      float64 `json:"instrumental"`
	Vocal             float64 `json:"vocal"`
	Melodic           float64 `json:"melodic"`
	Soft              float64 `json:"soft"`
	Heavy             float64 `json:"heavy"`
	Dream             float64 `json:"dream"`
	Emotional         float64 `json:"emotional"`
	Brightness        float64 `json:"brightness"`
	Tonality          float64 `json:"tonality"`
	Approachability   float64 `json:"approachability"`
	Engagement        float64 `json:"engagement"`
}

type EssentiaProbeReport struct {
	Mel       MelProbeReport    `json:"mel"`
	BaseModel BaseModelReport   `json:"baseModel"`
	Heads     []HeadProbeReport `json:"heads"`
	Genre     GenreProbeReport  `json:"genre"`
	Embedding EmbeddingReport   `json:"embedding"`
	MelMode   string            `json:"melMode"`
}

type FullAudioProbe struct {
	Audio    AudioDecodeProbe    `json:"audio"`
	Tempo    TempoProbe          `json:"tempo"`
	Essentia EssentiaProbeReport `json:"essentia"`
	Features FinalFeatureReport  `json:"features"`
	Timing   ProbeTiming         `json:"timing,omitempty"`
	Warnings []string            `json:"warnings,omitempty"`
}

type BatchProbeSummary struct {
	FeatureStats     map[string]Stat `json:"featureStats"`
	HeadWarnings     map[string]int  `json:"headWarnings"`
	TracksWithErrors []string        `json:"tracksWithErrors,omitempty"`
}

// Compatibility shims for older app/test code. They are thin wrappers around the newer probe APIs.
type ProbeStat struct {
	Mean    float64 `json:"mean"`
	Median  float64 `json:"median"`
	Trimmed float64 `json:"trimmed"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Std     float64 `json:"std"`
	Support float64 `json:"support"`
}

type ProbeModel struct {
	Name        string   `json:"name"`
	ModelPath   string   `json:"modelPath"`
	MetaPath    string   `json:"metaPath"`
	Present     bool     `json:"present"`
	Loaded      bool     `json:"loaded"`
	Message     string   `json:"message"`
	InputName   string   `json:"inputName"`
	OutputName  string   `json:"outputName"`
	InputShape  []string `json:"inputShape"`
	OutputShape []string `json:"outputShape"`
}

type EssentiaProbe struct {
	RuntimePath string       `json:"runtimePath"`
	ModelsDir   string       `json:"modelsDir"`
	Ready       bool         `json:"ready"`
	Base        ProbeModel   `json:"base"`
	Genre       ProbeModel   `json:"genre"`
	Heads       []ProbeModel `json:"heads"`
	Message     string       `json:"message"`
}

func ProbeEssentia(runtimePath, modelsDir string) (EssentiaProbe, error) {
	engine, err := NewEssentiaEngine(runtimePath, modelsDir)
	if err != nil {
		return EssentiaProbe{RuntimePath: runtimePath, ModelsDir: modelsDir, Message: err.Error()}, err
	}
	defer engine.Close()
	base := ProbeModel{Name: "base", ModelPath: modelsDir + "/discogs-effnet-bs64-1.onnx", MetaPath: modelsDir + "/discogs-effnet-bs64-1.json", Present: true, Loaded: true, InputName: engine.baseInput, OutputName: engine.baseOutput}
	genre := ProbeModel{Name: "genre", ModelPath: modelsDir + "/genre_discogs400-discogs-effnet-1.onnx", MetaPath: modelsDir + "/genre_discogs400-discogs-effnet-1.json", Present: engine.genreSession != nil, Loaded: engine.genreSession != nil, InputName: engine.genreInput, OutputName: engine.genreOutput}
	heads := make([]ProbeModel, 0, len(essentiaHeadNames))
	for _, name := range essentiaHeadNames {
		head := engine.headMap[name]
		pm := ProbeModel{Name: name, ModelPath: modelsDir + "/" + name + ".onnx", MetaPath: modelsDir + "/" + name + ".json", Present: head != nil, Loaded: head != nil}
		if head != nil {
			pm.InputName = head.inputName
			pm.OutputName = head.outputName
		}
		heads = append(heads, pm)
	}
	return EssentiaProbe{RuntimePath: runtimePath, ModelsDir: modelsDir, Ready: engine.Ready(), Base: base, Genre: genre, Heads: heads, Message: fmt.Sprintf("base=%t legacyGenre=%t heads=%d", base.Present, genre.Present, len(heads))}, nil
}

type ProbeHeadReport struct{}

type ProbeReport struct{}

func WriteProbeReport(path string, report any) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func ProbeEssentiaDetailed(ctx context.Context, runtimePath, modelsDir, audioPath, melMode string) (any, error) {
	engine, err := NewEssentiaEngine(runtimePath, modelsDir)
	if err != nil {
		return nil, err
	}
	defer engine.Close()
	probe, err := engine.ProbeAudioFileFull(ctx, audioPath, ProbeOptions{MelMode: melMode, IncludePatchRows: true, MaxPatchRows: 8, IncludeGenrePatchDebug: true, IncludeTempoDebug: true})
	if err != nil {
		return nil, err
	}
	return probe, nil
}

func (e *EssentiaEngine) ProbeAudioFileFull(ctx context.Context, path string, opts ProbeOptions) (FullAudioProbe, error) {
	if !e.Ready() {
		return FullAudioProbe{}, errors.New("essentia engine not ready")
	}
	melMode := strings.TrimSpace(opts.MelMode)
	if melMode == "" {
		melMode = string(analysis.DefaultMelMode())
	}
	featureOpts := analysis.AudioFeatureOptions{MaxAnalysisSec: opts.AnalysisMaxSec, PreferCenterWindow: opts.PreferCenterWindow, ForProbeShort: opts.ShortAnalysis}
	startDecode := time.Now()
	features, durMs, err := analysis.ExtractWithOptions(ctx, path, featureOpts)
	decodeMS := time.Since(startDecode).Milliseconds()
	if err != nil {
		return FullAudioProbe{}, err
	}
	decoded := audioProbeFromFeatures(features, durMs)
	mel, validPatches, inputShape, patchStats, err := prepareProbeMel(path, opts)
	if err != nil {
		return FullAudioProbe{}, err
	}
	if len(mel) == 0 {
		return FullAudioProbe{}, errors.New("empty mel")
	}
	inputMel, validPatchesResolved, shape, err := prepareBaseMelInput(e.baseExpectedDims, mel, validPatches)
	if err != nil {
		return FullAudioProbe{}, fmt.Errorf("prepare base mel input failed: %w (melLen=%d patches=%d inputShape=%v melMode=%q)", err, len(mel), validPatches, inputShape, melMode)
	}
	_ = inputMel
	_ = shape
	probe := FullAudioProbe{
		Audio:  decoded,
		Timing: ProbeTiming{DecodeFullMS: decodeMS, DecodeShortMS: decodeMS},
		Tempo: TempoProbe{
			BPM:          features.Tempo,
			BPMPerceived: features.BPMPerceived,
			Confidence:   features.TempoConfidence,
			Stability:    features.TempoStability,
			Source:       features.TempoSource,
			Patches:      validPatchesResolved,
		},
		Essentia: EssentiaProbeReport{MelMode: melMode, Mel: MelProbeReport{InputShape: inputShape, ValidPatches: validPatchesResolved, PatchStats: patchStats}, BaseModel: BaseModelReport{ModelName: "discogs-effnet-bs64-1"}},
		Features: FinalFeatureReport{
			Danceability: features.Danceability,
			Energy:       features.Energy,
			Valence:      features.Valence,
			Acoustic:     features.Acousticness,
			Instrumental: features.Instrumentalness,
			Brightness:   clamp01(features.SpectralCentroid / 8000),
			Tonality:     0,
		},
	}
	if opts.IncludeTempoDebug && probe.Tempo.Patches == 0 {
		probe.Warnings = append(probe.Warnings, "tempo_missing")
	}
	essStart := time.Now()
	result, analyzeErr := e.analyzeProbe(ctx, mel, validPatchesResolved, opts)
	probe.Timing.EssentiaMS = time.Since(essStart).Milliseconds()
	if analyzeErr != nil {
		return FullAudioProbe{}, analyzeErr
	}
	probe.Essentia = result
	probe.Features = extractFinalFeatures(result, features)
	probe.Tempo.Patches = validPatchesResolved
	probe.Timing.Audio2MS = probe.Timing.DecodeShortMS
	probe.Timing.TempoMS = 0
	probe.Timing.TotalMS = probe.Timing.DecodeShortMS + probe.Timing.EssentiaMS + probe.Timing.TempoMS
	return probe, nil
}

func (e *EssentiaEngine) analyzeProbe(ctx context.Context, mel []float32, patches int, opts ProbeOptions) (EssentiaProbeReport, error) {
	inputMel, validPatches, shape, err := prepareBaseMelInput(e.baseExpectedDims, mel, patches)
	if err != nil {
		return EssentiaProbeReport{}, err
	}
	input, err := ort.NewTensorValue(e.rt, inputMel, shape)
	if err != nil {
		return EssentiaProbeReport{}, err
	}
	defer input.Close()
	outputs, err := e.base.Run(ctx, map[string]*ort.Value{e.baseInput: input})
	if err != nil {
		return EssentiaProbeReport{}, err
	}
	defer closeValues(outputs)
	baseOutputs := make([]OutputProbe, 0, len(outputs))
	for name, out := range outputs {
		data, shapeOut, derr := ort.GetTensorData[float32](out)
		if derr != nil || len(data) == 0 {
			continue
		}
		minV, maxV, meanV, stdV := summarizeFloats(data)
		baseOutputs = append(baseOutputs, OutputProbe{Name: name, Shape: append([]int64{}, shapeOut...), Min: minV, Max: maxV, Mean: meanV, Std: stdV})
	}
	sort.Slice(baseOutputs, func(i, j int) bool { return baseOutputs[i].Name < baseOutputs[j].Name })
	embVal := outputs[e.baseEmbeddingOutput]
	if embVal == nil {
		return EssentiaProbeReport{}, errors.New("essentia embedding output missing")
	}
	embData, embShape, err := ort.GetTensorData[float32](embVal)
	if err != nil {
		return EssentiaProbeReport{}, err
	}
	embAvg, err := averageRows(embData, embShape, validPatches)
	if err != nil {
		return EssentiaProbeReport{}, err
	}
	patchEmbeddings, err := slicePatchEmbeddings(embData, embShape, validPatches)
	if err != nil {
		return EssentiaProbeReport{}, err
	}
	res := EssentiaProbeReport{
		MelMode:   opts.MelMode,
		Mel:       MelProbeReport{InputShape: append([]int64{}, shape...), ValidPatches: validPatches},
		BaseModel: BaseModelReport{ModelName: "discogs-effnet-bs64-1", Outputs: baseOutputs},
		Embedding: EmbeddingReport{},
	}
	res.Embedding = embeddingReport(embAvg)
	genreData, genreShape, err := e.predictGenreFromPatchEmbeddings(ctx, patchEmbeddings, validPatches)
	if err != nil {
		return EssentiaProbeReport{}, err
	}
	genreAvg, err := averageRows(genreData, genreShape, validPatches)
	if err != nil {
		return EssentiaProbeReport{}, err
	}
	res.Genre = buildGenreProbe(genreAvg, genreData, genreShape, e.genreClasses, validPatches, opts)
	res.Heads = make([]HeadProbeReport, 0, len(essentiaHeadNames))
	for _, name := range essentiaHeadNames {
		head := e.headMap[name]
		if head == nil {
			continue
		}
		probs, err := e.runHead(ctx, head, patchEmbeddings, validPatches)
		if err != nil || len(probs) == 0 {
			continue
		}
		avg := averageHeadPredictions(probs, validPatches)
		res.Heads = append(res.Heads, buildHeadProbe(name, head, avg, probs, opts))
	}
	sort.Slice(res.Heads, func(i, j int) bool { return res.Heads[i].Name < res.Heads[j].Name })
	return res, nil
}

func audioProbeFromFeatures(f analysis.Features, durMs int) AudioDecodeProbe {
	_ = f
	return AudioDecodeProbe{SampleRate: 16000, Channels: 1, DurationSec: float64(durMs) / 1000.0, Samples: int(math.Round(float64(durMs) * 16))}
}

func prepareProbeMel(path string, opts ProbeOptions) ([]float32, int, []int64, []PatchStat, error) {
	frames, err := analysis.ExtractMelFramesMode(path, opts.MelMode)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	frameCount := len(frames) / analysis.EssentiaMelBands
	if frameCount < analysis.EssentiaPatchFrames {
		return nil, 0, nil, nil, errors.New("too few mel frames")
	}
	patchHop := 62
	patches := 1 + (frameCount-analysis.EssentiaPatchFrames)/patchHop
	if patches <= 0 {
		return nil, 0, nil, nil, errors.New("too few mel frames")
	}
	mel := make([]float32, 0, patches*analysis.EssentiaMelBands*analysis.EssentiaPatchFrames)
	patchStats := []PatchStat{}
	if opts.IncludePatchRows && opts.MaxPatchRows > 0 {
		maxRows := patches
		if maxRows > opts.MaxPatchRows {
			maxRows = opts.MaxPatchRows
		}
		for i := 0; i < maxRows; i++ {
			startFrame := i * patchHop
			endFrame := startFrame + analysis.EssentiaPatchFrames
			if endFrame > frameCount {
				break
			}
			chunk := frames[startFrame*analysis.EssentiaMelBands : endFrame*analysis.EssentiaMelBands]
			minV, maxV, meanV, rmsV := summarizeFloats(chunk)
			patchStats = append(patchStats, PatchStat{Index: i, Min: minV, Max: maxV, Mean: meanV, RMS: rmsV})
		}
	}
	for p := 0; p < patches; p++ {
		startFrame := p * patchHop
		endFrame := startFrame + analysis.EssentiaPatchFrames
		if endFrame > frameCount {
			break
		}
		chunk := frames[startFrame*analysis.EssentiaMelBands : endFrame*analysis.EssentiaMelBands]
		mel = append(mel, chunk...)
	}
	shape := []int64{int64(maxInt(1, patches)), int64(analysis.EssentiaPatchFrames), int64(analysis.EssentiaMelBands)}
	return mel, patches, shape, patchStats, nil
}

func buildHeadProbe(name string, head *essentiaHead, avg []float32, probs []float32, opts ProbeOptions) HeadProbeReport {
	positiveLabel := positiveHeadClass[name]
	positiveIdx := findClassIndex(head.classes, positiveLabel)
	if positiveIdx < 0 {
		positiveIdx = 0
	}
	rows, cols := inferRowsCols(len(probs), len(head.classes))
	series := classSeries(probs, rows, cols, positiveIdx)
	if len(series) == 0 {
		series = []float64{float64(positiveClassProb(avg, head.classes, positiveLabel))}
	}
	statsByClass := map[string]Stat{}
	if rows > 0 && cols > 0 {
		limit := cols
		if len(head.classes) > 0 && len(head.classes) < limit {
			limit = len(head.classes)
		}
		for i := 0; i < limit; i++ {
			cls := cleanName(safeClass(head.classes, i))
			if cls == "" {
				continue
			}
			statsByClass[cls] = stat(classSeries(probs, rows, cols, i))
		}
	}
	pos := stat(series)
	warnings := headWarningsForHead(name, pos, isRegressionHead(name))
	shape := []int64{int64(rows), int64(cols)}
	if rows <= 0 || cols <= 0 {
		shape = []int64{int64(len(series))}
	}
	firstRows := [][]float64{}
	if opts.IncludePatchRows && rows > 0 && cols > 0 {
		maxRows := rows
		if opts.MaxPatchRows > 0 && maxRows > opts.MaxPatchRows {
			maxRows = opts.MaxPatchRows
		}
		for r := 0; r < maxRows; r++ {
			base := r * cols
			if base+cols > len(probs) {
				break
			}
			row := make([]float64, 0, cols)
			for c := 0; c < cols; c++ {
				row = append(row, float64(probs[base+c]))
			}
			firstRows = append(firstRows, row)
		}
	}
	chosenValue := pos.Mean
	chosenMode := "mean"
	if isRegressionHead(name) {
		diagnostics := aggregateRegressionHead(probs, rows)
		chosenValue = diagnostics.Value
		chosenMode = "robust_trimmed_mean"
		if !diagnostics.Reliable {
			chosenMode = "neutral_fallback"
		}
	}
	return HeadProbeReport{Name: name, InputName: head.inputName, OutputName: head.outputName, Classes: append([]string{}, head.classes...), Shape: shape, PositiveLabel: positiveLabel, PositiveIndex: positiveIdx, StatsByClass: statsByClass, PositiveStats: pos, Aggregation: AggregationReport{ChosenValue: chosenValue, ChosenMode: chosenMode, Mean: pos.Mean, Median: pos.Median, Trimmed10: pos.Trimmed10, Support30: pos.Support30, Support50: pos.Support50, Support70: pos.Support70, Binaryness: pos.Binaryness}, FirstRows: firstRows, Warnings: warnings}
}

func positiveStats(avg []float32, idx int) Stat {
	if idx < 0 || idx >= len(avg) {
		return Stat{}
	}
	return stat([]float64{float64(avg[idx])})
}

var positiveHeadClass = map[string]string{
	"danceability-discogs-effnet-1":       "danceable",
	"mood_happy-discogs-effnet-1":         "happy",
	"mood_sad-discogs-effnet-1":           "sad",
	"mood_relaxed-discogs-effnet-1":       "relaxed",
	"mood_party-discogs-effnet-1":         "party",
	"mood_aggressive-discogs-effnet-1":    "aggressive",
	"mood_acoustic-discogs-effnet-1":      "acoustic",
	"mood_electronic-discogs-effnet-1":    "electronic",
	"voice_instrumental-discogs-effnet-1": "instrumental",
	"timbre-discogs-effnet-1":             "bright",
	"tonal_atonal-discogs-effnet-1":       "tonal",
}

func classIndexByName(classes []string, wanted string) int {
	wanted = strings.ToLower(strings.TrimSpace(wanted))
	for i, c := range classes {
		if strings.ToLower(strings.TrimSpace(c)) == wanted {
			return i
		}
	}
	return -1
}

func inferRowsCols(valuesLen int, classesLen int) (int, int) {
	if classesLen > 0 && valuesLen%classesLen == 0 {
		return valuesLen / classesLen, classesLen
	}
	if valuesLen > 0 {
		return valuesLen, 1
	}
	return 0, 0
}

func classSeries(values []float32, rows, cols, classIndex int) []float64 {
	if rows <= 0 || cols <= 0 || classIndex < 0 || classIndex >= cols {
		return nil
	}
	out := make([]float64, 0, rows)
	for r := 0; r < rows; r++ {
		i := r*cols + classIndex
		if i < 0 || i >= len(values) {
			break
		}
		out = append(out, float64(values[i]))
	}
	return out
}

func isRegressionHead(name string) bool {
	return strings.Contains(name, "regression")
}

func headWarningsForHead(name string, st Stat, regression bool) []string {
	var out []string
	if st.P95-st.P05 < 0.03 {
		out = append(out, "no_spread")
	}
	if regression {
		if st.Max > 1.5 || st.Min < -0.5 {
			out = append(out, "regression_out_of_range")
		}
		return out
	}
	if st.NearZeroRatio > 0.85 {
		out = append(out, "near_zero_saturated")
	}
	if st.NearOneRatio > 0.85 {
		out = append(out, "near_one_saturated")
	}
	if st.Binaryness > 0.75 {
		out = append(out, "binary_like")
	}
	_ = name
	return out
}

func stat(xs []float64) Stat {
	if len(xs) == 0 {
		return Stat{}
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
	return Stat{Count: len(clean), Mean: mean, Median: percentileSorted(clean, 0.50), Trimmed10: trimmedMeanSorted(clean, 0.10), Min: clean[0], Max: clean[len(clean)-1], Std: std, P05: percentileSorted(clean, 0.05), P25: percentileSorted(clean, 0.25), P75: percentileSorted(clean, 0.75), P95: percentileSorted(clean, 0.95), Support30: supportRatio(clean, 0.30), Support50: supportRatio(clean, 0.50), Support70: supportRatio(clean, 0.70), NearZeroRatio: nearZeroRatio(clean), NearOneRatio: nearOneRatio(clean), Binaryness: binaryness(clean)}
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
func mapZeros(xs []float64) []float64 { return xs }

func summarizeFloats(xs []float32) (float64, float64, float64, float64) {
	if len(xs) == 0 {
		return 0, 0, 0, 0
	}
	minV, maxV := float64(xs[0]), float64(xs[0])
	sum, sumSq := 0.0, 0.0
	for _, x := range xs {
		v := float64(x)
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
		sum += v
		sumSq += v * v
	}
	mean := sum / float64(len(xs))
	std := math.Sqrt(sumSq/float64(len(xs)) - mean*mean)
	return minV, maxV, mean, std
}

func embeddingReport(xs []float32) EmbeddingReport {
	if len(xs) == 0 {
		return EmbeddingReport{}
	}
	minV, maxV, meanV, rmsV := summarizeFloat32(xs)
	vec := make([]float64, len(xs))
	for i, x := range xs {
		vec[i] = float64(x)
	}
	return EmbeddingReport{Dim: len(xs), Min: float64(minV), Max: float64(maxV), Mean: float64(meanV), RMS: float64(rmsV), Norm: l2Norm(xs), Vector: vec}
}

func buildGenreProbe(avg, raw []float32, shape []int64, classes []string, validPatches int, opts ProbeOptions) GenreProbeReport {
	top := topKGenres(avg, classes, 15)
	groups := topGenreGroups(avg, classes, 10)
	primary, detail, score, margin := choosePrimaryGenre(groups, top)
	warnings := []string{}
	if score < 0.10 {
		warnings = append(warnings, "genre_weak")
	}
	if margin < 0.05 {
		warnings = append(warnings, "genre_low_margin")
	}
	out := GenreProbeReport{ModelName: "genre_discogs400-discogs-effnet-1", OutputName: "sigmoid", Shape: append([]int64{}, shape...), Primary: primary, Label: primary, Detail: detail, Score: float64(score), Margin: float64(margin), Warnings: warnings}
	for _, c := range top {
		out.TopClasses = append(out.TopClasses, GenreClassScore{Index: c.Idx, Label: c.Label, Score: float64(c.Score)})
	}
	for _, g := range groups {
		out.Groups = append(out.Groups, GenreGroupScore{Label: g.Label, Score: float64(g.Score), Support: g.Support, SumScore: float64(g.SumScore), BestSubLabel: g.BestSubLabel, BestSubScore: float64(g.BestSubScore), SecondSubScore: float64(g.SecondSubScore)})
	}
	if opts.IncludeGenrePatchDebug && len(classes) > 0 {
		rows := minInt(validPatches, len(raw)/len(classes))
		rows = minInt(rows, 8)
		for patch := 0; patch < rows; patch++ {
			start := patch * len(classes)
			end := start + len(classes)
			if end > len(raw) {
				break
			}
			topPatch := topKGenres(raw[start:end], classes, 5)
			row := make([]GenreClassScore, 0, len(topPatch))
			for _, candidate := range topPatch {
				row = append(row, GenreClassScore{Index: candidate.Idx, Label: candidate.Label, Score: float64(candidate.Score)})
			}
			out.PatchTop = append(out.PatchTop, GenrePatchTop{Patch: patch, Top: row})
		}
	}
	return out
}

func extractFinalFeatures(ess EssentiaProbeReport, f analysis.Features) FinalFeatureReport {
	dance := essHeadsValue(ess.Heads, "danceability-discogs-effnet-1")
	happy := essHeadsValue(ess.Heads, "mood_happy-discogs-effnet-1")
	sad := essHeadsValue(ess.Heads, "mood_sad-discogs-effnet-1")
	relaxed := essHeadsValue(ess.Heads, "mood_relaxed-discogs-effnet-1")
	party := essHeadsValue(ess.Heads, "mood_party-discogs-effnet-1")
	aggressive := essHeadsValue(ess.Heads, "mood_aggressive-discogs-effnet-1")
	acoustic := essHeadsValue(ess.Heads, "mood_acoustic-discogs-effnet-1")
	electronic := essHeadsValue(ess.Heads, "mood_electronic-discogs-effnet-1")
	instrumental := probeBlendMetric(f.Instrumentalness, essHeadsValue(ess.Heads, "voice_instrumental-discogs-effnet-1"), 0.7)
	brightness := essHeadsValue(ess.Heads, "timbre-discogs-effnet-1")
	tonality := essHeadsValue(ess.Heads, "tonal_atonal-discogs-effnet-1")
	approachability := essHeadsValue(ess.Heads, "approachability_regression-discogs-effnet-1")
	engagement := essHeadsValue(ess.Heads, "engagement_regression-discogs-effnet-1")

	melodic := essHeadWeightedEvidence(ess.Heads, "mtg_jamendo_moodtheme-discogs-effnet-1", jamendoMelodicWeights)
	soft := essHeadWeightedEvidence(ess.Heads, "mtg_jamendo_moodtheme-discogs-effnet-1", jamendoSoftnessWeights)
	heavy := essHeadWeightedEvidence(ess.Heads, "mtg_jamendo_moodtheme-discogs-effnet-1", jamendoHeavinessWeights)
	dream := essHeadWeightedEvidence(ess.Heads, "mtg_jamendo_moodtheme-discogs-effnet-1", jamendoDreaminessWeights)
	emotional := essHeadWeightedEvidence(ess.Heads, "mtg_jamendo_moodtheme-discogs-effnet-1", jamendoEmotionalityWeights)
	melodic = clamp01(maxf(melodic, 0.15*tonality))
	soft = clamp01(maxf(soft, 0.20*relaxed))
	heavy = clamp01(maxf(heavy, 0.30*aggressive))

	mlValence := deriveValence(happy, sad, relaxed, party, aggressive)
	mlEnergy := deriveEnergy(dance, party, aggressive, engagement)

	return FinalFeatureReport{
		Danceability:      probeBlendMetric(f.Danceability, dance, 0.7),
		Energy:            probeBlendMetric(f.Energy, mlEnergy, 0.65),
		Valence:           probeBlendMetric(f.Valence, mlValence, 0.7),
		Loudness:          f.Loudness,
		SpectralCentroid:  f.SpectralCentroid,
		ZeroCrossingRate:  f.ZeroCrossingRate,
		RMS:               f.RMS,
		SpectralFlatness:  f.SpectralFlatness,
		SpectralRolloff85: f.SpectralRolloff85,
		SpectralFlux:      f.SpectralFlux,
		OnsetRate:         f.OnsetRate,
		DynamicRange:      f.DynamicRange,
		LowBandRatio:      f.LowBandRatio,
		MidBandRatio:      f.MidBandRatio,
		HighBandRatio:     f.HighBandRatio,
		Happy:             happy,
		Sad:               sad,
		Relaxed:           relaxed,
		Party:             party,
		Aggressive:        aggressive,
		Acoustic:          probeBlendMetric(f.Acousticness, acoustic, 0.7),
		Electronic:        electronic,
		Instrumental:      instrumental,
		Vocal:             clamp01(1 - instrumental),
		Melodic:           melodic,
		Soft:              soft,
		Heavy:             heavy,
		Dream:             dream,
		Emotional:         emotional,
		Brightness:        brightness,
		Tonality:          tonality,
		Approachability:   approachability,
		Engagement:        engagement,
	}
}

func essHeadsValue(heads []HeadProbeReport, name string) float64 {
	for _, h := range heads {
		if h.Name == name {
			return h.Aggregation.ChosenValue
		}
	}
	return 0
}

func essHeadWeightedEvidence(heads []HeadProbeReport, name string, weights map[string]float64) float64 {
	remaining := 1.0
	for _, h := range heads {
		if h.Name != name {
			continue
		}
		for className, weight := range weights {
			st, ok := h.StatsByClass[className]
			if !ok {
				continue
			}
			evidence := clamp01(st.Mean * clamp01(weight))
			remaining *= 1 - evidence
		}
		break
	}
	return clamp01(1 - remaining)
}

func probeBlendMetric(base, ml, mlWeight float64) float64 {
	if ml <= 0 {
		return base
	}
	return base*(1-mlWeight) + ml*mlWeight
}

func cleanName(s string) string { return strings.TrimSpace(s) }
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
