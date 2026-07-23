package onnx

import (
	"fmt"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

type TensorInfo struct {
	Name       string
	Dimensions ort.Shape
	DataType   ort.TensorElementDataType
}

type ModelInfo struct {
	Inputs  []TensorInfo
	Outputs []TensorInfo
}

func InspectModel(path string) (ModelInfo, error) {
	inputs, outputs, err := ort.GetInputOutputInfo(path)
	if err != nil {
		return ModelInfo{}, fmt.Errorf(
			"inspect ONNX model %q: %w",
			path,
			err,
		)
	}

	info := ModelInfo{
		Inputs:  make([]TensorInfo, 0, len(inputs)),
		Outputs: make([]TensorInfo, 0, len(outputs)),
	}

	for _, input := range inputs {
		info.Inputs = append(info.Inputs, TensorInfo{
			Name:       input.Name,
			Dimensions: input.Dimensions.Clone(),
			DataType:   input.DataType,
		})
	}

	for _, output := range outputs {
		info.Outputs = append(info.Outputs, TensorInfo{
			Name:       output.Name,
			Dimensions: output.Dimensions.Clone(),
			DataType:   output.DataType,
		})
	}

	return info, nil
}

func (m ModelInfo) Input(name string) (
	TensorInfo,
	bool,
) {
	for _, item := range m.Inputs {
		if item.Name == name {
			return item, true
		}
	}
	return TensorInfo{}, false
}

func (m ModelInfo) Output(name string) (
	TensorInfo,
	bool,
) {
	for _, item := range m.Outputs {
		if item.Name == name {
			return item, true
		}
	}
	return TensorInfo{}, false
}

func (m ModelInfo) String() string {
	var builder strings.Builder

	builder.WriteString("inputs=[")
	for index, input := range m.Inputs {
		if index > 0 {
			builder.WriteString(", ")
		}
		fmt.Fprintf(
			&builder,
			"%s:%v:%v",
			input.Name,
			[]int64(input.Dimensions),
			input.DataType,
		)
	}
	builder.WriteString("] outputs=[")

	for index, output := range m.Outputs {
		if index > 0 {
			builder.WriteString(", ")
		}
		fmt.Fprintf(
			&builder,
			"%s:%v:%v",
			output.Name,
			[]int64(output.Dimensions),
			output.DataType,
		)
	}
	builder.WriteString("]")

	return builder.String()
}

func resolvedShape(
	modelShape ort.Shape,
	fallback ort.Shape,
) (ort.Shape, error) {
	if len(modelShape) == 0 {
		return nil, fmt.Errorf("empty tensor shape")
	}

	result := modelShape.Clone()
	for index, dimension := range result {
		if dimension > 0 {
			continue
		}
		if index >= len(fallback) ||
			fallback[index] <= 0 {
			return nil, fmt.Errorf(
				"dynamic dimension %d cannot be resolved: model=%v fallback=%v",
				index,
				modelShape,
				fallback,
			)
		}
		result[index] = fallback[index]
	}

	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}
