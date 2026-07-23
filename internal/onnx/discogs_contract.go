package onnx

import "fmt"

const (
	discogsContractInputName = "serving_default_melspectrogram"

	discogsContractPredictionsOutputName = "PartitionedCall:0"
	discogsContractEmbeddingOutputName   = "PartitionedCall:1"

	discogsContractClassCount    = 400
	discogsContractEmbeddingSize = 1280
)

func ValidateDiscogsContract(info ModelInfo) error {
	input, ok := info.Input(discogsContractInputName)
	if !ok {
		return fmt.Errorf(
			"Discogs model input %q not found: %s",
			discogsContractInputName,
			info.String(),
		)
	}
	if len(input.Dimensions) != 3 {
		return fmt.Errorf(
			"Discogs input rank=%d want=3 shape=%v",
			len(input.Dimensions),
			input.Dimensions,
		)
	}

	predictions, ok := info.Output(discogsContractPredictionsOutputName)
	if !ok {
		return fmt.Errorf(
			"Discogs predictions output %q not found: %s",
			discogsContractPredictionsOutputName,
			info.String(),
		)
	}

	embedding, ok := info.Output(discogsContractEmbeddingOutputName)
	if !ok {
		return fmt.Errorf(
			"Discogs embedding output %q not found: %s",
			discogsContractEmbeddingOutputName,
			info.String(),
		)
	}

	if lastPositiveDimension(predictions.Dimensions) != discogsContractClassCount {
		return fmt.Errorf(
			"Discogs predictions shape=%v, expected last dimension=%d",
			predictions.Dimensions,
			discogsContractClassCount,
		)
	}

	if lastPositiveDimension(embedding.Dimensions) != discogsContractEmbeddingSize {
		return fmt.Errorf(
			"Discogs embedding shape=%v, expected last dimension=%d",
			embedding.Dimensions,
			discogsContractEmbeddingSize,
		)
	}

	return nil
}

func lastPositiveDimension(shape []int64) int {
	for index := len(shape) - 1; index >= 0; index-- {
		if shape[index] > 0 {
			return int(shape[index])
		}
	}
	return 0
}
