package onnx

import "fmt"

const (
	discogsContractClassCount    = 400
	discogsContractEmbeddingSize = 1280
)

// DiscogsIONames holds resolved tensor names for a Discogs EffNet ONNX export.
// Supports both legacy TF Serving names and cleaned export names.
type DiscogsIONames struct {
	Input       string
	Predictions string
	Embedding   string
}

var (
	discogsInputNameCandidates = []string{
		"serving_default_melspectrogram",
		"melspectrogram",
	}
	discogsPredictionsOutputCandidates = []string{
		"PartitionedCall:0",
		"activations",
	}
	discogsEmbeddingOutputCandidates = []string{
		"PartitionedCall:1",
		"embeddings",
	}
)

func ResolveDiscogsIONames(info ModelInfo) (DiscogsIONames, error) {
	input, ok := firstMatchingInput(info, discogsInputNameCandidates...)
	if !ok {
		return DiscogsIONames{}, fmt.Errorf(
			"Discogs model input not found (tried %v): %s",
			discogsInputNameCandidates,
			info.String(),
		)
	}
	predictions, ok := firstMatchingOutput(info, discogsPredictionsOutputCandidates...)
	if !ok {
		return DiscogsIONames{}, fmt.Errorf(
			"Discogs predictions output not found (tried %v): %s",
			discogsPredictionsOutputCandidates,
			info.String(),
		)
	}
	embedding, ok := firstMatchingOutput(info, discogsEmbeddingOutputCandidates...)
	if !ok {
		return DiscogsIONames{}, fmt.Errorf(
			"Discogs embedding output not found (tried %v): %s",
			discogsEmbeddingOutputCandidates,
			info.String(),
		)
	}
	return DiscogsIONames{
		Input:       input.Name,
		Predictions: predictions.Name,
		Embedding:   embedding.Name,
	}, nil
}

func ValidateDiscogsContract(info ModelInfo) (DiscogsIONames, error) {
	names, err := ResolveDiscogsIONames(info)
	if err != nil {
		return DiscogsIONames{}, err
	}

	input, _ := info.Input(names.Input)
	if len(input.Dimensions) != 3 {
		return DiscogsIONames{}, fmt.Errorf(
			"Discogs input rank=%d want=3 shape=%v",
			len(input.Dimensions),
			input.Dimensions,
		)
	}

	predictions, _ := info.Output(names.Predictions)
	embedding, _ := info.Output(names.Embedding)

	if lastPositiveDimension(predictions.Dimensions) != discogsContractClassCount {
		return DiscogsIONames{}, fmt.Errorf(
			"Discogs predictions shape=%v, expected last dimension=%d",
			predictions.Dimensions,
			discogsContractClassCount,
		)
	}

	if lastPositiveDimension(embedding.Dimensions) != discogsContractEmbeddingSize {
		return DiscogsIONames{}, fmt.Errorf(
			"Discogs embedding shape=%v, expected last dimension=%d",
			embedding.Dimensions,
			discogsContractEmbeddingSize,
		)
	}

	return names, nil
}

func firstMatchingInput(info ModelInfo, candidates ...string) (TensorInfo, bool) {
	for _, name := range candidates {
		if item, ok := info.Input(name); ok {
			return item, true
		}
	}
	return TensorInfo{}, false
}

func firstMatchingOutput(info ModelInfo, candidates ...string) (TensorInfo, bool) {
	for _, name := range candidates {
		if item, ok := info.Output(name); ok {
			return item, true
		}
	}
	return TensorInfo{}, false
}

func lastPositiveDimension(shape []int64) int {
	for index := len(shape) - 1; index >= 0; index-- {
		if shape[index] > 0 {
			return int(shape[index])
		}
	}
	return 0
}
