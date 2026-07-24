package onnx

import (
	"testing"

	ort "github.com/yalue/onnxruntime_go"
)

func TestValidateDiscogsContractLegacyNames(t *testing.T) {
	info := ModelInfo{
		Inputs: []TensorInfo{
			{
				Name:       "serving_default_melspectrogram",
				Dimensions: ort.NewShape(-1, 128, 96),
			},
		},
		Outputs: []TensorInfo{
			{
				Name:       "PartitionedCall:0",
				Dimensions: ort.NewShape(-1, 400),
			},
			{
				Name:       "PartitionedCall:1",
				Dimensions: ort.NewShape(-1, 1280),
			},
		},
	}

	names, err := ValidateDiscogsContract(info)
	if err != nil {
		t.Fatalf("ValidateDiscogsContract: %v", err)
	}
	if names.Input != "serving_default_melspectrogram" {
		t.Fatalf("input=%q", names.Input)
	}
	if names.Predictions != "PartitionedCall:0" {
		t.Fatalf("predictions=%q", names.Predictions)
	}
	if names.Embedding != "PartitionedCall:1" {
		t.Fatalf("embedding=%q", names.Embedding)
	}
}

func TestValidateDiscogsContractCleanNames(t *testing.T) {
	info := ModelInfo{
		Inputs: []TensorInfo{
			{
				Name:       "melspectrogram",
				Dimensions: ort.NewShape(-1, 128, 96),
			},
		},
		Outputs: []TensorInfo{
			{
				Name:       "activations",
				Dimensions: ort.NewShape(-1, 400),
			},
			{
				Name:       "embeddings",
				Dimensions: ort.NewShape(-1, 1280),
			},
		},
	}

	names, err := ValidateDiscogsContract(info)
	if err != nil {
		t.Fatalf("ValidateDiscogsContract: %v", err)
	}
	if names.Input != "melspectrogram" {
		t.Fatalf("input=%q", names.Input)
	}
	if names.Predictions != "activations" {
		t.Fatalf("predictions=%q", names.Predictions)
	}
	if names.Embedding != "embeddings" {
		t.Fatalf("embedding=%q", names.Embedding)
	}
}

func TestValidateDiscogsContractPrefersLegacyWhenBothPresent(t *testing.T) {
	info := ModelInfo{
		Inputs: []TensorInfo{
			{
				Name:       "serving_default_melspectrogram",
				Dimensions: ort.NewShape(-1, 128, 96),
			},
			{
				Name:       "melspectrogram",
				Dimensions: ort.NewShape(-1, 128, 96),
			},
		},
		Outputs: []TensorInfo{
			{
				Name:       "PartitionedCall:0",
				Dimensions: ort.NewShape(-1, 400),
			},
			{
				Name:       "activations",
				Dimensions: ort.NewShape(-1, 400),
			},
			{
				Name:       "PartitionedCall:1",
				Dimensions: ort.NewShape(-1, 1280),
			},
			{
				Name:       "embeddings",
				Dimensions: ort.NewShape(-1, 1280),
			},
		},
	}

	names, err := ValidateDiscogsContract(info)
	if err != nil {
		t.Fatalf("ValidateDiscogsContract: %v", err)
	}
	if names.Input != "serving_default_melspectrogram" ||
		names.Predictions != "PartitionedCall:0" ||
		names.Embedding != "PartitionedCall:1" {
		t.Fatalf("unexpected names: %+v", names)
	}
}

func TestValidateDiscogsContractRejectsBadShapes(t *testing.T) {
	info := ModelInfo{
		Inputs: []TensorInfo{
			{
				Name:       "melspectrogram",
				Dimensions: ort.NewShape(-1, 128),
			},
		},
		Outputs: []TensorInfo{
			{
				Name:       "activations",
				Dimensions: ort.NewShape(-1, 400),
			},
			{
				Name:       "embeddings",
				Dimensions: ort.NewShape(-1, 1280),
			},
		},
	}
	if _, err := ValidateDiscogsContract(info); err == nil {
		t.Fatal("expected rank error")
	}
}
