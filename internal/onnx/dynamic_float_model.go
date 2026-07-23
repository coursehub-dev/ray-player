package onnx

import (
	"errors"
	"fmt"
	"math"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

type DynamicFloatModel struct {
	mu sync.Mutex

	session *ort.DynamicAdvancedSession
	spec    ModelSpec
	closed  bool
}

func NewDynamicFloatModel(
	spec ModelSpec,
) (*DynamicFloatModel, error) {
	if err := AcquireEnvironment(); err != nil {
		return nil, err
	}

	info, err := InspectModel(spec.Path)
	if err != nil {
		_ = ReleaseEnvironment()
		return nil, err
	}

	if _, ok := info.Input(spec.InputName); !ok {
		_ = ReleaseEnvironment()
		return nil, fmt.Errorf(
			"model %s input %q not found: %s",
			spec.Name,
			spec.InputName,
			info.String(),
		)
	}
	if _, ok := info.Output(spec.OutputName); !ok {
		_ = ReleaseEnvironment()
		return nil, fmt.Errorf(
			"model %s output %q not found: %s",
			spec.Name,
			spec.OutputName,
			info.String(),
		)
	}

	session, err := ort.NewDynamicAdvancedSession(
		spec.Path,
		[]string{spec.InputName},
		[]string{spec.OutputName},
		nil,
	)
	if err != nil {
		_ = ReleaseEnvironment()
		return nil, fmt.Errorf(
			"create dynamic ONNX session %s: %w",
			spec.Name,
			err,
		)
	}

	return &DynamicFloatModel{
		session: session,
		spec:    spec,
	}, nil
}

func (m *DynamicFloatModel) Run(
	input []float32,
	inputShape ort.Shape,
) ([]float32, ort.Shape, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, nil, errors.New(
			"dynamic ONNX model is closed",
		)
	}

	inputTensor, err := ort.NewTensor(
		inputShape,
		input,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create %s input tensor: %w",
			m.spec.Name,
			err,
		)
	}
	defer inputTensor.Destroy()

	outputs := []ort.Value{nil}
	err = m.session.Run(
		[]ort.Value{inputTensor},
		outputs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"run dynamic ONNX model %s: %w",
			m.spec.Name,
			err,
		)
	}
	if outputs[0] == nil {
		return nil, nil, fmt.Errorf(
			"model %s returned nil output",
			m.spec.Name,
		)
	}
	defer outputs[0].Destroy()

	tensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, nil, fmt.Errorf(
			"model %s returned non-float output",
			m.spec.Name,
		)
	}

	data := append(
		[]float32(nil),
		tensor.GetData()...,
	)
	if !allFiniteDynamic(data) {
		return nil, nil, fmt.Errorf(
			"model %s returned NaN/Inf",
			m.spec.Name,
		)
	}

	return data, tensor.GetShape(), nil
}

func (m *DynamicFloatModel) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	err := m.session.Destroy()
	releaseErr := ReleaseEnvironment()
	if err != nil {
		return err
	}
	return releaseErr
}

func allFiniteDynamic(values []float32) bool {
	for _, value := range values {
		if math.IsNaN(float64(value)) ||
			math.IsInf(float64(value), 0) {
			return false
		}
	}
	return true
}
