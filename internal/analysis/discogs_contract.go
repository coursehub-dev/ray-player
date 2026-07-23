package analysis

import "fmt"

const (
	discogsInputName = "serving_default_melspectrogram"

	discogsPredictionsOutputName = "PartitionedCall:0"
	discogsEmbeddingOutputName   = "PartitionedCall:1"

	discogsPatchFrames = 128
	discogsMelBands    = 96

	discogsClassCount    = 400
	discogsEmbeddingSize = 1280
)

type DiscogsPatchResult struct {
	Predictions []float32
	Embedding   []float32
}

type DiscogsResult struct {
	PatchPredictions [][]float32
	PatchEmbeddings  [][]float32

	MeanPredictions []float32
	MeanEmbedding   []float32
}

func validateDiscogsPatchResult(
	result DiscogsPatchResult,
) error {
	if len(result.Predictions) != discogsClassCount {
		return fmt.Errorf(
			"discogs predictions size=%d want=%d",
			len(result.Predictions),
			discogsClassCount,
		)
	}
	if len(result.Embedding) != discogsEmbeddingSize {
		return fmt.Errorf(
			"discogs embedding size=%d want=%d",
			len(result.Embedding),
			discogsEmbeddingSize,
		)
	}
	return nil
}
