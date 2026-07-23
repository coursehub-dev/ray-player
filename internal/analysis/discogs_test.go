package analysis

import "testing"

func TestDiscogsAggregationKeepsOutputDimensions(
	t *testing.T,
) {
	const patches = 3

	predictions := make(
		[]float32,
		patches*discogsClassCount,
	)
	embeddings := make(
		[]float32,
		patches*discogsEmbeddingSize,
	)

	for patch := 0; patch < patches; patch++ {
		for index := 0; index < discogsClassCount; index++ {
			predictions[patch*discogsClassCount+index] = float32(patch + 1)
		}
		for index := 0; index < discogsEmbeddingSize; index++ {
			embeddings[patch*discogsEmbeddingSize+index] = float32(patch + 1)
		}
	}

	patchPredictions := make([][]float32, patches)
	patchEmbeddings := make([][]float32, patches)
	meanPredictions := make([]float32, discogsClassCount)
	meanEmbedding := make([]float32, discogsEmbeddingSize)

	for p := 0; p < patches; p++ {
		predStart := p * discogsClassCount
		embStart := p * discogsEmbeddingSize

		patchPredictions[p] = make([]float32, discogsClassCount)
		copy(patchPredictions[p], predictions[predStart:predStart+discogsClassCount])

		patchEmbeddings[p] = make([]float32, discogsEmbeddingSize)
		copy(patchEmbeddings[p], embeddings[embStart:embStart+discogsEmbeddingSize])

		for i := 0; i < discogsClassCount; i++ {
			meanPredictions[i] += patchPredictions[p][i]
		}
		for i := 0; i < discogsEmbeddingSize; i++ {
			meanEmbedding[i] += patchEmbeddings[p][i]
		}
	}

	inv := float32(1) / float32(patches)
	for i := range meanPredictions {
		meanPredictions[i] *= inv
	}
	for i := range meanEmbedding {
		meanEmbedding[i] *= inv
	}

	if len(meanPredictions) != 400 {
		t.Fatalf(
			"predictions=%d want=400",
			len(meanPredictions),
		)
	}
	if len(meanEmbedding) != 1280 {
		t.Fatalf(
			"embedding=%d want=1280",
			len(meanEmbedding),
		)
	}
}

func TestMakeDiscogsPatchesProducesCorrectCount(t *testing.T) {
	const (
		frameCount = 300
		melBands   = 96
	)
	mel := make([]float32, frameCount*melBands)
	for i := range mel {
		mel[i] = float32(i) / float32(len(mel))
	}

	patches, count := MakeDiscogsPatches(mel, frameCount)
	if count <= 0 {
		t.Fatalf("expected positive patch count, got %d", count)
	}
	expectedSize := count * discogsPatchFrames * discogsMelBands
	if len(patches) != expectedSize {
		t.Fatalf("patches len=%d want=%d", len(patches), expectedSize)
	}
}
