package onnx

import (
	"fmt"

	ort "github.com/yalue/onnxruntime_go"
)

type ModelKind string

const (
	ModelEmbedding ModelKind = "embedding"
	ModelGenre     ModelKind = "genre"
	ModelAttribute ModelKind = "attribute"
)

type ModelSpec struct {
	Kind ModelKind
	Name string
	Path string

	InputName  string
	OutputName string

	InputShape  ort.Shape
	OutputShape ort.Shape

	ExpectedOutputSize int
}

func ValidateModelSpec(
	spec ModelSpec,
	info ModelInfo,
) error {
	input, ok := info.Input(spec.InputName)
	if !ok {
		return fmt.Errorf(
			"model %s has no input %q; %s",
			spec.Name,
			spec.InputName,
			info.String(),
		)
	}

	output, ok := info.Output(spec.OutputName)
	if !ok {
		return fmt.Errorf(
			"model %s has no output %q; %s",
			spec.Name,
			spec.OutputName,
			info.String(),
		)
	}

	if _, err := resolvedShape(
		input.Dimensions,
		spec.InputShape,
	); err != nil {
		return fmt.Errorf(
			"model %s invalid input shape: %w",
			spec.Name,
			err,
		)
	}

	outputShape, err := resolvedShape(
		output.Dimensions,
		spec.OutputShape,
	)
	if err != nil {
		return fmt.Errorf(
			"model %s invalid output shape: %w",
			spec.Name,
			err,
		)
	}

	if spec.ExpectedOutputSize > 0 &&
		int(outputShape.FlattenedSize()) !=
			spec.ExpectedOutputSize {
		return fmt.Errorf(
			"model %s output size mismatch: got=%d want=%d shape=%v output=%q",
			spec.Name,
			outputShape.FlattenedSize(),
			spec.ExpectedOutputSize,
			outputShape,
			spec.OutputName,
		)
	}

	return nil
}
