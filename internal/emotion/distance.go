package emotion

import "math"

func Distance(a, b Basis) float64 {
	d := 0.0
	d += math.Abs(a.Combat-b.Combat) * 0.18
	d += math.Abs(a.Pressure-b.Pressure) * 0.15
	d += math.Abs(a.Roughness-b.Roughness) * 0.13
	d += math.Abs(a.Joy-b.Joy) * 0.12
	d += math.Abs(a.Melancholy-b.Melancholy) * 0.13
	d += math.Abs(a.Serenity-b.Serenity) * 0.12
	d += math.Abs(a.Swagger-b.Swagger) * 0.08
	d += math.Abs(a.SprintClean-b.SprintClean) * 0.06
	d += math.Abs(a.Brightness-b.Brightness) * 0.04
	d += math.Abs(a.Motion-b.Motion) * 0.04
	d += labelDistancePenalty(a.Label, b.Label) * 0.18
	return clamp01(d)
}

func Smoothness(a, b Basis) float64 { return clamp01(1 - Distance(a, b)) }

func HardJumpRisk(a, b Basis) float64 {
	r := 0.0
	r += math.Abs(a.Combat-b.Combat) * 0.24
	r += math.Abs(a.Pressure-b.Pressure) * 0.18
	r += math.Abs(a.Roughness-b.Roughness) * 0.16
	r += math.Abs(a.Melancholy-b.Melancholy) * 0.14
	r += math.Abs(a.Serenity-b.Serenity) * 0.12
	r += math.Abs(a.Joy-b.Joy) * 0.10
	r += math.Abs(a.Swagger-b.Swagger) * 0.04
	r += math.Abs(a.SprintClean-b.SprintClean) * 0.04
	r += labelDistancePenalty(a.Label, b.Label) * 0.10
	return clamp01(r)
}

func BridgeScore(a, b Basis) float64 {
	dist := Distance(a, b)
	pressureStep := 1 - math.Min(1, math.Abs(a.Pressure-b.Pressure)/0.28)
	combatStep := 1 - math.Min(1, math.Abs(a.Combat-b.Combat)/0.28)
	serenityStep := 1 - math.Min(1, math.Abs(a.Serenity-b.Serenity)/0.32)
	joyStep := 1 - math.Min(1, math.Abs(a.Joy-b.Joy)/0.34)
	melStep := 1 - math.Min(1, math.Abs(a.Melancholy-b.Melancholy)/0.34)
	return clamp01((1-dist)*0.34 + pressureStep*0.18 + combatStep*0.16 + serenityStep*0.12 + joyStep*0.10 + melStep*0.10)
}

func labelDistancePenalty(a, b string) float64 {
	if a == "" || b == "" || a == b {
		return 0
	}

	ga := labelGroup(a)
	gb := labelGroup(b)
	if ga == "" || gb == "" {
		return 0.35
	}
	if ga == gb {
		return 0.10
	}

	switch {
	case ga == "combat" && gb == "soft":
		return 0.95
	case ga == "soft" && gb == "combat":
		return 0.95
	case ga == "combat" && gb == "joy":
		return 0.70
	case ga == "joy" && gb == "combat":
		return 0.70
	case ga == "melancholy" && gb == "joy":
		return 0.62
	case ga == "joy" && gb == "melancholy":
		return 0.62
	default:
		return 0.42
	}
}

func labelGroup(label string) string {
	switch label {
	case "combat_force", "dirty_electro_combat", "dirty_drive", "tense_pressure":
		return "combat"
	case "joy_party", "joy_funk", "street_swagger", "speed_flight", "uplift_drive":
		return "joy"
	case "melancholy_grief", "melancholy_calm", "melancholy_pressure", "dramatic_arc":
		return "melancholy"
	case "serene_calm", "serene_warm_groove", "serene_bright", "night_smooth":
		return "soft"
	case "neutral", "steady_groove":
		return "neutral"
	default:
		return ""
	}
}

type TransitionCompatibility struct {
	Allowed bool    `json:"allowed"`
	Penalty float64 `json:"penalty"`
	Reason  string  `json:"reason"`
}

func EvaluateTransition(prev, next Result) TransitionCompatibility {
	dist := Distance(prev.Basis, next.Basis)
	hard := HardJumpRisk(prev.Basis, next.Basis)
	bridge := BridgeScore(prev.Basis, next.Basis)

	fg := labelGroup(prev.Basis.Label)
	tg := labelGroup(next.Basis.Label)

	if fg == "soft" && tg == "combat" {
		return TransitionCompatibility{Allowed: false, Penalty: 0.90, Reason: "serene_to_extreme_combat"}
	}
	if fg == "combat" && tg == "soft" {
		return TransitionCompatibility{Allowed: false, Penalty: 0.90, Reason: "extreme_combat_to_serene"}
	}
	if fg == "joy" && tg == "combat" && bridge < 0.35 {
		return TransitionCompatibility{Allowed: false, Penalty: 0.75, Reason: "joy_to_combat_without_bridge"}
	}
	if fg == "melancholy" && tg == "combat" && bridge < 0.35 {
		return TransitionCompatibility{Allowed: false, Penalty: 0.65, Reason: "melancholy_to_combat_without_bridge"}
	}
	if fg == "soft" && tg == "melancholy" {
		return TransitionCompatibility{Allowed: true, Penalty: 0.15, Reason: "soft_to_melancholy_adjacent"}
	}
	if fg == "melancholy" && tg == "soft" {
		return TransitionCompatibility{Allowed: true, Penalty: 0.15, Reason: "melancholy_to_soft_adjacent"}
	}
	if fg == "melancholy" && tg == "joy" {
		return TransitionCompatibility{Allowed: true, Penalty: 0.40, Reason: "melancholy_to_joy_distant"}
	}
	if fg == "joy" && tg == "melancholy" {
		return TransitionCompatibility{Allowed: true, Penalty: 0.40, Reason: "joy_to_melancholy_distant"}
	}
	if fg == tg {
		return TransitionCompatibility{Allowed: true, Penalty: 0, Reason: "same_family"}
	}
	penalty := math.Max(dist, hard) * 0.50
	return TransitionCompatibility{Allowed: true, Penalty: penalty, Reason: "default_cross_family"}
}
