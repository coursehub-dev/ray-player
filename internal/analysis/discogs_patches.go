package analysis

func MakeDiscogsPatches(
	mel []float32,
	frameCount int,
) ([]float32, int) {
	if frameCount < discogsPatchFrames {
		return nil, 0
	}
	if len(mel) != frameCount*discogsMelBands {
		return nil, 0
	}

	const patchHopFrames = 62

	count := 1 + (frameCount-discogsPatchFrames)/patchHopFrames

	patchSize := discogsPatchFrames * discogsMelBands
	result := make([]float32, count*patchSize)

	for patchIndex := 0; patchIndex < count; patchIndex++ {
		startFrame := patchIndex * patchHopFrames
		sourceStart := startFrame * discogsMelBands
		sourceEnd := sourceStart + patchSize

		destinationStart := patchIndex * patchSize
		copy(result[destinationStart:destinationStart+patchSize], mel[sourceStart:sourceEnd])
	}

	return result, count
}
