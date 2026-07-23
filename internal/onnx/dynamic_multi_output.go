package onnx

import (
	"errors"
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

type DynamicMultiOutputFloatModel struct {
	mu sync.Mutex

	name string

	session *ort.DynamicAdvancedSession

	inputName   string
	outputNames []string

	closed bool
}

func NewDynamicMultiOutputFloatModel(
	name string,
	modelPath string,
	inputName string,
	outputNames []string,
) (*DynamicMultiOutputFloatModel, error) {
	if err := AcquireEnvironment(); err != nil {
		return nil, err
	}

	if modelPath == "" {
		_ = ReleaseEnvironment()
		return nil, errors.New("ONNX model path is empty")
	}
	if inputName == "" {
		_ = ReleaseEnvironment()
		return nil, errors.New("ONNX input name is empty")
	}
	if len(outputNames) == 0 {
		_ = ReleaseEnvironment()
		return nil, errors.New("ONNX outputs are empty")
	}

	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{inputName},
		append([]string(nil), outputNames...),
		nil,
	)
	if err != nil {
		_ = ReleaseEnvironment()
		return nil, fmt.Errorf(
			"create ONNX dynamic session %s: %w",
			name,
			err,
		)
	}

	return &DynamicMultiOutputFloatModel{
		name:        name,
		session:     session,
		inputName:   inputName,
		outputNames: append([]string(nil), outputNames...),
	}, nil
}

func (m *DynamicMultiOutputFloatModel) Run(
	input []float32,
	inputShape []int64,
	outputShapes [][]int64,
) ([][]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, errors.New("ONNX model is closed")
	}
	if len(outputShapes) != len(m.outputNames) {
		return nil, fmt.Errorf(
			"ONNX model %s output shape count=%d want=%d",
			m.name,
			len(outputShapes),
			len(m.outputNames),
		)
	}

	flatSize := int64(1)
	for _, d := range inputShape {
		flatSize *= d
	}
	if int(flatSize) != len(input) {
		return nil, fmt.Errorf(
			"ONNX model %s input size=%d shape=%v requires=%d",
			m.name,
			len(input),
			inputShape,
			flatSize,
		)
	}

	inputTensor, err := ort.NewTensor(
		ort.Shape(inputShape),
		input,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create ONNX input tensor %s: %w",
			m.name,
			err,
		)
	}
	defer inputTensor.Destroy()

	outputValues := make(
		[]ort.Value,
		len(outputShapes),
	)
	outputTensors := make(
		[]*ort.Tensor[float32],
		len(outputShapes),
	)

	for index, shape := range outputShapes {
		tensor, tensorErr :=
			ort.NewEmptyTensor[float32](ort.Shape(shape))
		if tensorErr != nil {
			for cleanup := 0; cleanup < index; cleanup++ {
				_ = outputTensors[cleanup].Destroy()
			}
			return nil, fmt.Errorf(
				"create ONNX output %s[%d]: %w",
				m.name,
				index,
				tensorErr,
			)
		}
		outputTensors[index] = tensor
		outputValues[index] = tensor
	}

	defer func() {
		for _, tensor := range outputTensors {
			if tensor != nil {
				_ = tensor.Destroy()
			}
		}
	}()

	err = m.session.Run(
		[]ort.Value{inputTensor},
		outputValues,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"run ONNX model %s: %w",
			m.name,
			err,
		)
	}

	result := make([][]float32, len(outputTensors))
	for index, tensor := range outputTensors {
		result[index] = append(
			[]float32(nil),
			tensor.GetData()...,
		)
	}

	return result, nil
}

func (m *DynamicMultiOutputFloatModel) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	sessionErr := m.session.Destroy()
	environmentErr := ReleaseEnvironment()
	if sessionErr != nil {
		return sessionErr
	}
	return environmentErr
}
