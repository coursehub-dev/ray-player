package emotion

import "math"

type Accent struct {
	Hue        float64 `json:"h"`
	Saturation float64 `json:"s"`
	Lightness  float64 `json:"l"`
	Family     string  `json:"family"`
	Label      string  `json:"label"`
}

var labelHueAnchors = map[LabelID]float64{
	LabelJoyParty:           36,
	LabelJoyFunk:            29,
	LabelUpliftDrive:        47,
	LabelCombatForce:        4,
	LabelDirtyElectroCombat: 348,
	LabelTensePressure:      352,
	LabelMelancholyPressure: 258,
	LabelMelancholyGrief:    230,
	LabelMelancholyCalm:     225,
	LabelSereneCalm:         185,
	LabelSereneBright:       175,
	LabelSereneWarmGroove:   32,
	LabelNightSmooth:        275,
	LabelDramaticArc:        290,
	LabelStreetSwagger:      35,
	LabelNeutral:            230,
}

func AccentFromEmotion(b Basis, confidence float64) Accent {
	label := LabelID(b.Label)
	fam := labelFamily(label)

	hueAnchor, ok := labelHueAnchors[label]
	if !ok {
		hueAnchor = 230
	}

	hueShift := computeHueShift(b, fam)
	hue := math.Mod(hueAnchor+hueShift+360, 360)

	sat := computeSaturation(b, confidence)
	light := computeLightness(b)

	return Accent{
		Hue:        math.Round(hue*10) / 10,
		Saturation: math.Round(sat*10) / 10,
		Lightness:  math.Round(light*10) / 10,
		Family:     fam,
		Label:      string(label),
	}
}

func labelFamily(label LabelID) string {
	switch label {
	case LabelSereneCalm, LabelSereneBright:
		return "calm"
	case LabelSereneWarmGroove, LabelNightSmooth:
		return "warm"
	case LabelMelancholyGrief, LabelMelancholyCalm, LabelMelancholyPressure:
		return "melancholy"
	case LabelDramaticArc:
		return "mixed"
	case LabelJoyParty, LabelJoyFunk, LabelUpliftDrive, LabelStreetSwagger:
		return "positive"
	case LabelCombatForce, LabelDirtyElectroCombat, LabelTensePressure:
		return "danger"
	default:
		return "neutral"
	}
}

func computeHueShift(b Basis, family string) float64 {
	shift := 0.0

	if family == "positive" {
		shift += (b.Joy - 0.40) * 15
		shift += (b.Brightness - 0.40) * 8
	}
	if family == "melancholy" {
		shift += (b.Melancholy - 0.45) * -12
		shift += (b.Serenity - 0.50) * 5
	}
	if family == "calm" {
		shift += (b.Serenity - 0.60) * 6
		shift += (b.Brightness - 0.40) * -4
	}
	if family == "danger" {
		shift += (b.Combat - 0.50) * 5
		shift += (b.Roughness - 0.45) * 3
	}
	if family == "pressure" {
		shift += (b.Pressure - 0.50) * -4
		shift += (b.Melancholy - 0.45) * 6
	}
	if family == "warm" {
		shift += (b.Serenity - 0.50) * 4
		shift += (b.Melancholy - 0.40) * 3
	}
	if family == "mixed" {
		positive := math.Max(b.Joy, math.Max(b.Serenity, b.Brightness))
		negative := math.Max(b.Melancholy, math.Max(b.Pressure, b.Combat))
		shift += (positive - negative) * 12
	}

	return clamp(shift, -20, 20)
}

func computeSaturation(b Basis, confidence float64) float64 {
	energy := math.Max(b.Combat, math.Max(b.Pressure, b.Joy))
	vibrancy := math.Max(b.Brightness, b.Swagger)

	sat := 55 + energy*25 + vibrancy*10

	if confidence < 0.60 {
		sat *= 0.70 + confidence*0.50
	}
	if b.Roughness > 0.50 {
		sat += (b.Roughness - 0.50) * 12
	}
	if b.Serenity > 0.65 && b.Melancholy < 0.35 {
		sat -= (b.Serenity - 0.65) * 20
	}

	return clamp(sat, 25, 92)
}

func computeLightness(b Basis) float64 {
	light := 52.0

	light += (b.Brightness - 0.40) * 18
	light += (b.Joy - 0.35) * 10
	light -= (b.Melancholy - 0.45) * 12
	light -= (b.Pressure - 0.45) * 8
	light -= (b.Combat - 0.40) * 6
	light += (b.Serenity - 0.55) * 5

	return clamp(light, 28, 78)
}
