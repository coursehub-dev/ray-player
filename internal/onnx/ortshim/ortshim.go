package ortshim

import (
	"context"
	"fmt"

	ort "github.com/yalue/onnxruntime_go"
)

type LoggingLevel int

const (
	LoggingLevelWarning = 2
)

type SessionOptions struct {
	IntraOpNumThreads int
}

type Runtime struct{}

type Env struct{}

type Session struct {
	session     *ort.DynamicSession[float32, float32]
	inputNames  []string
	outputNames []string
	outputDims  map[string][]int64
}

type Value struct {
	tensor *ort.Tensor[float32]
}

func NewRuntime(_ string, _ int) (*Runtime, error) {
	return &Runtime{}, nil
}

func (r *Runtime) NewEnv(_ string, _ LoggingLevel) (*Env, error) { return &Env{}, nil }

func (r *Runtime) NewSession(_ *Env, modelPath string, _ *SessionOptions) (*Session, error) {
	inputs, outputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, err
	}
	inputNames := make([]string, 0, len(inputs))
	for _, item := range inputs {
		inputNames = append(inputNames, item.Name)
	}
	outputNames := make([]string, 0, len(outputs))
	outputDims := make(map[string][]int64, len(outputs))
	for _, item := range outputs {
		outputNames = append(outputNames, item.Name)
		outputDims[item.Name] = append([]int64{}, item.Dimensions...)
	}
	s, err := ort.NewDynamicSession[float32, float32](modelPath, inputNames, outputNames)
	if err != nil {
		return nil, err
	}
	return &Session{session: s, inputNames: inputNames, outputNames: outputNames, outputDims: outputDims}, nil
}

func (r *Runtime) Close() error { return nil }

func (e *Env) Close() error { return nil }

func (s *Session) InputNames() []string  { return append([]string{}, s.inputNames...) }
func (s *Session) OutputNames() []string { return append([]string{}, s.outputNames...) }
func (s *Session) Close() error          { return s.session.Destroy() }

func NewTensorValue(_ *Runtime, data []float32, shape []int64) (*Value, error) {
	tensor, err := ort.NewTensor(ort.NewShape(shape...), data)
	if err != nil {
		return nil, err
	}
	return &Value{tensor: tensor}, nil
}

func (v *Value) Close() error {
	if v == nil || v.tensor == nil {
		return nil
	}
	err := v.tensor.Destroy()
	v.tensor = nil
	return err
}

func GetTensorData[T any](v *Value) ([]T, []int64, error) {
	if v == nil || v.tensor == nil {
		return nil, nil, fmt.Errorf("tensor is nil")
	}
	data := v.tensor.GetData()
	shape := []int64(v.tensor.GetShape())
	result := make([]T, len(data))
	for i, item := range data {
		casted, ok := any(item).(T)
		if !ok {
			return nil, nil, fmt.Errorf("unsupported tensor cast")
		}
		result[i] = casted
	}
	return result, shape, nil
}

func (s *Session) Run(ctx context.Context, inputs map[string]*Value) (map[string]*Value, error) {
	in := make([]*ort.Tensor[float32], 0, len(s.inputNames))
	for _, name := range s.inputNames {
		value := inputs[name]
		if value == nil || value.tensor == nil {
			return nil, fmt.Errorf("missing input tensor: %s", name)
		}
		in = append(in, value.tensor)
	}
	batch := int64(1)
	if len(in) > 0 {
		shape := in[0].GetShape()
		if len(shape) > 0 && shape[0] > 0 {
			batch = shape[0]
		}
	}
	out := make([]*ort.Tensor[float32], 0, len(s.outputNames))
	result := make(map[string]*Value, len(s.outputNames))
	for _, name := range s.outputNames {
		dims := append([]int64{}, s.outputDims[name]...)
		if len(dims) == 0 {
			dims = []int64{1}
		}
		for i, dim := range dims {
			if dim > 0 {
				continue
			}
			if i == 0 {
				dims[i] = batch
			} else {
				dims[i] = 1
			}
		}
		tensor, err := ort.NewEmptyTensor[float32](ort.NewShape(dims...))
		if err != nil {
			for _, created := range out {
				_ = created.Destroy()
			}
			return nil, err
		}
		out = append(out, tensor)
		result[name] = &Value{tensor: tensor}
	}
	select {
	case <-ctx.Done():
		for _, created := range out {
			_ = created.Destroy()
		}
		return nil, ctx.Err()
	default:
	}
	if err := s.session.Run(in, out); err != nil {
		for _, created := range out {
			_ = created.Destroy()
		}
		return nil, err
	}
	return result, nil
}
