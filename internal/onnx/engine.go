package onnx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

const defaultMaxSeqLen = 96

func TestRuntime(runtimePath string) error {
	if err := AcquireEnvironmentWithPath(strings.TrimSpace(runtimePath)); err != nil {
		return err
	}
	return ReleaseEnvironment()
}

type Engine struct {
	mu          sync.Mutex
	session     *ort.DynamicSession[int64, float32]
	tokenizer   *tokenizer.Tokenizer
	inputNames  []string
	outputNames []string
	outputInfo  []ort.InputOutputInfo
}

func New(runtimePath, modelDir string) (*Engine, error) {
	model, tok, err := ResolveModelFiles(modelDir)
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

	inputs, outputs, err := ort.GetInputOutputInfo(model)
	if err != nil {
		return nil, err
	}
	inputNames := make([]string, 0, len(inputs))
	for _, in := range inputs {
		inputNames = append(inputNames, in.Name)
	}
	outputNames := make([]string, 0, len(outputs))
	for _, out := range outputs {
		outputNames = append(outputNames, out.Name)
	}
	session, err := ort.NewDynamicSession[int64, float32](model, inputNames, outputNames)
	if err != nil {
		return nil, err
	}
	tk, err := pretrained.FromFile(tok)
	if err != nil {
		_ = session.Destroy()
		return nil, err
	}
	cleanupEnv = false
	return &Engine{session: session, tokenizer: tk, inputNames: inputNames, outputNames: outputNames, outputInfo: outputs}, nil
}

func ResolveModelFiles(modelDir string) (string, string, error) {
	modelDir = strings.TrimSpace(modelDir)
	if modelDir == "" {
		return "", "", errors.New("onnx model dir empty")
	}
	candidates := []string{
		filepath.Join(modelDir, "model_quantized.onnx"),
		filepath.Join(modelDir, "model.onnx"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			tok := filepath.Join(modelDir, "tokenizer.json")
			if _, err := os.Stat(tok); err != nil {
				return "", "", err
			}
			return candidate, tok, nil
		}
	}
	matches, _ := filepath.Glob(filepath.Join(modelDir, "*.onnx"))
	if len(matches) == 0 {
		return "", "", fmt.Errorf("onnx model not found in %s", modelDir)
	}
	tok := filepath.Join(modelDir, "tokenizer.json")
	if _, err := os.Stat(tok); err != nil {
		return "", "", err
	}
	return matches[0], tok, nil
}

func DiscoverRuntimePath() string {
	path, err := ResolveRuntimeLibrary()
	if err != nil {
		return ""
	}
	return path
}

func (e *Engine) Close() error {
	if e == nil {
		return nil
	}
	if e.session != nil {
		_ = e.session.Destroy()
	}
	return ReleaseEnvironment()
}

func (e *Engine) Ready() bool {
	return e != nil && e.session != nil && e.tokenizer != nil
}

func (e *Engine) Encode(ctx context.Context, text string) ([]float32, error) {
	if !e.Ready() {
		return nil, errors.New("onnx not ready")
	}
	enc, err := e.encodeSingleSafe(text)
	if err != nil {
		return nil, err
	}
	ids := trimInts(enc.GetIds(), defaultMaxSeqLen)
	mask := trimInts(enc.GetAttentionMask(), defaultMaxSeqLen)
	types := trimInts(enc.GetTypeIds(), defaultMaxSeqLen)
	if len(ids) == 0 {
		return nil, errors.New("tokenizer returned empty ids")
	}
	if len(mask) == 0 {
		mask = make([]int, len(ids))
		for i := range mask {
			mask[i] = 1
		}
	}
	if len(types) == 0 {
		types = make([]int, len(ids))
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	shape := ort.NewShape(1, int64(len(ids)))
	inputs := make([]*ort.Tensor[int64], 0, len(e.inputNames))
	for _, name := range e.inputNames {
		switch strings.ToLower(name) {
		case "input_ids":
			v, err := ort.NewTensor(shape, intsToInt64(ids))
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, v)
		case "attention_mask":
			v, err := ort.NewTensor(shape, intsToInt64(mask))
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, v)
		case "token_type_ids":
			v, err := ort.NewTensor(shape, intsToInt64(types))
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, v)
		default:
			v, err := ort.NewTensor(shape, intsToInt64(ids))
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, v)
		}
	}
	defer func() {
		for _, v := range inputs {
			if v != nil {
				_ = v.Destroy()
			}
		}
	}()

	outputs, err := e.makeOutputTensors(len(ids))
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, v := range outputs {
			if v != nil {
				_ = v.Destroy()
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if err := e.session.Run(inputs, outputs); err != nil {
		return nil, err
	}
	if len(outputs) == 0 || outputs[0] == nil {
		return nil, errors.New("onnx output is empty")
	}
	return poolAndNormalize(outputs[0].GetData(), outputs[0].GetShape(), mask), nil
}

func (e *Engine) makeOutputTensors(seqLen int) ([]*ort.Tensor[float32], error) {
	outputs := make([]*ort.Tensor[float32], 0, len(e.outputInfo))
	for _, info := range e.outputInfo {
		shape := cloneShape(info.Dimensions)
		for i, dim := range shape {
			if dim > 0 {
				continue
			}
			switch i {
			case 0:
				shape[i] = 1
			case 1:
				shape[i] = int64(seqLen)
			default:
				shape[i] = 1
			}
		}
		if len(shape) == 0 {
			shape = ort.NewShape(1)
		}
		v, err := ort.NewEmptyTensor[float32](shape)
		if err != nil {
			for _, created := range outputs {
				_ = created.Destroy()
			}
			return nil, err
		}
		outputs = append(outputs, v)
	}
	return outputs, nil
}

func cloneShape(shape ort.Shape) ort.Shape {
	if len(shape) == 0 {
		return nil
	}
	out := make(ort.Shape, len(shape))
	copy(out, shape)
	return out
}

func (e *Engine) encodeSingleSafe(text string) (enc *tokenizer.Encoding, err error) {
	candidates := []string{sanitizeText(text), conservativeText(text)}
	seen := map[string]struct{}{}
	var lastErr error
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		panicErr := func() (panicErr error) {
			defer func() {
				if r := recover(); r != nil {
					panicErr = fmt.Errorf("tokenizer panic: %v", r)
				}
			}()
			enc, err = e.tokenizer.EncodeSingle(candidate, true)
			return nil
		}()
		if panicErr != nil {
			lastErr = panicErr
			continue
		}
		if err == nil && enc != nil {
			return enc, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("tokenizer returned no encoding")
	}
	return nil, lastErr
}

func sanitizeText(text string) string {
	text = strings.ToValidUTF8(strings.TrimSpace(text), " ")
	text = strings.Map(func(r rune) rune {
		switch {
		case r == 0:
			return -1
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff':
			return ' '
		case unicode.IsControl(r) && r != '\n' && r != '\t' && r != ' ':
			return ' '
		default:
			return r
		}
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > 320 {
		text = strings.TrimSpace(string(runes[:320]))
	}
	return text
}

func conservativeText(text string) string {
	text = sanitizeText(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
			return r
		}
		switch r {
		case '.', ',', ':', ';', '-', '_', '#', '/', '(', ')', '?', '!', '+':
			return r
		default:
			return ' '
		}
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

func trimInts(values []int, limit int) []int {
	if limit <= 0 || len(values) <= limit {
		return append([]int{}, values...)
	}
	return append([]int{}, values[:limit]...)
}

func intsToInt64(values []int) []int64 {
	out := make([]int64, len(values))
	for i, v := range values {
		out[i] = int64(v)
	}
	return out
}

func poolAndNormalize(data []float32, shape []int64, mask []int) []float32 {
	if len(shape) == 2 {
		return l2Normalize(append([]float32{}, data...))
	}
	if len(shape) < 3 || len(data) == 0 {
		return l2Normalize(append([]float32{}, data...))
	}
	seq := int(shape[1])
	hidden := int(shape[2])
	if seq <= 0 || hidden <= 0 {
		return l2Normalize(append([]float32{}, data...))
	}
	vec := make([]float32, hidden)
	weight := float32(0)
	for i := 0; i < seq; i++ {
		w := float32(1)
		if i < len(mask) {
			w = float32(mask[i])
		}
		if w <= 0 {
			continue
		}
		base := i * hidden
		for j := 0; j < hidden; j++ {
			vec[j] += data[base+j] * w
		}
		weight += w
	}
	if weight > 0 {
		for i := range vec {
			vec[i] /= weight
		}
	}
	return l2Normalize(vec)
}

func l2Normalize(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return vec
	}
	norm := float32(1.0 / sumSqrt(sum))
	for i := range vec {
		vec[i] *= norm
	}
	return vec
}

func sumSqrt(v float64) float64 {
	if v <= 0 {
		return 1
	}
	x := v
	for i := 0; i < 8; i++ {
		x = 0.5 * (x + v/x)
	}
	return x
}
