package analysis

func addFloatVector(
	target []float32,
	source []float32,
) {
	limit := len(target)
	if len(source) < limit {
		limit = len(source)
	}
	for index := 0; index < limit; index++ {
		target[index] += source[index]
	}
}

func scaleFloatVector(
	values []float32,
	scale float32,
) {
	for index := range values {
		values[index] *= scale
	}
}
