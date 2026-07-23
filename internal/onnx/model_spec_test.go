package onnx

import (
	"testing"

	ort "github.com/yalue/onnxruntime_go"
)

func TestValidateModelSpecRejectsTruncatedEmbedding(
	t *testing.T,
) {
	info := ModelInfo{
		Inputs: []TensorInfo{
			{
				Name:       "audio",
				Dimensions: ort.NewShape(1, 16000),
			},
		},
		Outputs: []TensorInfo{
			{
				Name:       "embedding",
				Dimensions: ort.NewShape(1, 16),
			},
		},
	}

	err := ValidateModelSpec(
		ModelSpec{
			Name:               "embedding",
			InputName:          "audio",
			OutputName:         "embedding",
			InputShape:         ort.NewShape(1, 16000),
			OutputShape:        ort.NewShape(1, 1280),
			ExpectedOutputSize: 1280,
		},
		info,
	)
	if err == nil {
		t.Fatal(
			"expected truncated embedding error",
		)
	}
}
