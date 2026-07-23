package onnx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ray-player1/internal/analysis"
	"ray-player1/internal/logx"

	ort "github.com/yalue/onnxruntime_go"
)

var tempoLog = logx.New("tempo")

type TempoResult struct {
	BPM             float64
	BPMPerceived    float64
	Confidence      float64
	Stability       float64
	BPMHalf         float64
	BPMDouble       float64
	Source          string
	ModelVersion    string
	LocalBPM        []float64
	LocalConfidence []float64
	AnalyzedAt      int64

	RawBPMMean float64
	RawBPMStd  float64

	Reliable bool
}

type tempoModelMeta struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Inference struct {
		SampleRate int    `json:"sample_rate"`
		Algorithm  string `json:"algorithm"`
	} `json:"inference"`
	Schema struct {
		Inputs  []essentiaIO `json:"inputs"`
		Outputs []essentiaIO `json:"outputs"`
	} `json:"schema"`
}

type TempoEngine struct {
	mu           sync.Mutex
	session      *ort.DynamicSession[float32, float32]
	inputName    string
	outputName   string
	modelName    string
	modelVersion string
	inputShape   []int64
	outputDims   []int64
}

func NewTempoEngine(runtimePath, modelsDir string) (*TempoEngine, error) {
	if strings.TrimSpace(modelsDir) == "" {
		return nil, errors.New("tempo models dir empty")
	}
	modelPath := filepath.Join(modelsDir, "deeptemp-k4-3.onnx")
	metaPath := filepath.Join(modelsDir, "deeptemp-k4-3.json")
	meta, err := readTempoMeta(metaPath)
	if err != nil {
		return nil, err
	}
	if err := AcquireEnvironmentWithPath(strings.TrimSpace(runtimePath)); err != nil {
		return nil, err
	}
	cleanupEnv := true
	defer func() {
		if cleanupEnv {
			_ = ReleaseEnvironment()
		}
	}()
	inputs, outputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, err
	}
	inputNames := []string{firstNameFromInfo(inputs)}
	outputNames := []string{pickSessionOutput(namesFromInfo(outputs), "output", "softmax", "predictions")}
	session, err := ort.NewDynamicSession[float32, float32](modelPath, inputNames, outputNames)
	if err != nil {
		return nil, err
	}
	modelName := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))
	engine := &TempoEngine{session: session, inputName: inputNames[0], outputName: outputNames[0], modelName: modelName, modelVersion: strings.TrimSpace(meta.Version), outputDims: dimsByName(outputs, outputNames[0])}
	cleanupEnv = false
	tempoLog.I("model loaded name=%s version=%s input=%s output=%s", engine.modelName, engine.modelVersion, engine.inputName, engine.outputName)
	return engine, nil
}

func (e *TempoEngine) Ready() bool {
	return e != nil && e.session != nil
}

func (e *TempoEngine) Close() error {
	if e == nil {
		return nil
	}
	if e.session != nil {
		_ = e.session.Destroy()
	}
	return ReleaseEnvironment()
}

func (e *TempoEngine) AnalyzePath(ctx context.Context, path string) (TempoResult, error) {
	if !e.Ready() {
		return TempoResult{}, errors.New("tempo engine not ready")
	}
	patches, err := analysis.ExtractTempoPatches(path)
	if err != nil {
		return TempoResult{}, err
	}
	preds := make([]tempoPrediction, 0, len(patches))
	localTempo := make([]float64, 0, len(patches))
	localProb := make([]float64, 0, len(patches))
	start := time.Now()
	for _, patch := range patches {
		select {
		case <-ctx.Done():
			return TempoResult{}, ctx.Err()
		default:
		}
		probs, runErr := e.runPatch(ctx, patch)
		if runErr != nil {
			return TempoResult{}, runErr
		}
		bpm, conf, _ := DecodeTempoOutput(probs)
		preds = append(preds, tempoPrediction{BPM: bpm, Confidence: conf})
		localTempo = append(localTempo, bpm)
		localProb = append(localProb, conf)
	}
	result := AggregateTempoMajorityVoting(preds)
	result.BPMPerceived = analysis.NormalizePerceivedBPM(result.BPM)
	result.Stability = analysis.TempoStability(localTempo, result.BPM)
	result.Confidence = clamp01(math.Max(result.Confidence, analysis.TempoConfidenceMedian(localProb)))
	result.BPMHalf = result.BPM * 0.5
	result.BPMDouble = result.BPM * 2
	result.LocalBPM = localTempo
	result.LocalConfidence = localProb
	result.Source = "tempocnn"
	if e.modelVersion != "" {
		result.ModelVersion = e.modelName + "@" + e.modelVersion
	} else {
		result.ModelVersion = e.modelName
	}
	result.AnalyzedAt = time.Now().Unix()

	if len(localTempo) > 0 {
		var sum float64
		for _, value := range localTempo {
			sum += value
		}
		result.RawBPMMean = sum / float64(len(localTempo))

		var variance float64
		for _, value := range localTempo {
			delta := value - result.RawBPMMean
			variance += delta * delta
		}
		result.RawBPMStd = math.Sqrt(variance / float64(len(localTempo)))
	}

	result.Reliable =
		len(patches) >= 4 &&
			result.BPM >= 45 &&
			result.BPM <= 240 &&
			result.Confidence >= 0.35 &&
			result.RawBPMStd >= 0.25

	if !result.Reliable {
		tempoLog.W(
			"tempo rejected bpm=%.2f conf=%.3f stability=%.3f patches=%d rawMean=%.2f rawStd=%.4f",
			result.BPM,
			result.Confidence,
			result.Stability,
			len(patches),
			result.RawBPMMean,
			result.RawBPMStd,
		)
	}

	tempoLog.I("analyzed path=%s bpm=%.2f perceived=%.2f conf=%.3f stability=%.3f patches=%d rawMean=%.2f rawStd=%.2f reliable=%t ms=%d", path, result.BPM, result.BPMPerceived, result.Confidence, result.Stability, len(patches), result.RawBPMMean, result.RawBPMStd, result.Reliable, time.Since(start).Milliseconds())
	return result, nil
}

func tempoInputShapeCandidates() [][]int64 {
	f := int64(analysis.TempoPatchFrames)
	m := int64(analysis.TempoMelBands)
	return [][]int64{{1, 1, f, m}, {1, f, m, 1}, {1, 1, m, f}, {1, m, f, 1}}
}

func tempoShapeNeedsTranspose(shape []int64) bool {
	if len(shape) != 4 {
		return false
	}
	f := int64(analysis.TempoPatchFrames)
	m := int64(analysis.TempoMelBands)
	return (shape[2] == m && shape[3] == f) || (shape[1] == m && shape[2] == f)
}

func transposeTempoPatch(in []float32) []float32 {
	frames := analysis.TempoPatchFrames
	mels := analysis.TempoMelBands
	out := make([]float32, len(in))
	for frame := 0; frame < frames; frame++ {
		for mel := 0; mel < mels; mel++ {
			out[mel*frames+frame] = in[frame*mels+mel]
		}
	}
	return out
}

func sameInt64Slice(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (e *TempoEngine) runPatch(ctx context.Context, patch []float32) ([]float32, error) {
	if len(patch) != analysis.TempoPatchFrames*analysis.TempoMelBands {
		return nil, fmt.Errorf("tempo patch len=%d expected=%d", len(patch), analysis.TempoPatchFrames*analysis.TempoMelBands)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	shapes := tempoInputShapeCandidates()
	if len(e.inputShape) == 4 {
		shapes = append([][]int64{append([]int64{}, e.inputShape...)}, shapes...)
	}

	var lastErr error
	for _, shape := range shapes {
		data := patch
		if tempoShapeNeedsTranspose(shape) {
			data = transposeTempoPatch(patch)
		}
		input, err := ort.NewTensor(ort.NewShape(shape...), data)
		if err != nil {
			lastErr = fmt.Errorf("shape=%v tensor: %w", shape, err)
			continue
		}
		outputShape := concreteTempoOutputShape(e.outputDims)
		output, err := ort.NewEmptyTensor[float32](ort.NewShape(outputShape...))
		if err != nil {
			_ = input.Destroy()
			lastErr = fmt.Errorf("shape=%v output tensor: %w", shape, err)
			continue
		}
		select {
		case <-ctx.Done():
			_ = output.Destroy()
			_ = input.Destroy()
			return nil, ctx.Err()
		default:
		}
		err = e.session.Run([]*ort.Tensor[float32]{input}, []*ort.Tensor[float32]{output})
		_ = input.Destroy()
		if err != nil {
			_ = output.Destroy()
			lastErr = fmt.Errorf("shape=%v run: %w", shape, err)
			continue
		}
		dataOut := append([]float32{}, output.GetData()...)
		_ = output.Destroy()
		if len(dataOut) == 0 {
			lastErr = fmt.Errorf("shape=%v empty output", shape)
			continue
		}
		if !sameInt64Slice(e.inputShape, shape) {
			e.inputShape = append([]int64{}, shape...)
			tempoLog.I("selected input shape=%v input=%s output=%s", shape, e.inputName, e.outputName)
		}
		return dataOut, nil
	}
	return nil, fmt.Errorf("tempo inference failed for all input shapes: %w", lastErr)
}

func concreteTempoOutputShape(shape []int64) []int64 {
	if len(shape) == 0 {
		return []int64{1, 256}
	}
	out := append([]int64{}, shape...)
	for i, dim := range out {
		if dim <= 0 {
			out[i] = 1
		}
	}
	return out
}

type tempoPrediction struct {
	BPM        float64
	Confidence float64
}

func DecodeTempoOutput(probs []float32) (float64, float64, int) {
	if len(probs) == 0 {
		return 0, 0, -1
	}
	bestIdx := 0
	best := probs[0]
	for i := 1; i < len(probs); i++ {
		if probs[i] > best {
			best = probs[i]
			bestIdx = i
		}
	}
	return float64(30 + bestIdx), clamp01(float64(best)), bestIdx
}

func AggregateTempoMajorityVoting(preds []tempoPrediction) TempoResult {
	if len(preds) == 0 {
		return TempoResult{}
	}
	buckets := map[int]float64{}
	bestConf := map[int]float64{}
	for _, pred := range preds {
		if pred.BPM <= 0 || pred.Confidence <= 0 {
			continue
		}
		bucket := int(math.Round(analysis.NormalizePerceivedBPM(pred.BPM)))
		buckets[bucket] += pred.Confidence
		if pred.Confidence > bestConf[bucket] {
			bestConf[bucket] = pred.Confidence
		}
	}
	bestBucket := 0
	bestScore := -1.0
	total := 0.0
	for bucket, score := range buckets {
		total += score
		if score > bestScore {
			bestScore = score
			bestBucket = bucket
		}
	}
	conf := 0.0
	if total > 0 {
		conf = bestScore / total
	}
	return TempoResult{BPM: float64(bestBucket), Confidence: clamp01(math.Max(conf, bestConf[bestBucket]))}
}

func readTempoMeta(path string) (tempoModelMeta, error) {
	var meta tempoModelMeta
	raw, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func firstNameFromInfo(info []ort.InputOutputInfo) string {
	if len(info) == 0 {
		return ""
	}
	return info[0].Name
}

func namesFromInfo(info []ort.InputOutputInfo) []string {
	out := make([]string, 0, len(info))
	for _, item := range info {
		out = append(out, item.Name)
	}
	return out
}

func dimsByName(info []ort.InputOutputInfo, name string) []int64 {
	for _, item := range info {
		if item.Name == name {
			return append([]int64{}, item.Dimensions...)
		}
	}
	return nil
}
