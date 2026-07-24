package onnx

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ray-player1/internal/analysis"
	"ray-player1/internal/logx"

	ort "ray-player1/internal/onnx/ortshim"
)

var essentiaLog = logx.New("essentia")

const essentiaEmbeddingSize = 1280

var essentiaHeadNames = []string{
	"danceability-discogs-effnet-1",
	"mood_happy-discogs-effnet-1",
	"mood_sad-discogs-effnet-1",
	"mood_relaxed-discogs-effnet-1",
	"mood_party-discogs-effnet-1",
	"mood_aggressive-discogs-effnet-1",
	"mood_acoustic-discogs-effnet-1",
	"mood_electronic-discogs-effnet-1",
	"voice_instrumental-discogs-effnet-1",
	"mtg_jamendo_moodtheme-discogs-effnet-1",
	"timbre-discogs-effnet-1",
	"tonal_atonal-discogs-effnet-1",
	"approachability_regression-discogs-effnet-1",
	"engagement_regression-discogs-effnet-1",
}

type EssentiaOutput struct {
	Embedding         []float32
	FeatureVector     []float32
	Backend           string
	ModelPath         string
	DiscogsPatchCount int
	Danceability      float64
	Valence           float64
	Acousticness      float64
	Instrumentalness  float64
	Electronic        float64
	Energy            float64
	MoodHappy         float64
	MoodSad           float64
	MoodRelaxed       float64
	MoodParty         float64
	MoodAggressive    float64
	TimbreBrightness  float64
	Tonality          float64
	Approachability   float64
	Engagement        float64
	Melodicness       float64
	Softness          float64
	Heaviness         float64
	Dreaminess        float64
	Emotionality      float64
	GenrePrimary      string
	GenreDetail       string
	GenreScore        float64
	GenreMargin       float64
	GenreTags         []GenreTag
	GenreLabel        string
	GenreReliable     bool
	GenreQuality      GenreOutputQuality
}

func (o EssentiaOutput) ValidateSemanticOutput() error {
	if o.Backend != "discogs-effnet-onnx" {
		return fmt.Errorf(
			"unexpected semantic analysis backend %q",
			o.Backend,
		)
	}
	if len(o.Embedding) != 1280 {
		return fmt.Errorf(
			"invalid semantic embedding size: got=%d want=1280 backend=%s model=%q patches=%d",
			len(o.Embedding),
			o.Backend,
			o.ModelPath,
			o.DiscogsPatchCount,
		)
	}
	if o.DiscogsPatchCount <= 0 {
		return fmt.Errorf(
			"Discogs analysis returned no patches",
		)
	}
	return nil
}

// GenreTag keeps a compact, structured summary of the top-level genre signal.
type GenreTag struct {
	Label   string  `json:"label"`
	Detail  string  `json:"detail,omitempty"`
	Score   float64 `json:"score"`
	Rank    int     `json:"rank"`
	Support int     `json:"support,omitempty"`
}

type genreCandidate struct {
	Idx   int
	Label string
	Score float32
}

type GenreGroupCandidate struct {
	Label        string
	Score        float32
	Support      int
	SumScore     float32
	BestSubLabel string
	BestSubScore float32
}

type GenrePrediction struct {
	Label  string
	Score  float64
	Margin float64
	Top    []genreCandidate
}

type essentiaHead struct {
	name       string
	inputName  string
	outputName string
	session    *ort.Session
	classes    []string
	outputs    []string
}

type essentiaIO struct {
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	Shape         []interface{} `json:"shape"`
	Op            string        `json:"op"`
	OutputPurpose string        `json:"output_purpose"`
}

type essentiaModelMeta struct {
	Classes []string `json:"classes"`
	Schema  struct {
		Inputs  []essentiaIO `json:"inputs"`
		Outputs []essentiaIO `json:"outputs"`
	} `json:"schema"`
}

type EssentiaEngine struct {
	mu                  sync.Mutex
	rt                  *ort.Runtime
	env                 *ort.Env
	base                *ort.Session
	baseInput           string
	baseOutput          string
	baseGenreOutput     string
	baseEmbeddingOutput string
	baseExpectedDims    []interface{}
	genreSession        *ort.Session
	genreInput          string
	genreOutput         string
	genreClasses        []string
	headMap             map[string]*essentiaHead
	discogs             *DynamicMultiOutputFloatModel

	baseModelPath  string
	genreModelPath string
}

func NewEssentiaEngine(runtimePath, modelsDir string) (*EssentiaEngine, error) {
	runtimePath = strings.TrimSpace(runtimePath)
	if runtimePath == "" {
		runtimePath = DiscoverRuntimePath()
	}
	if strings.TrimSpace(modelsDir) == "" {
		return nil, errors.New("essentia models dir empty")
	}
	essentiaLog.I("init runtime=%q models=%q", runtimePath, modelsDir)

	discogsPath := filepath.Join(modelsDir, "discogs-effnet-bsdynamic-1.onnx")
	essentiaLog.I("discogs model path=%q", discogsPath)

	if err := AcquireEnvironmentWithPath(runtimePath); err != nil {
		return nil, err
	}
	cleanupEnv := true
	defer func() {
		if cleanupEnv {
			_ = ReleaseEnvironment()
		}
	}()

	for _, model := range []struct {
		name string
		path string
	}{
		{"base", filepath.Join(modelsDir, "discogs-effnet-bs64-1.onnx")},
		{"genre-head", filepath.Join(modelsDir, "genre_discogs400-discogs-effnet-1.onnx")},
		{"genre-labels", filepath.Join(modelsDir, "genre_discogs400-discogs-effnet-1.json")},
	} {
		if err := logModelIdentity(model.name, model.path); err != nil {
			return nil, err
		}
	}

	rt, err := ort.NewRuntime(runtimePath, requiredONNXRuntimeAPIVersion)
	if err != nil {
		return nil, err
	}
	env, err := rt.NewEnv("ray-player-essentia", ort.LoggingLevelWarning)
	if err != nil {
		_ = rt.Close()
		return nil, err
	}

	basePath := filepath.Join(modelsDir, "discogs-effnet-bs64-1.onnx")
	base, err := rt.NewSession(env, basePath, &ort.SessionOptions{IntraOpNumThreads: 2})
	if err != nil {
		env.Close()
		_ = rt.Close()
		return nil, err
	}

	baseInputs := base.InputNames()
	baseOutputs := base.OutputNames()
	engine := &EssentiaEngine{
		rt:                  rt,
		env:                 env,
		base:                base,
		baseInput:           pickSessionInput(baseInputs, "serving_default_melspectrogram:0", "serving_default_melspectrogram", "melspectrogram"),
		baseGenreOutput:     pickSessionOutput(baseOutputs, "PartitionedCall:0", "partitionedcall:0"),
		baseEmbeddingOutput: pickSessionOutput(baseOutputs, "PartitionedCall:1", "partitionedcall:1", "embedding"),
		baseExpectedDims:    firstShape(metaInputs(metaFromPath(filepath.Join(modelsDir, "discogs-effnet-bs64-1.json")))),
		headMap:             map[string]*essentiaHead{},
		baseModelPath:       basePath,
		genreModelPath:      filepath.Join(modelsDir, "genre_discogs400-discogs-effnet-1.onnx"),
	}
	engine.baseOutput = engine.baseEmbeddingOutput
	essentiaLog.I("base loaded input=%q genreOutput=%q embeddingOutput=%q inputs=%v outputs=%v", engine.baseInput, engine.baseGenreOutput, engine.baseEmbeddingOutput, baseInputs, baseOutputs)

	baseMetaPath := filepath.Join(modelsDir, "discogs-effnet-bs64-1.json")
	baseMeta, metaErr := readEssentiaMeta(baseMetaPath)
	if metaErr != nil {
		essentiaLog.I("base meta read failed path=%q err=%v", baseMetaPath, metaErr)
	} else {
		engine.genreClasses = append([]string{}, baseMeta.Classes...)
		essentiaLog.I("base classes loaded model=%q classes=%d c70=%q c187=%q c314=%q c335=%q", "discogs-effnet-bs64-1", len(engine.genreClasses), safeClass(engine.genreClasses, 70), safeClass(engine.genreClasses, 187), safeClass(engine.genreClasses, 314), safeClass(engine.genreClasses, 335))
		if len(engine.genreClasses) != 400 {
			essentiaLog.I("warning unexpected base classes len=%d", len(engine.genreClasses))
		}
	}

	engine.loadGenre(modelsDir)

	for _, name := range essentiaHeadNames {
		if head, err := engine.loadHead(modelsDir, name); err == nil {
			engine.headMap[name] = head
		}
	}
	essentiaLog.I("models summary base=%q genre=%q heads=%d enabledHeads=%v", "discogs-effnet-bs64-1", "genre_discogs400-discogs-effnet-1", len(engine.headMap), engine.enabledHeadNames())

	discogsPath = filepath.Join(modelsDir, "discogs-effnet-bsdynamic-1.onnx")
	if _, discogsErr := os.Stat(discogsPath); discogsErr == nil {
		discogsInfo, inspectErr := InspectModel(discogsPath)
		if inspectErr != nil {
			essentiaLog.I("discogs inspect failed path=%q err=%v", discogsPath, inspectErr)
		} else {
			essentiaLog.I("discogs contract path=%q %s", discogsPath, discogsInfo.String())
			ioNames, contractErr := ValidateDiscogsContract(discogsInfo)
			if contractErr != nil {
				essentiaLog.I("discogs contract validation failed: %v", contractErr)
			} else {
				discogsModel, dErr := NewDynamicMultiOutputFloatModelWithPath(
					runtimePath,
					"discogs-effnet",
					discogsPath,
					ioNames.Input,
					[]string{
						ioNames.Predictions,
						ioNames.Embedding,
					},
				)
				if dErr != nil {
					essentiaLog.I("discogs dynamic model load failed: %v", dErr)
				} else {
					engine.discogs = discogsModel
					essentiaLog.I(
						"discogs dynamic model loaded path=%q input=%q predictions=%q embedding=%q",
						discogsPath,
						ioNames.Input,
						ioNames.Predictions,
						ioNames.Embedding,
					)
				}
			}
		}
	} else {
		essentiaLog.I("discogs dynamic model not found path=%q err=%v", discogsPath, discogsErr)
	}

	for _, model := range []struct {
		name string
		path string
	}{
		{
			name: "embedding",
			path: filepath.Join(modelsDir, "discogs-effnet-bs64-1.onnx"),
		},
		{
			name: "genre",
			path: filepath.Join(modelsDir, "genre_discogs400-discogs-effnet-1.onnx"),
		},
		{
			name: "danceability",
			path: filepath.Join(modelsDir, "danceability-discogs-effnet-1.onnx"),
		},
		{
			name: "valence",
			path: filepath.Join(modelsDir, "mood_happy-discogs-effnet-1.onnx"),
		},
	} {
		if _, err := os.Stat(model.path); err != nil {
			continue
		}
		info, err := InspectModel(model.path)
		if err != nil {
			essentiaLog.I("inspect %s model failed: %v", model.name, err)
			continue
		}
		essentiaLog.I(
			"onnx model=%s path=%q %s",
			model.name,
			model.path,
			info.String(),
		)
	}

	if err := engine.selfTest(); err != nil {
		engine.Close()
		return nil, err
	}

	if engine.discogs != nil {
		if err := engine.validateDiscogsModel(); err != nil {
			essentiaLog.I("discogs validation failed: %v (falling back to base model)", err)
			_ = engine.discogs.Close()
			engine.discogs = nil
		}
	}

	cleanupEnv = false
	return engine, nil
}

func (e *EssentiaEngine) selfTest() error {
	input := make([]float32, 128*96)
	for index := range input {
		input[index] = float32(
			math.Sin(
				2 *
					math.Pi *
					440 *
					float64(index) /
					16000,
			),
		)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.base == nil {
		return errors.New("base model not loaded for self-test")
	}
	inputMel, validPatches, shape, err := prepareBaseMelInput(e.baseExpectedDims, input, 1)
	if err != nil {
		return fmt.Errorf("self-test prepare mel: %w", err)
	}
	tensor, err := ort.NewTensorValue(e.rt, inputMel, shape)
	if err != nil {
		return fmt.Errorf("self-test create tensor: %w", err)
	}
	defer tensor.Close()
	outputs, err := e.base.Run(context.Background(), map[string]*ort.Value{e.baseInput: tensor})
	if err != nil {
		return fmt.Errorf("embedding self-test: %w", err)
	}
	defer closeValues(outputs)
	embVal := outputs[e.baseEmbeddingOutput]
	if embVal == nil {
		return errors.New("embedding self-test: output missing")
	}
	embData, embShape, err := ort.GetTensorData[float32](embVal)
	if err != nil {
		return fmt.Errorf("embedding self-test: %w", err)
	}
	_ = validPatches
	if len(embShape) < 2 {
		return fmt.Errorf("embedding self-test: unexpected shape %v", embShape)
	}
	dim := int(embShape[1])
	if dim != essentiaEmbeddingSize {
		return fmt.Errorf(
			"embedding self-test dim=%d want=%d",
			dim,
			essentiaEmbeddingSize,
		)
	}
	if len(embData) < dim {
		return fmt.Errorf(
			"embedding self-test data len=%d want>=%d",
			len(embData),
			dim,
		)
	}

	essentiaLog.I(
		"onnx self-test ok embedding=%d",
		dim,
	)
	return nil
}

func (e *EssentiaEngine) validateDiscogsModel() error {
	if e.discogs == nil {
		return errors.New("discogs model not loaded")
	}

	const (
		testPatchFrames = 128
		testMelBands    = 96
		testPatchCount  = 2
		classCount      = 400
		embeddingSize   = 1280
	)

	input := make([]float32, testPatchCount*testPatchFrames*testMelBands)
	for index := range input {
		input[index] = float32(index%97) / 97
	}

	outputs, err := e.discogs.Run(
		input,
		[]int64{int64(testPatchCount), int64(testPatchFrames), int64(testMelBands)},
		[][]int64{
			{int64(testPatchCount), int64(classCount)},
			{int64(testPatchCount), int64(embeddingSize)},
		},
	)
	if err != nil {
		return fmt.Errorf("discogs self-test: %w", err)
	}
	if len(outputs) != 2 {
		return fmt.Errorf("discogs self-test output count=%d want=2", len(outputs))
	}
	if len(outputs[0]) != testPatchCount*classCount {
		return fmt.Errorf("discogs self-test predictions=%d want=%d", len(outputs[0]), testPatchCount*classCount)
	}
	if len(outputs[1]) != testPatchCount*embeddingSize {
		return fmt.Errorf("discogs self-test embeddings=%d want=%d", len(outputs[1]), testPatchCount*embeddingSize)
	}

	essentiaLog.I("discogs ONNX self-test ok patches=%d predictions=%d embedding=%d", testPatchCount, classCount, embeddingSize)
	return nil
}

func (e *EssentiaEngine) loadHead(modelsDir, name string) (*essentiaHead, error) {
	modelPath := filepath.Join(modelsDir, name+".onnx")
	jsonPath := filepath.Join(modelsDir, name+".json")
	meta, metaErr := readEssentiaMeta(jsonPath)
	if metaErr != nil {
		essentiaLog.I("head meta read failed name=%s path=%q err=%v", name, jsonPath, metaErr)
	}
	session, err := e.rt.NewSession(e.env, modelPath, &ort.SessionOptions{IntraOpNumThreads: 1})
	if err != nil {
		essentiaLog.I("head load failed name=%s model=%q err=%v", name, modelPath, err)
		return nil, err
	}
	inputs := session.InputNames()
	outputs := session.OutputNames()
	head := &essentiaHead{
		name:       name,
		inputName:  firstName(inputs),
		outputName: pickHeadPredictionOutput(name, outputs, meta.Classes),
		session:    session,
		classes:    append([]string{}, meta.Classes...),
		outputs:    append([]string{}, outputs...),
	}
	essentiaLog.I("head loaded name=%s input=%q output=%q classes=%d class0=%q class1=%q inputs=%v outputs=%v", name, head.inputName, head.outputName, len(head.classes), safeClass(head.classes, 0), safeClass(head.classes, 1), inputs, head.outputs)
	return head, nil
}

func (e *EssentiaEngine) loadGenre(modelsDir string) {
	modelPath := filepath.Join(modelsDir, "genre_discogs400-discogs-effnet-1.onnx")
	jsonPath := filepath.Join(modelsDir, "genre_discogs400-discogs-effnet-1.json")
	meta, metaErr := readEssentiaMeta(jsonPath)
	if metaErr != nil {
		essentiaLog.I("genre meta read failed path=%q err=%v", jsonPath, metaErr)
	}
	session, err := e.rt.NewSession(e.env, modelPath, &ort.SessionOptions{IntraOpNumThreads: 1})
	if err != nil {
		essentiaLog.I("genre load failed model=%q err=%v", modelPath, err)
		return
	}
	inputs := session.InputNames()
	outputs := session.OutputNames()
	e.genreSession = session
	e.genreInput = firstName(inputs)
	e.genreOutput = pickSessionOutput(outputs, "sigmoid", "predictions", "identity", "logits", "activations")
	e.genreClasses = append([]string{}, meta.Classes...)
	essentiaLog.I("genre loaded model=%q input=%q output=%q classes=%d inputs=%v outputs=%v firstClasses=%v", modelPath, e.genreInput, e.genreOutput, len(e.genreClasses), inputs, outputs, firstNStrings(e.genreClasses, 12))
	essentiaLog.I("genre classes sanity count=%d c0=%q c1=%q c2=%q c10=%q c50=%q c100=%q", len(e.genreClasses), safeClass(e.genreClasses, 0), safeClass(e.genreClasses, 1), safeClass(e.genreClasses, 2), safeClass(e.genreClasses, 10), safeClass(e.genreClasses, 50), safeClass(e.genreClasses, 100))

	if len(e.genreClasses) != 400 {
		essentiaLog.I("warning unexpected genre classes count=%d want=400", len(e.genreClasses))
	}

	checks := map[int]string{
		0:   "Blues /",
		314: "Rock /",
	}
	for index, prefix := range checks {
		if index >= len(e.genreClasses) {
			essentiaLog.I("genre label check index=%d is outside labels count=%d", index, len(e.genreClasses))
			continue
		}
		clean := cleanGenreLabel(e.genreClasses[index])
		if !strings.HasPrefix(clean, prefix) {
			essentiaLog.I("genre labels mismatch index=%d got=%q clean=%q expectedPrefix=%q", index, e.genreClasses[index], clean, prefix)
		}
	}
}

func (e *EssentiaEngine) Ready() bool { return e != nil && e.base != nil && e.genreSession != nil }

func (e *EssentiaEngine) BackendName() string {
	if e == nil {
		return "none"
	}
	if e.discogs != nil {
		return "discogs-bsdynamic"
	}
	return "discogs-bs64"
}

func (e *EssentiaEngine) enabledHeadNames() []string {
	if e == nil || len(e.headMap) == 0 {
		return nil
	}
	out := make([]string, 0, len(e.headMap))
	for _, name := range essentiaHeadNames {
		if head := e.headMap[name]; head != nil && head.session != nil {
			out = append(out, name)
		}
	}
	return out
}

func (e *EssentiaEngine) Close() error {
	if e == nil {
		return nil
	}
	if e.discogs != nil {
		_ = e.discogs.Close()
		e.discogs = nil
	}
	for _, head := range e.headMap {
		if head != nil && head.session != nil {
			head.session.Close()
		}
	}
	if e.genreSession != nil {
		e.genreSession.Close()
	}
	if e.base != nil {
		e.base.Close()
	}
	if e.env != nil {
		e.env.Close()
	}
	if e.rt != nil {
		_ = e.rt.Close()
	}
	return ReleaseEnvironment()
}

func (e *EssentiaEngine) Analyze(ctx context.Context, mel []float32, patches int) (EssentiaOutput, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.Ready() {
		return EssentiaOutput{}, errors.New("essentia not ready")
	}

	if e.discogs != nil {
		essentiaLog.I("analyze path=discogs-discogs")
		return e.analyzeWithDiscogs(ctx, mel, patches)
	}
	essentiaLog.I(
		"analyze backend=%s baseModel=%q genreHead=%q ready=%t",
		e.BackendName(),
		e.baseModelPath,
		e.genreModelPath,
		e.Ready(),
	)

	inputMel, validPatches, shape, err := prepareBaseMelInput(e.baseExpectedDims, mel, patches)
	if err != nil {
		return EssentiaOutput{}, err
	}
	const (
		effnetBatch = 64
		melBands    = 128
		melFrames   = 96
		patchSize   = melBands * melFrames
	)
	essentiaLog.I(
		"[mel] input tensor shape=[64,128,96] validPatches=%d len=%d expected=%d",
		validPatches,
		len(inputMel),
		effnetBatch*patchSize,
	)
	logPatchStats(inputMel, validPatches)
	dumpFloat32("/tmp/ray_mel_64x128x96.f32", inputMel)
	essentiaLog.D("analyze start mel=%d patches=%d validPatches=%d baseInput=%q genreReady=%t expectedShape=%v resolvedShape=%v", len(mel), patches, validPatches, e.baseInput, e.genreSession != nil, e.baseExpectedDims, shape)
	input, err := ort.NewTensorValue(e.rt, inputMel, shape)
	if err != nil {
		return EssentiaOutput{}, err
	}
	defer input.Close()
	baseInputName := e.baseInput
	if baseInputName == "" {
		return EssentiaOutput{}, errors.New("essentia base input missing")
	}
	baseRunStart := time.Now()
	outputs, err := e.base.Run(ctx, map[string]*ort.Value{baseInputName: input})
	if err != nil {
		essentiaLog.E("base run failed input=%q err=%v ms=%d", baseInputName, err, time.Since(baseRunStart).Milliseconds())
		return EssentiaOutput{}, err
	}
	essentiaLog.D("base run done ms=%d outputNames=%v", time.Since(baseRunStart).Milliseconds(), e.base.OutputNames())
	defer closeValues(outputs)
	embVal := outputs[e.baseEmbeddingOutput]
	if embVal == nil {
		return EssentiaOutput{}, errors.New("essentia embedding output missing")
	}
	embData, embShape, err := ort.GetTensorData[float32](embVal)
	if err != nil {
		return EssentiaOutput{}, err
	}
	dumpFloat32("/tmp/feuer_go_embeddings_64x1280.f32", embData)
	if len(embShape) != 2 {
		return EssentiaOutput{}, fmt.Errorf("essentia embedding output shape invalid: %v", embShape)
	}
	essentiaLog.D("base embeddings shape=%v len=%d validPatches=%d", embShape, len(embData), validPatches)
	embAvg, err := averageRows(embData, embShape, validPatches)
	if err != nil {
		return EssentiaOutput{}, err
	}
	patchEmbeddings, err := slicePatchEmbeddings(embData, embShape, validPatches)
	if err != nil {
		return EssentiaOutput{}, err
	}
	rawEmb := append([]float32{}, embAvg...)
	normEmb := l2Normalize(append([]float32{}, embAvg...))
	result := EssentiaOutput{Embedding: normEmb}
	minE, maxE, meanE, rmsE := summarizeFloat32(rawEmb)
	essentiaLog.I("embedding summary validPatches=%d dim=%d raw min=%.5f max=%.5f mean=%.5f rms=%.5f norm=%.5f", validPatches, len(rawEmb), minE, maxE, meanE, rmsE, l2Norm(rawEmb))
	genreData, genreShape, err := e.predictGenreFromPatchEmbeddings(ctx, patchEmbeddings, validPatches)
	if err != nil {
		return EssentiaOutput{}, err
	}
	genreAvg, err := averageRows(genreData, genreShape, validPatches)
	if err != nil {
		return EssentiaOutput{}, err
	}

	quality, qualityErr := inspectGenreOutput(
		genreData,
		validPatches,
		len(e.genreClasses),
	)
	if qualityErr != nil {
		essentiaLog.I("genre quality inspect failed: %v", qualityErr)
	}

	essentiaLog.I(
		"genre quality patches=%d classes=%d active=%d saturated=%d exactOne=%d nearOnePatches=%d nearOneRatio=%.3f dominantClass=%d dominantNearOne=%d dominantRatio=%.3f meanMax=%.4f suspicious=%t",
		quality.PatchCount,
		quality.ClassCount,
		quality.ActivePatches,
		quality.SaturatedPatches,
		quality.ExactOneCount,
		quality.NearOnePatchCount,
		quality.NearOnePatchRatio,
		quality.DominantClassIndex,
		quality.DominantNearOnePatches,
		quality.DominantPatchRatio,
		quality.MeanMaxScore,
		quality.Suspicious(),
	)

	subTop := topKGenres(genreAvg, e.genreClasses, 15)
	groupTop := topGenreGroups(genreAvg, e.genreClasses, 10)
	primary, detail, score, margin := choosePrimaryGenre(groupTop, subTop)
	result.GenrePrimary = primary
	result.GenreDetail = detail
	result.GenreScore = float64(score)
	result.GenreMargin = float64(margin)
	result.GenreTags = buildGenreTagsForUI(groupTop, 3)
	result.GenrePrimary, result.GenreDetail = choosePrimaryFromTags(result.GenreTags)
	result.GenreLabel = formatGenreTags(result.GenreTags)
	finalizeGenreResult(&result)
	result.GenreReliable = !quality.Suspicious()
	result.GenreQuality = quality

	if !result.GenreReliable ||
		result.GenreScore < 0.08 ||
		result.GenreMargin < 0.025 {
		essentiaLog.W(
			"genre rejected primary=%q detail=%q score=%.4f margin=%.4f reliable=%t",
			result.GenrePrimary,
			result.GenreDetail,
			result.GenreScore,
			result.GenreMargin,
			result.GenreReliable,
		)
		result.GenrePrimary = ""
		result.GenreDetail = ""
		result.GenreScore = 0
		result.GenreMargin = 0
		result.GenreTags = nil
		result.GenreLabel = ""
	}
	essentiaLog.I("genre from head validPatches=%d primary=%q label=%q detail=%q groupScore=%.3f groupMargin=%.3f groups=%+v subTop=%+v", validPatches, result.GenrePrimary, result.GenreLabel, result.GenreDetail, result.GenreScore, result.GenreMargin, groupTop, subTop)
	debugGenrePatchVotes(genreData, validPatches, e.genreClasses)

	headsStart := time.Now()
	for _, name := range essentiaHeadNames {
		head := e.headMap[name]
		if head == nil {
			essentiaLog.D("head skipped name=%s reason=missing-session", name)
			continue
		}
		probs, err := e.runHead(ctx, head, patchEmbeddings, validPatches)
		if err != nil {
			essentiaLog.E("head run failed name=%s err=%v", name, err)
			continue
		}
		if len(probs) == 0 {
			essentiaLog.D("head empty name=%s", name)
			continue
		}
		avg := averageHeadPredictions(probs, validPatches)
		if len(avg) == 0 {
			essentiaLog.D("head avg empty name=%s", name)
			continue
		}
		essentiaLog.T("head ok name=%s prob0=%.4f len=%d avg=%d", name, probs[0], len(probs), len(avg))
		switch name {
		case "danceability-discogs-effnet-1":
			result.Danceability = headClassProbability(head, avg, "danceable", "dance")
		case "mood_happy-discogs-effnet-1":
			result.MoodHappy = headClassProbability(head, avg, "happy")
		case "mood_sad-discogs-effnet-1":
			result.MoodSad = headClassProbability(head, avg, "sad")
		case "mood_relaxed-discogs-effnet-1":
			result.MoodRelaxed = headClassProbability(head, avg, "relaxed", "calm")
		case "mood_party-discogs-effnet-1":
			result.MoodParty = headClassProbability(head, avg, "party")
		case "mood_aggressive-discogs-effnet-1":
			result.MoodAggressive = headClassProbability(head, avg, "aggressive")
		case "mood_acoustic-discogs-effnet-1":
			result.Acousticness = headClassProbability(head, avg, "acoustic")
		case "mood_electronic-discogs-effnet-1":
			result.Electronic = headClassProbability(head, avg, "electronic")
		case "voice_instrumental-discogs-effnet-1":
			result.Instrumentalness = headClassProbability(head, avg, "instrumental")
		case "timbre-discogs-effnet-1":
			result.TimbreBrightness = headClassProbability(head, avg, "bright")
		case "tonal_atonal-discogs-effnet-1":
			result.Tonality = headClassProbability(head, avg, "tonal")
		case "approachability_regression-discogs-effnet-1":
			result.Approachability = regressionHeadValue(avg)
		case "engagement_regression-discogs-effnet-1":
			result.Engagement = regressionHeadValue(avg)
		case "mtg_jamendo_moodtheme-discogs-effnet-1":
			result.Melodicness = clamp01(
				0.65*classProb(avg, head.classes, "melodic") +
					0.20*classProb(avg, head.classes, "emotional") +
					0.15*classProb(avg, head.classes, "romantic"),
			)
			result.Softness = clamp01(
				0.45*classProb(avg, head.classes, "soft") +
					0.35*classProb(avg, head.classes, "calm") +
					0.20*classProb(avg, head.classes, "relaxing"),
			)
			result.Heaviness = clamp01(
				0.50*classProb(avg, head.classes, "heavy") +
					0.30*classProb(avg, head.classes, "powerful") +
					0.20*classProb(avg, head.classes, "dramatic"),
			)
			result.Dreaminess = clamp01(
				0.60*classProb(avg, head.classes, "dream") +
					0.25*classProb(avg, head.classes, "deep") +
					0.15*classProb(avg, head.classes, "romantic"),
			)
			result.Emotionality = clamp01(
				0.45*classProb(avg, head.classes, "emotional") +
					0.25*classProb(avg, head.classes, "melancholic") +
					0.15*classProb(avg, head.classes, "romantic") +
					0.15*classProb(avg, head.classes, "dramatic"),
			)
		}
	}

	result.Valence = clamp01((result.MoodHappy + (1 - result.MoodSad)) * 0.5)
	result.Energy = clamp01(maxf(result.MoodParty, result.MoodAggressive*0.85))
	result.Melodicness = clamp01(maxf(result.Melodicness, 0.15*result.Tonality))
	result.Softness = clamp01(maxf(result.Softness, 0.20*result.MoodRelaxed))
	result.Heaviness = clamp01(maxf(result.Heaviness, 0.30*result.MoodAggressive))
	essentiaLog.D("heads done ms=%d", time.Since(headsStart).Milliseconds())
	essentiaLog.D("derived dance=%.4f valence=%.4f acoustic=%.4f instr=%.4f energy=%.4f genre=%q detail=%q", result.Danceability, result.Valence, result.Acousticness, result.Instrumentalness, result.Energy, result.GenrePrimary, result.GenreDetail)
	return result, nil
}

func (e *EssentiaEngine) analyzeWithDiscogs(ctx context.Context, mel []float32, patches int) (EssentiaOutput, error) {
	const (
		discogsPatchFramesVal = 128
		discogsMelBandsVal    = 96
	)

	frameCount := len(mel) / discogsMelBandsVal
	if frameCount < discogsPatchFramesVal {
		frameCount = discogsPatchFramesVal
	}
	discogsMel, discogsFrames, err := makeDiscogsMelSpectrogramFromFlat(mel, frameCount, discogsMelBandsVal)
	if err != nil {
		return EssentiaOutput{}, fmt.Errorf("discogs mel prepare: %w", err)
	}
	patchesFlat, patchCount := analysis.MakeDiscogsPatches(discogsMel, discogsFrames)
	if patchCount == 0 {
		return EssentiaOutput{}, errors.New("no Discogs mel patches produced")
	}

	essentiaLog.I("discogs patches=%d len=%d", patchCount, len(patchesFlat))

	discogsResult, err := e.runDiscogs(patchesFlat, patchCount)
	if err != nil {
		return EssentiaOutput{}, err
	}

	result := EssentiaOutput{
		Backend:           "discogs-effnet-onnx",
		ModelPath:         "discogs-effnet-bsdynamic-1.onnx",
		DiscogsPatchCount: patchCount,
	}
	if len(discogsResult.MeanEmbedding) > 0 {
		result.Embedding = l2Normalize(append([]float32{}, discogsResult.MeanEmbedding...))
	}

	patchEmbeddings := make([]float32, 0, patchCount*1280)
	for _, pe := range discogsResult.PatchEmbeddings {
		patchEmbeddings = append(patchEmbeddings, pe...)
	}

	if len(discogsResult.MeanPredictions) > 0 && len(e.genreClasses) > 0 {
		subTop := topKGenres(discogsResult.MeanPredictions, e.genreClasses, 15)
		groupTop := topGenreGroups(discogsResult.MeanPredictions, e.genreClasses, 10)
		primary, detail, score, margin := choosePrimaryGenre(groupTop, subTop)
		result.GenrePrimary = primary
		result.GenreDetail = detail
		result.GenreScore = float64(score)
		result.GenreMargin = float64(margin)
		result.GenreTags = buildGenreTagsForUI(groupTop, 3)
		result.GenrePrimary, result.GenreDetail = choosePrimaryFromTags(result.GenreTags)
		result.GenreLabel = formatGenreTags(result.GenreTags)
		finalizeGenreResult(&result)

		patchPredictions := flattenPatchPredictions(discogsResult.PatchPredictions)
		quality, qualityErr := inspectGenreOutput(
			patchPredictions,
			len(discogsResult.PatchPredictions),
			len(e.genreClasses),
		)
		if qualityErr != nil {
			essentiaLog.I("discogs genre quality inspect failed: %v", qualityErr)
		}
		result.GenreReliable = qualityErr == nil && !quality.Suspicious()
		result.GenreQuality = quality

		if qualityErr == nil {
			essentiaLog.I(
				"discogs genre quality patches=%d classes=%d active=%d saturated=%d exactOne=%d nearOnePatches=%d nearOneRatio=%.3f dominantClass=%d dominantNearOne=%d dominantRatio=%.3f meanMax=%.4f suspicious=%t",
				quality.PatchCount,
				quality.ClassCount,
				quality.ActivePatches,
				quality.SaturatedPatches,
				quality.ExactOneCount,
				quality.NearOnePatchCount,
				quality.NearOnePatchRatio,
				quality.DominantClassIndex,
				quality.DominantNearOnePatches,
				quality.DominantPatchRatio,
				quality.MeanMaxScore,
				quality.Suspicious(),
			)
		}

		if !result.GenreReliable ||
			result.GenreScore < 0.08 ||
			result.GenreMargin < 0.025 {
			essentiaLog.W(
				"discogs genre rejected primary=%q detail=%q score=%.4f margin=%.4f reliable=%t",
				result.GenrePrimary,
				result.GenreDetail,
				result.GenreScore,
				result.GenreMargin,
				result.GenreReliable,
			)
			result.GenrePrimary = ""
			result.GenreDetail = ""
			result.GenreScore = 0
			result.GenreMargin = 0
			result.GenreTags = nil
			result.GenreLabel = ""
		}
	}

	headsStart := time.Now()
	for _, name := range essentiaHeadNames {
		head := e.headMap[name]
		if head == nil {
			continue
		}
		probs, err := e.runHead(ctx, head, patchEmbeddings, patchCount)
		if err != nil {
			essentiaLog.E("head run failed name=%s err=%v", name, err)
			continue
		}
		if len(probs) == 0 {
			continue
		}
		avg := averageHeadPredictions(probs, patchCount)
		if len(avg) == 0 {
			continue
		}
		switch name {
		case "danceability-discogs-effnet-1":
			result.Danceability = headClassProbability(head, avg, "danceable", "dance")
		case "mood_happy-discogs-effnet-1":
			result.MoodHappy = headClassProbability(head, avg, "happy")
		case "mood_sad-discogs-effnet-1":
			result.MoodSad = headClassProbability(head, avg, "sad")
		case "mood_relaxed-discogs-effnet-1":
			result.MoodRelaxed = headClassProbability(head, avg, "relaxed", "calm")
		case "mood_party-discogs-effnet-1":
			result.MoodParty = headClassProbability(head, avg, "party")
		case "mood_aggressive-discogs-effnet-1":
			result.MoodAggressive = headClassProbability(head, avg, "aggressive")
		case "mood_acoustic-discogs-effnet-1":
			result.Acousticness = headClassProbability(head, avg, "acoustic")
		case "mood_electronic-discogs-effnet-1":
			result.Electronic = headClassProbability(head, avg, "electronic")
		case "voice_instrumental-discogs-effnet-1":
			result.Instrumentalness = headClassProbability(head, avg, "instrumental")
		case "timbre-discogs-effnet-1":
			result.TimbreBrightness = headClassProbability(head, avg, "bright")
		case "tonal_atonal-discogs-effnet-1":
			result.Tonality = headClassProbability(head, avg, "tonal")
		case "approachability_regression-discogs-effnet-1":
			result.Approachability = regressionHeadValue(avg)
		case "engagement_regression-discogs-effnet-1":
			result.Engagement = regressionHeadValue(avg)
		case "mtg_jamendo_moodtheme-discogs-effnet-1":
			result.Melodicness = clamp01(
				0.65*classProb(avg, head.classes, "melodic") +
					0.20*classProb(avg, head.classes, "emotional") +
					0.15*classProb(avg, head.classes, "romantic"),
			)
			result.Softness = clamp01(
				0.45*classProb(avg, head.classes, "soft") +
					0.35*classProb(avg, head.classes, "calm") +
					0.20*classProb(avg, head.classes, "relaxing"),
			)
			result.Heaviness = clamp01(
				0.50*classProb(avg, head.classes, "heavy") +
					0.30*classProb(avg, head.classes, "powerful") +
					0.20*classProb(avg, head.classes, "dramatic"),
			)
			result.Dreaminess = clamp01(
				0.60*classProb(avg, head.classes, "dream") +
					0.25*classProb(avg, head.classes, "deep") +
					0.15*classProb(avg, head.classes, "romantic"),
			)
			result.Emotionality = clamp01(
				0.45*classProb(avg, head.classes, "emotional") +
					0.25*classProb(avg, head.classes, "melancholic") +
					0.15*classProb(avg, head.classes, "romantic") +
					0.15*classProb(avg, head.classes, "dramatic"),
			)
		}
	}

	result.Valence = clamp01((result.MoodHappy + (1 - result.MoodSad)) * 0.5)
	result.Energy = clamp01(maxf(result.MoodParty, result.MoodAggressive*0.85))
	result.Melodicness = clamp01(maxf(result.Melodicness, 0.15*result.Tonality))
	result.Softness = clamp01(maxf(result.Softness, 0.20*result.MoodRelaxed))
	result.Heaviness = clamp01(maxf(result.Heaviness, 0.30*result.MoodAggressive))
	essentiaLog.D("discogs heads done ms=%d", time.Since(headsStart).Milliseconds())
	if err := result.ValidateSemanticOutput(); err != nil {
		return EssentiaOutput{}, fmt.Errorf("Discogs semantic output validation failed: %w", err)
	}
	return result, nil
}

func (e *EssentiaEngine) runDiscogs(patches []float32, patchCount int) (struct {
	PatchPredictions [][]float32
	PatchEmbeddings  [][]float32
	MeanPredictions  []float32
	MeanEmbedding    []float32
}, error) {
	const (
		classCount    = 400
		embeddingSize = 1280
		patchFrames   = 128
		melBands      = 96
	)

	type discogsAgg struct {
		PatchPredictions [][]float32
		PatchEmbeddings  [][]float32
		MeanPredictions  []float32
		MeanEmbedding    []float32
	}

	outputs, err := e.discogs.Run(
		patches,
		[]int64{int64(patchCount), int64(patchFrames), int64(melBands)},
		[][]int64{
			{int64(patchCount), int64(classCount)},
			{int64(patchCount), int64(embeddingSize)},
		},
	)
	if err != nil {
		return discogsAgg{}, err
	}

	predSize := 0
	embSize := 0
	if len(outputs) > 0 {
		predSize = len(outputs[0])
	}
	if len(outputs) > 1 {
		embSize = len(outputs[1])
	}
	essentiaLog.I("discogs raw outputs patches=%d outputCount=%d predictionValues=%d embeddingValues=%d expectedPredictions=%d expectedEmbeddings=%d",
		patchCount, len(outputs), predSize, embSize, patchCount*classCount, patchCount*embeddingSize)

	if len(outputs) != 2 {
		return discogsAgg{}, fmt.Errorf("discogs output count=%d want=2", len(outputs))
	}

	predictionsFlat := outputs[0]
	embeddingsFlat := outputs[1]

	if len(predictionsFlat) != patchCount*classCount {
		return discogsAgg{}, fmt.Errorf("discogs prediction output size=%d want=%d", len(predictionsFlat), patchCount*classCount)
	}
	if len(embeddingsFlat) != patchCount*embeddingSize {
		return discogsAgg{}, fmt.Errorf("discogs embedding output size=%d want=%d", len(embeddingsFlat), patchCount*embeddingSize)
	}

	result := discogsAgg{
		PatchPredictions: make([][]float32, patchCount),
		PatchEmbeddings:  make([][]float32, patchCount),
		MeanPredictions:  make([]float32, classCount),
		MeanEmbedding:    make([]float32, embeddingSize),
	}

	for patch := 0; patch < patchCount; patch++ {
		predStart := patch * classCount
		embStart := patch * embeddingSize

		result.PatchPredictions[patch] = append([]float32(nil), predictionsFlat[predStart:predStart+classCount]...)
		result.PatchEmbeddings[patch] = append([]float32(nil), embeddingsFlat[embStart:embStart+embeddingSize]...)

		for i := 0; i < classCount; i++ {
			result.MeanPredictions[i] += result.PatchPredictions[patch][i]
		}
		for i := 0; i < embeddingSize; i++ {
			result.MeanEmbedding[i] += result.PatchEmbeddings[patch][i]
		}
	}

	inv := float32(1) / float32(patchCount)
	for i := range result.MeanPredictions {
		result.MeanPredictions[i] *= inv
	}
	for i := range result.MeanEmbedding {
		result.MeanEmbedding[i] *= inv
	}

	return result, nil
}

func makeDiscogsMelSpectrogramFromFlat(mel []float32, frameCount, melBands int) ([]float32, int, error) {
	if len(mel) == 0 {
		return nil, 0, errors.New("empty mel")
	}
	return mel, frameCount, nil
}

func (e *EssentiaEngine) runHead(ctx context.Context, head *essentiaHead, patchEmbeddings []float32, validPatches int) ([]float32, error) {
	if head == nil || head.session == nil {
		return nil, errors.New("head session unavailable")
	}
	inputName := head.inputName
	if inputName == "" {
		return nil, errors.New("head input missing")
	}
	if validPatches <= 0 {
		return nil, errors.New("invalid head patch count")
	}
	const dim = 1280
	if len(patchEmbeddings) != validPatches*dim {
		return nil, fmt.Errorf("head patch embeddings len=%d expected=%d", len(patchEmbeddings), validPatches*dim)
	}
	outputName := head.outputName
	if outputName == "" {
		outputName = ""
	}
	essentiaLog.T("head input name=%s inputLen=%d output=%s shape=[%d,1280]", inputName, len(patchEmbeddings), outputName, validPatches)
	input, err := ort.NewTensorValue(e.rt, patchEmbeddings, []int64{int64(validPatches), dim})
	if err != nil {
		return nil, err
	}
	defer input.Close()
	outputs, err := head.session.Run(ctx, map[string]*ort.Value{inputName: input})
	if err != nil {
		return nil, err
	}
	defer closeValues(outputs)
	var out *ort.Value
	if outputName != "" {
		out = outputs[outputName]
	}
	if out == nil {
		for _, name := range head.outputs {
			if strings.Contains(strings.ToLower(name), "softmax") || strings.Contains(strings.ToLower(name), "sigmoid") || strings.Contains(strings.ToLower(name), "identity") {
				out = outputs[name]
				if out != nil {
					break
				}
			}
		}
	}
	if out == nil {
		for _, v := range outputs {
			out = v
			break
		}
	}
	if out == nil {
		return nil, errors.New("head output missing")
	}
	data, _, err := ort.GetTensorData[float32](out)
	if err != nil {
		return nil, err
	}
	return append([]float32{}, data...), nil
}

func (e *EssentiaEngine) predictGenreFromPatchEmbeddings(ctx context.Context, patchEmbeddings []float32, validPatches int) ([]float32, []int64, error) {
	if e.genreSession == nil {
		return nil, nil, errors.New("genre session unavailable")
	}
	if validPatches <= 0 {
		return nil, nil, fmt.Errorf("validPatches=%d", validPatches)
	}
	const dim = 1280
	if len(patchEmbeddings) != validPatches*dim {
		return nil, nil, fmt.Errorf("genre patch embeddings len=%d expected=%d", len(patchEmbeddings), validPatches*dim)
	}
	inputName := e.genreInput
	if inputName == "" {
		return nil, nil, errors.New("genre input missing")
	}
	outputName := e.genreOutput
	if outputName == "" {
		return nil, nil, errors.New("genre output missing")
	}
	essentiaLog.I("genre head input=%q output=%q classes=%d shape=[%d,1280]", inputName, outputName, len(e.genreClasses), validPatches)
	input, err := ort.NewTensorValue(e.rt, patchEmbeddings, []int64{int64(validPatches), dim})
	if err != nil {
		return nil, nil, err
	}
	defer input.Close()
	outputs, err := e.genreSession.Run(ctx, map[string]*ort.Value{inputName: input})
	if err != nil {
		return nil, nil, err
	}
	defer closeValues(outputs)
	var out *ort.Value
	if outputName != "" {
		out = outputs[outputName]
	}
	if out == nil {
		for name, v := range outputs {
			ln := strings.ToLower(name)
			if strings.Contains(ln, "sigmoid") || strings.Contains(ln, "predictions") || strings.Contains(ln, "identity") {
				out = v
				break
			}
		}
	}
	if out == nil {
		for _, v := range outputs {
			out = v
			break
		}
	}
	if out == nil {
		return nil, nil, errors.New("genre output missing")
	}
	data, shapeOut, err := ort.GetTensorData[float32](out)
	if err != nil || len(data) == 0 {
		return nil, nil, err
	}
	minV, maxV, meanV, rmsV := summarizeFloat32(data)
	var top []genreCandidate
	if avg, err := averageRows(data, shapeOut, validPatches); err == nil {
		top = topKGenres(avg, e.genreClasses, 10)
	} else {
		essentiaLog.I("debug topK average failed: %v", err)
		top = []genreCandidate{}
	}
	essentiaLog.I("genre head output=%q shape=%v len=%d classes=%d min=%.5f max=%.5f mean=%.5f rms=%.5f top=%+v", outputName, shapeOut, len(data), len(e.genreClasses), minV, maxV, meanV, rmsV, top)
	for name, val := range outputs {
		candidateData, candidateShape, candidateErr := ort.GetTensorData[float32](val)
		if candidateErr != nil || len(candidateData) == 0 {
			continue
		}
		candidateTop := topKGenres(candidateData, e.genreClasses, 5)
		essentiaLog.T("genre output candidate name=%q shape=%v len=%d top=%+v", name, candidateShape, len(candidateData), candidateTop)
	}
	return append([]float32{}, data...), shapeOut, nil
}

func (e *EssentiaEngine) predictGenre(ctx context.Context, embedding []float32) (GenrePrediction, error) {
	if len(embedding) == 0 {
		return GenrePrediction{}, errors.New("genre embedding empty")
	}
	data, shape, err := e.predictGenreFromPatchEmbeddings(ctx, embedding, 1)
	if err != nil {
		return GenrePrediction{}, err
	}
	if len(shape) != 2 || len(data) == 0 {
		return GenrePrediction{}, errors.New("genre output missing")
	}
	avg, err := averageRows(data, shape, 1)
	if err != nil {
		return GenrePrediction{}, err
	}
	bestIdx := 0
	bestVal := avg[0]
	secondVal := float32(-1)
	for i := 1; i < len(avg); i++ {
		if avg[i] > bestVal {
			secondVal = bestVal
			bestVal = avg[i]
			bestIdx = i
		} else if avg[i] > secondVal {
			secondVal = avg[i]
		}
	}
	label := safeClass(e.genreClasses, bestIdx)
	if strings.TrimSpace(label) == "" {
		label = fmt.Sprintf("genre_%d", bestIdx)
	}
	label = cleanGenreLabel(label)
	margin := float64(bestVal - secondVal)
	essentiaLog.I("genre best idx=%d value=%.5f margin=%.5f label=%q", bestIdx, bestVal, margin, label)
	return GenrePrediction{Label: label, Score: float64(bestVal), Margin: margin, Top: topKGenres(avg, e.genreClasses, 10)}, nil
}

func readEssentiaMeta(path string) (essentiaModelMeta, error) {
	var meta essentiaModelMeta
	raw, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimSpace(names[0])
}

func pickSessionInput(names []string, preferred ...string) string {
	return pickSessionOutput(names, preferred...)
}

func pickSessionOutput(names []string, preferred ...string) string {
	for _, want := range preferred {
		want = strings.ToLower(strings.TrimSpace(want))
		if want == "" {
			continue
		}
		for _, name := range names {
			if strings.Contains(strings.ToLower(name), want) {
				return strings.TrimSpace(name)
			}
		}
	}
	return firstName(names)
}

func topKGenres(data []float32, classes []string, k int) []genreCandidate {
	if k <= 0 {
		k = 10
	}
	items := make([]genreCandidate, 0, len(data))
	for i, v := range data {
		if v <= 1e-8 {
			continue
		}
		label := fmt.Sprintf("genre_%d", i)
		if i < len(classes) && strings.TrimSpace(classes[i]) != "" {
			label = cleanGenreLabel(classes[i])
		}
		items = append(items, genreCandidate{Idx: i, Label: label, Score: v})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Score > items[j].Score })
	if len(items) > k {
		items = items[:k]
	}
	return items
}

func topGenreGroups(probs []float32, classes []string, k int) []GenreGroupCandidate {
	groups := map[string]*GenreGroupCandidate{}
	for i, score := range probs {
		if i >= len(classes) || score <= 1e-8 {
			continue
		}
		detail := cleanGenreLabel(classes[i])
		parent := parentGenre(detail)
		if parent == "" {
			continue
		}
		g := groups[parent]
		if g == nil {
			g = &GenreGroupCandidate{Label: parent}
			groups[parent] = g
		}
		if score > g.BestSubScore {
			g.BestSubScore = score
			g.BestSubLabel = detail
		}
		if score >= 0.02 {
			g.Support++
			g.SumScore += score
		}
	}
	out := make([]GenreGroupCandidate, 0, len(groups))
	for _, g := range groups {
		if g.BestSubScore <= 0 {
			continue
		}
		avgSupported := float32(0)
		if g.Support > 0 {
			avgSupported = g.SumScore / float32(g.Support)
		}
		g.Score = 0.85*g.BestSubScore + 0.15*avgSupported
		if g.Support <= 1 && g.BestSubScore < 0.04 {
			g.Score *= 0.5
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > k {
		out = out[:k]
	}
	return out
}

func buildGenreTagsForUI(groups []GenreGroupCandidate, limit int) []GenreTag {
	if limit <= 0 {
		limit = 3
	}
	if len(groups) == 0 {
		return nil
	}
	tags := make([]GenreTag, 0, limit)
	topScore := groups[0].Score
	for _, g := range groups {
		if len(tags) >= limit {
			break
		}
		label := strings.TrimSpace(g.Label)
		if label == "" || isNoisyDisplayGenre(label) {
			continue
		}
		if len(tags) == 0 {
			if !acceptTopGenre(g) {
				continue
			}
		} else {
			if !acceptSecondaryGenre(g, topScore) {
				continue
			}
		}
		tags = append(tags, GenreTag{Label: label, Detail: genreDetailForUI(g), Score: float64(g.Score), Rank: len(tags) + 1, Support: g.Support})
	}
	return tags
}

func choosePrimaryFromTags(tags []GenreTag) (string, string) {
	if len(tags) == 0 {
		return "", ""
	}
	return tags[0].Label, tags[0].Detail
}

func formatGenreTags(tags []GenreTag) string {
	if len(tags) == 0 {
		return ""
	}
	labels := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		label := strings.TrimSpace(tag.Label)
		if label == "" {
			continue
		}
		if seen[label] {
			continue
		}
		labels = append(labels, label)
		seen[label] = true
		if len(labels) >= 3 {
			break
		}
	}
	return strings.Join(labels, ", ")
}

func finalizeGenreResult(result *EssentiaOutput) {
	if result == nil {
		return
	}
	if len(result.GenreTags) == 0 {
		result.GenrePrimary = ""
		result.GenreDetail = ""
		result.GenreLabel = ""
		result.GenreScore = 0
		result.GenreMargin = 0
		return
	}
	top := result.GenreTags[0]
	result.GenrePrimary = top.Label
	result.GenreDetail = top.Detail
	result.GenreLabel = formatGenreTags(result.GenreTags)
	result.GenreScore = top.Score
	if len(result.GenreTags) > 1 {
		result.GenreMargin = result.GenreTags[0].Score - result.GenreTags[1].Score
	} else {
		result.GenreMargin = result.GenreTags[0].Score
	}
}

func acceptTopGenre(g GenreGroupCandidate) bool {
	if isNoisyDisplayGenre(g.Label) {
		return false
	}
	switch g.Label {
	case "Rock", "Electronic", "Pop", "Hip Hop", "Funk", "Reggae", "Jazz":
		return g.Score >= 0.035 || g.BestSubScore >= 0.05
	default:
		return g.Score >= 0.05 || g.BestSubScore >= 0.07
	}
}

func acceptSecondaryGenre(g GenreGroupCandidate, topScore float32) bool {
	if isNoisyDisplayGenre(g.Label) {
		return false
	}
	if g.Score < 0.03 {
		return false
	}
	if g.Score < topScore*0.35 {
		return false
	}
	if g.Support <= 1 && g.Score < 0.04 {
		return false
	}
	return true
}

func isNoisyDisplayGenre(label string) bool {
	switch strings.TrimSpace(label) {
	case "Non-Music", "Stage & Screen", "Children's", "Brass & Military", "Classical", "Score":
		return true
	default:
		return false
	}
}

func logPatchStats(mel []float32, validPatches int) {
	const patchSize = 128 * 96

	for p := 0; p < minInt(validPatches, 5); p++ {
		start := p * patchSize
		end := start + patchSize
		if end > len(mel) {
			break
		}

		minV, maxV, meanV, rmsV := summarizeFloat32(mel[start:end])
		essentiaLog.I(
			"[mel] patch=%02d min=%.5f max=%.5f mean=%.5f rms=%.5f",
			p,
			minV,
			maxV,
			meanV,
			rmsV,
		)
	}
}

func debugGenrePatchVotes(genreData []float32, validPatches int, classes []string) {
	const dims = 400
	if validPatches <= 0 {
		return
	}
	parentVotes := map[string]int{}

	for p := 0; p < validPatches; p++ {
		start := p * dims
		end := start + dims
		if end > len(genreData) {
			break
		}
		row := genreData[start:end]
		top := topKGenres(row, classes, 3)

		if len(top) > 0 {
			parent := parentGenre(top[0].Label)
			if parent != "" {
				parentVotes[parent]++
			}
		}
		if len(top) > 0 {
			essentiaLog.D(
				"patch=%02d top=%+v",
				p,
				top,
			)
		}
	}
	if len(parentVotes) > 0 {
		essentiaLog.I("parentVotes=%+v", parentVotes)
	}
}

func choosePrimaryGenre(groups []GenreGroupCandidate, subTop []genreCandidate) (string, string, float32, float32) {
	if len(groups) > 0 {
		top := groups[0]
		score := top.Score
		margin := float32(0)
		if len(groups) > 1 {
			margin = groups[0].Score - groups[1].Score
		}
		if score >= 0.06 || margin >= 0.02 {
			return top.Label, top.BestSubLabel, score, margin
		}
	}
	if len(subTop) > 0 && subTop[0].Score >= 0.08 {
		primary := parentGenre(subTop[0].Label)
		if primary != "" {
			return primary, subTop[0].Label, subTop[0].Score, 0
		}
	}
	return "", "", 0, 0
}

func firstNStrings(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return xs[:n]
}

func safeClass(classes []string, idx int) string {
	if idx < 0 || idx >= len(classes) {
		return ""
	}
	return classes[idx]
}

func parentGenre(label string) string {
	label = cleanGenreLabel(label)
	if label == "" {
		return ""
	}
	parts := strings.Split(label, " / ")
	parent := strings.TrimSpace(parts[0])
	if parent == "" || strings.EqualFold(parent, "Score") {
		return ""
	}
	return parent
}

func genreDetailForUI(g GenreGroupCandidate) string {
	detail := cleanGenreLabel(g.BestSubLabel)
	if detail == "" || g.Support < 2 || g.BestSubScore < 0.16 {
		return ""
	}
	switch detail {
	case "Rock / Black Metal", "Rock / Funeral Doom Metal", "Hip Hop / DJ Battle Tool":
		if g.BestSubScore < 0.24 {
			return ""
		}
	}
	return detail
}

func flattenPatchPredictions(rows [][]float32) []float32 {
	if len(rows) == 0 {
		return nil
	}
	total := 0
	for _, row := range rows {
		total += len(row)
	}
	out := make([]float32, 0, total)
	for _, row := range rows {
		out = append(out, row...)
	}
	return out
}

func averageHeadPredictions(data []float32, validRows int) []float32 {
	if validRows <= 0 || len(data) == 0 {
		return nil
	}
	if len(data)%validRows != 0 {
		return append([]float32{}, data...)
	}
	dims := len(data) / validRows
	out := make([]float32, dims)
	for row := 0; row < validRows; row++ {
		base := row * dims
		for i := 0; i < dims; i++ {
			out[i] += data[base+i]
		}
	}
	inv := float32(1.0 / float64(validRows))
	for i := range out {
		out[i] *= inv
	}
	return out
}

func regressionHeadValue(probs []float32) float64 {
	if len(probs) == 0 {
		return 0
	}
	v := float64(probs[0])
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return clamp01(v)
}

func headClassProbability(head *essentiaHead, probs []float32, positiveHints ...string) float64 {
	if head == nil {
		return 0
	}
	if idx := findClassIndex(head.classes, positiveHints...); idx >= 0 && idx < len(probs) {
		return clamp01(float64(probs[idx]))
	}
	return positiveClassProb(probs, head.classes, positiveHints...)
}

func classProb(probs []float32, classes []string, positiveHints ...string) float64 {
	return positiveClassProb(probs, classes, positiveHints...)
}

func positiveClassProb(probs []float32, classes []string, positiveHints ...string) float64 {
	if len(probs) == 0 {
		return 0
	}
	if idx := findClassIndex(classes, positiveHints...); idx >= 0 && idx < len(probs) {
		return clamp01(float64(probs[idx]))
	}
	if len(probs) == 1 {
		return clamp01(float64(probs[0]))
	}
	if len(probs) == 2 {
		return clamp01(float64(probs[1]))
	}
	return clamp01(float64(probs[0]))
}

func findClassIndex(classes []string, positiveHints ...string) int {
	for _, hint := range positiveHints {
		h := strings.ToLower(strings.TrimSpace(hint))
		if h == "" {
			continue
		}
		for i, cls := range classes {
			lc := strings.ToLower(strings.TrimSpace(cls))
			if lc == h {
				return i
			}
		}
		for i, cls := range classes {
			lc := strings.ToLower(strings.TrimSpace(cls))
			if strings.Contains(lc, h) {
				return i
			}
		}
	}
	return -1
}

func pickHeadPredictionOutput(name string, outputs []string, classes []string) string {
	preferred := []string{"softmax", "sigmoid", "predictions", "identity"}
	if strings.Contains(strings.ToLower(name), "regression") {
		preferred = []string{"identity", "prediction", "predictions", "output"}
	}
	if out := pickSessionOutput(outputs, preferred...); out != "" {
		return out
	}
	if len(outputs) == 1 {
		return outputs[0]
	}
	if out := pickSessionOutput(outputs, "activations"); out != "" {
		essentiaLog.I("head output warning using activations name=%s outputs=%v classes=%d", name, outputs, len(classes))
		return out
	}
	return firstName(outputs)
}

func summarizeFloat32(xs []float32) (min, max, mean, rms float32) {
	if len(xs) == 0 {
		return 0, 0, 0, 0
	}
	min, max = xs[0], xs[0]
	var sum, sumSq float64
	for _, x := range xs {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
		sum += float64(x)
		sumSq += float64(x * x)
	}
	mean = float32(sum / float64(len(xs)))
	rms = float32(localSqrt(sumSq / float64(len(xs))))
	return
}

func l2Norm(xs []float32) float64 {
	var sum float64
	for _, x := range xs {
		sum += float64(x * x)
	}
	return localSqrt(sum)
}

func localSqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Sqrt(v)
}

func averageRows(data []float32, shape []int64, validRows int) ([]float32, error) {
	if len(shape) != 2 {
		return nil, fmt.Errorf("unexpected rows shape: %v", shape)
	}
	rows := int(shape[0])
	dims := int(shape[1])
	if rows <= 0 || dims <= 0 {
		return nil, fmt.Errorf("invalid rows shape: %v", shape)
	}
	if len(data) < rows*dims {
		return nil, fmt.Errorf("rows data too short: len=%d shape=%v", len(data), shape)
	}
	if validRows <= 0 || validRows > rows {
		validRows = rows
	}
	out := make([]float32, dims)
	for r := 0; r < validRows; r++ {
		base := r * dims
		for d := 0; d < dims; d++ {
			out[d] += data[base+d]
		}
	}
	inv := float32(1.0 / float64(validRows))
	for d := range out {
		out[d] *= inv
	}
	return out, nil
}

func slicePatchEmbeddings(data []float32, shape []int64, validRows int) ([]float32, error) {
	if len(shape) != 2 {
		return nil, fmt.Errorf("unexpected patch shape: %v", shape)
	}
	rows := int(shape[0])
	dims := int(shape[1])
	if rows <= 0 || dims <= 0 {
		return nil, fmt.Errorf("invalid patch shape: %v", shape)
	}
	if len(data) < rows*dims {
		return nil, fmt.Errorf("patch data too short: len=%d shape=%v", len(data), shape)
	}
	if validRows <= 0 || validRows > rows {
		validRows = rows
	}
	out := make([]float32, validRows*dims)
	copy(out, data[:validRows*dims])
	return out, nil
}

func averageEmbeddings(data []float32, shape []int64) []float32 {
	if len(shape) < 2 || len(data) == 0 {
		return append([]float32{}, data...)
	}
	rows := int(shape[0])
	dims := int(shape[1])
	if rows <= 0 || dims <= 0 {
		return append([]float32{}, data...)
	}
	out := make([]float32, dims)
	for r := 0; r < rows; r++ {
		base := r * dims
		if base+dims > len(data) {
			break
		}
		for d := 0; d < dims; d++ {
			out[d] += data[base+d]
		}
	}
	for d := range out {
		out[d] /= float32(rows)
	}
	return out
}

func meanEmbedding(
	values []float32,
	rows int,
	columns int,
) []float32 {
	result := make([]float32, columns)
	if rows <= 0 || columns <= 0 {
		return result
	}

	for row := 0; row < rows; row++ {
		offset := row * columns
		for column := 0; column < columns; column++ {
			result[column] +=
				values[offset+column]
		}
	}

	scale := float32(1) / float32(rows)
	for index := range result {
		result[index] *= scale
	}
	return result
}

func closeValues(values map[string]*ort.Value) {
	for _, v := range values {
		if v != nil {
			v.Close()
		}
	}
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

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func cleanGenreLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	label = strings.ReplaceAll(label, "---", " / ")
	label = strings.ReplaceAll(label, "--", " / ")
	label = strings.Join(strings.Fields(label), " ")
	return label
}

func metaFromPath(path string) essentiaModelMeta {
	meta, err := readEssentiaMeta(path)
	if err != nil {
		return essentiaModelMeta{}
	}
	return meta
}

func metaInputs(meta essentiaModelMeta) []essentiaIO {
	return meta.Schema.Inputs
}

func firstShape(inputs []essentiaIO) []interface{} {
	if len(inputs) == 0 {
		return nil
	}
	return append([]interface{}{}, inputs[0].Shape...)
}

func prepareBaseMelInput(expected []interface{}, mel []float32, patches int) ([]float32, int, []int64, error) {
	patchSize := 128 * 96
	if len(expected) == 3 {
		if second, ok := expected[1].(float64); ok && second > 0 {
			patchSize = int(second)
			if third, ok := expected[2].(float64); ok && third > 0 {
				patchSize *= int(third)
			}
		}
	}
	if patches <= 0 && patchSize > 0 && len(mel)%patchSize == 0 {
		patches = len(mel) / patchSize
	}
	shape, err := resolveBaseInputShape(expected, patches, len(mel))
	if err != nil {
		return nil, 0, nil, err
	}
	rows := int(shape[0])
	bands := int(shape[1])
	frames := int(shape[2])
	patchSize = bands * frames
	valid := patches
	if valid > rows {
		valid = rows
	}
	if valid <= 0 && patchSize > 0 && len(mel)%patchSize == 0 {
		valid = len(mel) / patchSize
		if valid > rows {
			valid = rows
		}
	}
	out := make([]float32, rows*patchSize)
	copyLen := valid * patchSize
	if copyLen > len(mel) {
		copyLen = len(mel)
	}
	copy(out[:copyLen], mel[:copyLen])
	return out, valid, shape, nil
}

func resolveBaseInputShape(expected []interface{}, patches int, melLen int) ([]int64, error) {
	if len(expected) == 0 {
		return []int64{int64(patches), 128, 96}, nil
	}
	if len(expected) != 3 {
		return nil, fmt.Errorf("unsupported base input rank: %v", expected)
	}
	first := expected[0]
	second, okSecond := expected[1].(float64)
	third, okThird := expected[2].(float64)
	if !okSecond || !okThird {
		return nil, fmt.Errorf("unsupported base input shape: %v", expected)
	}
	bands := int(second)
	frames := int(third)
	if bands <= 0 || frames <= 0 {
		return nil, fmt.Errorf("invalid base input shape: %v", expected)
	}
	rows := patches
	if fixed, ok := first.(float64); ok && int(fixed) > 0 {
		rows = int(fixed)
	}
	if bands*frames*patches != melLen {
		return nil, fmt.Errorf("mel size mismatch: expected %d values for patches=%d shape=%v, got %d", bands*frames*patches, patches, expected, melLen)
	}
	return []int64{int64(rows), int64(bands), int64(frames)}, nil
}

func dumpFloat32(path string, xs []float32) {
	f, err := os.Create(path)
	if err != nil {
		essentiaLog.I("dump create failed: %v", err)
		return
	}
	defer f.Close()

	_ = binary.Write(f, binary.LittleEndian, xs)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
