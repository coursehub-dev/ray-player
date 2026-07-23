package onnx

import (
	"errors"
	"fmt"
	"math"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

type FloatModel struct {
	mu sync.Mutex

	session *ort.AdvancedSession
	input   *ort.Tensor[float32]
	output  *ort.Tensor[float32]

	inputData  []float32
	outputData []float32
	closed     bool

	spec ModelSpec
}

type FloatModelConfig struct {
	Spec ModelSpec
}

func NewFloatModel(
	config FloatModelConfig,
) (*FloatModel, error) {
	spec := config.Spec
	if spec.Path == "" {
		return nil, errors.New("ONNX model path is empty")
	}

	if err := AcquireEnvironment(); err != nil {
		return nil, err
	}

	cleanupEnvironment := true
	defer func() {
		if cleanupEnvironment {
			_ = ReleaseEnvironment()
		}
	}()

	info, err := InspectModel(spec.Path)
	if err != nil {
		return nil, err
	}
	if err := ValidateModelSpec(spec, info); err != nil {
		return nil, err
	}

	inputInfo, _ := info.Input(spec.InputName)
	outputInfo, _ := info.Output(spec.OutputName)

	inputShape, err := resolvedShape(
		inputInfo.Dimensions,
		spec.InputShape,
	)
	if err != nil {
		return nil, err
	}
	outputShape, err := resolvedShape(
		outputInfo.Dimensions,
		spec.OutputShape,
	)
	if err != nil {
		return nil, err
	}

	inputSize := inputShape.FlattenedSize()
	outputSize := outputShape.FlattenedSize()
	if inputSize <= 0 || outputSize <= 0 {
		return nil, errors.New(
			"invalid ONNX tensor shape",
		)
	}

	inputData := make([]float32, inputSize)
	outputData := make([]float32, outputSize)

	input, err := ort.NewTensor(
		inputShape,
		inputData,
	)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}

	output, err := ort.NewTensor(
		outputShape,
		outputData,
	)
	if err != nil {
		_ = input.Destroy()
		return nil, fmt.Errorf("create output tensor: %w", err)
	}

	session, err := ort.NewAdvancedSession(
		spec.Path,
		[]string{spec.InputName},
		[]string{spec.OutputName},
		[]ort.Value{input},
		[]ort.Value{output},
		nil,
	)
	if err != nil {
		_ = output.Destroy()
		_ = input.Destroy()
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	cleanupEnvironment = false
	return &FloatModel{
		session:    session,
		input:      input,
		output:     output,
		inputData:  inputData,
		outputData: outputData,
		spec:       spec,
	}, nil
}

func (m *FloatModel) Run(
	input []float32,
) ([]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, errors.New("ONNX model is closed")
	}
	if len(input) != len(m.inputData) {
		return nil, fmt.Errorf(
			"invalid ONNX input size: got %d, want %d",
			len(input),
			len(m.inputData),
		)
	}

	if m.spec.ExpectedOutputSize > 0 &&
		len(m.outputData) !=
			m.spec.ExpectedOutputSize {
		return nil, fmt.Errorf(
			"ONNX model %s returned invalid output size: got=%d want=%d",
			m.spec.Name,
			len(m.outputData),
			m.spec.ExpectedOutputSize,
		)
	}

	copy(m.inputData, input)
	m.output.ZeroContents()

	if err := m.session.Run(); err != nil {
		return nil, fmt.Errorf(
			"run ONNX session %s: %w",
			m.spec.Name,
			err,
		)
	}

	if !allFinite(m.outputData) {
		return nil, fmt.Errorf(
			"ONNX model %s returned NaN/Inf",
			m.spec.Name,
		)
	}

	result := make([]float32, len(m.outputData))
	copy(result, m.outputData)
	return result, nil
}

func (m *FloatModel) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	_ = m.session.Destroy()
	_ = m.output.Destroy()
	_ = m.input.Destroy()
	return ReleaseEnvironment()
}

func allFinite(values []float32) bool {
	for _, value := range values {
		if math.IsNaN(float64(value)) ||
			math.IsInf(float64(value), 0) {
			return false
		}
	}
	return true
}
