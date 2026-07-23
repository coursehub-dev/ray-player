package emotion

import "math"

const (
	atmosphericCalmPressureMax = 0.46
	atmosphericCalmSerenityMin = 0.62
	atmosphericCalmSmoothMin   = 0.65
	atmosphericCalmCombatMax   = 0.34
	atmosphericCalmRoughMax    = 0.36
	atmosphericCalmTenseMax    = 0.58
)

func isAtmosphericSereneCalm(b Basis, d Debug) bool {
	return b.Pressure < atmosphericCalmPressureMax &&
		b.Serenity >= atmosphericCalmSerenityMin &&
		b.Smoothness >= atmosphericCalmSmoothMin &&
		b.Combat < atmosphericCalmCombatMax &&
		b.Roughness < atmosphericCalmRoughMax &&
		d.TensePressure < atmosphericCalmTenseMax
}

const (
	sereneCounterSerenityMin = 0.62
	sereneCounterSmoothMin   = 0.62
	sereneCounterPressureMax = 0.42
	sereneCounterRoughMax    = 0.15
	sereneCounterCombatMax   = 0.30
	sereneCounterTenseMax    = 0.50
)

func hasSereneCounterEvidence(b Basis, d Debug) bool {
	return b.Serenity >= sereneCounterSerenityMin &&
		b.Smoothness >= sereneCounterSmoothMin &&
		b.Pressure < sereneCounterPressureMax &&
		b.Roughness < sereneCounterRoughMax &&
		b.Combat < sereneCounterCombatMax &&
		d.TensePressure < sereneCounterTenseMax
}

func nightRootedIdentity(b Basis) float64 {
	return clamp01(
		0.30*b.Density +
			0.25*b.Pressure +
			0.25*b.Roughness +
			0.20*b.Combat,
	)
}

func griefDominanceScore(b Basis, d Debug) float64 {
	griefCore := 0.4*b.Melancholy + 0.4*d.VocalGrief
	calmCounter := 0.25*b.Serenity + 0.30*b.Smoothness + 0.2*(1-b.Pressure) + 0.25*(1-b.Roughness)
	return clamp01(0.5 + griefCore - calmCounter)
}

const griefDominanceThreshold = 0.355

func isGriefDominant(b Basis, d Debug, cfg Tuning) bool {
	griefCore := b.Melancholy >= cfg.MelGriefMinMelancholy && d.VocalGrief >= cfg.MelGriefMinVocalGrief
	dominance := griefDominanceScore(b, d)
	return griefCore && dominance >= griefDominanceThreshold
}

func isCalmMelancholy(b Basis, d Debug, cfg Tuning) bool {
	if b.Melancholy < 0.48 {
		return false
	}
	if b.Pressure >= 0.44 || b.Combat >= 0.36 {
		return false
	}
	if isGriefDominant(b, d, cfg) {
		return false
	}
	return b.Joy >= 0.40 || b.Melancholy >= 0.58
}

func dramaticMixedStateScore(b Basis) float64 {
	positive := math.Max(b.Joy, math.Max(b.Serenity, b.Brightness))
	negative := math.Max(b.Melancholy, math.Max(b.Pressure, b.Combat))
	contrast := math.Min(positive, negative)

	dynamicArc := clamp01(0.35*b.Impact + 0.30*b.Motion + 0.20*b.Density + 0.15*b.Sprint)

	return clamp01(0.55*contrast + 0.45*dynamicArc)
}

func brightClarityScore(b Basis, d Debug) float64 {
	instrumentalSignal := clamp01(1.0 - 2.0*b.Swagger + b.Smoothness)
	dynamicSignal := clamp01(0.40*b.Impact + 0.30*b.Motion + 0.20*b.Sprint + 0.10*b.Density)
	tonalSignal := clamp01(0.30*b.Smoothness + 0.25*(1.0-b.Roughness) + 0.25*(1.0-b.Combat) + 0.20*d.SereneBright)
	return clamp01(0.35*instrumentalSignal + 0.35*dynamicSignal + 0.30*tonalSignal)
}
