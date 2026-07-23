package onnx

import (
	"fmt"
	"math"
)

type GenreOutputQuality struct {
	PatchCount       int
	ClassCount       int
	ExactZeroCount   int
	ExactOneCount    int
	SaturatedPatches int
	ActivePatches    int
	MeanMaxScore     float64

	DominantClassIndex     int
	DominantNearOnePatches int
	DominantPatchRatio     float64
	NearOnePatchCount      int
	NearOnePatchRatio      float64
}

func inspectGenreOutput(
	values []float32,
	patchCount int,
	classCount int,
) (GenreOutputQuality, error) {
	quality := GenreOutputQuality{
		PatchCount: patchCount,
		ClassCount: classCount,
	}

	if patchCount <= 0 || classCount <= 0 {
		return quality, fmt.Errorf(
			"invalid genre dimensions patches=%d classes=%d",
			patchCount,
			classCount,
		)
	}
	if len(values) != patchCount*classCount {
		return quality, fmt.Errorf(
			"genre output length=%d want=%d",
			len(values),
			patchCount*classCount,
		)
	}

	nearOneByClass := make(
		[]int,
		classCount,
	)

	var maxSum float64
	for patch := 0; patch < patchCount; patch++ {
		start := patch * classCount
		end := start + classCount

		maxScore := float32(0)
		maxIndex := -1
		onesInPatch := 0

		for classIndex, value := range values[start:end] {
			if math.IsNaN(float64(value)) ||
				math.IsInf(float64(value), 0) {
				return quality, fmt.Errorf(
					"genre output contains NaN/Inf",
				)
			}

			if value == 0 {
				quality.ExactZeroCount++
			}
			if value >= 0.9999 {
				quality.ExactOneCount++
				onesInPatch++
			}
			if value > maxScore {
				maxScore = value
				maxIndex = classIndex
			}
		}

		if maxScore >= 0.05 {
			quality.ActivePatches++
		}

		if maxScore >= 0.9999 {
			quality.NearOnePatchCount++
			if maxIndex >= 0 {
				nearOneByClass[maxIndex]++
			}
		}

		if onesInPatch >= 2 {
			quality.SaturatedPatches++
		}
		maxSum += float64(maxScore)
	}

	quality.MeanMaxScore =
		maxSum / float64(patchCount)

	quality.DominantClassIndex = -1
	for classIndex, count := range nearOneByClass {
		if count > quality.DominantNearOnePatches {
			quality.DominantClassIndex = classIndex
			quality.DominantNearOnePatches = count
		}
	}

	quality.NearOnePatchRatio =
		float64(quality.NearOnePatchCount) /
			float64(patchCount)

	quality.DominantPatchRatio =
		float64(quality.DominantNearOnePatches) /
			float64(patchCount)

	return quality, nil
}

func (q GenreOutputQuality) Suspicious() bool {
	if q.PatchCount == 0 {
		return true
	}

	saturatedRatio :=
		float64(q.SaturatedPatches) /
			float64(q.PatchCount)

	activeRatio :=
		float64(q.ActivePatches) /
			float64(q.PatchCount)

	return saturatedRatio > 0.20 ||
		activeRatio < 0.10 ||
		q.MeanMaxScore > 0.98 ||
		q.NearOnePatchRatio > 0.20 ||
		q.DominantPatchRatio > 0.15
}
