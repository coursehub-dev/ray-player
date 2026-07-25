const finiteNumber = (value, fallback) => {
	const number = Number(value);
	return Number.isFinite(number) ? number : fallback;
};

export const clampSliderValue = (value, min, max) => {
	const lower = finiteNumber(min, 0);
	const upper = Math.max(lower, finiteNumber(max, lower));
	return Math.max(lower, Math.min(upper, finiteNumber(value, lower)));
};

export const snapSliderValue = (value, min, max, step) => {
	const lower = finiteNumber(min, 0);
	const upper = Math.max(lower, finiteNumber(max, lower));
	const bounded = clampSliderValue(value, lower, upper);
	const increment = finiteNumber(step, 0);

	if (bounded <= lower || bounded >= upper || increment <= 0) {
		return bounded;
	}

	const snapped = lower + Math.round((bounded - lower) / increment) * increment;
	return Math.round(clampSliderValue(snapped, lower, upper) * 1e12) / 1e12;
};

export const sliderValueFromKey = ({ key, value, min, max, step }) => {
	const lower = finiteNumber(min, 0);
	const upper = Math.max(lower, finiteNumber(max, lower));
	const current = clampSliderValue(value, lower, upper);
	const unit = finiteNumber(step, 0) > 0 ? finiteNumber(step, 0) : (upper - lower) / 100;
	const page = unit > 0 ? unit * 10 : 0;

	switch (key) {
		case "ArrowLeft":
		case "ArrowDown":
			return snapSliderValue(current - unit, lower, upper, step);
		case "ArrowRight":
		case "ArrowUp":
			return snapSliderValue(current + unit, lower, upper, step);
		case "PageDown":
			return snapSliderValue(current - page, lower, upper, step);
		case "PageUp":
			return snapSliderValue(current + page, lower, upper, step);
		case "Home":
			return lower;
		case "End":
			return upper;
		default:
			return null;
	}
};
