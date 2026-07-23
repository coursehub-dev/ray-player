package emotion

import "testing"

func TestJoyPartyAccentIsOrange(t *testing.T) {
	b := Basis{Label: "joy_party", Joy: 0.55, Brightness: 0.58, Serenity: 0.40, Combat: 0.15, Melancholy: 0.22, Pressure: 0.28, Swagger: 0.48, Roughness: 0.20}
	a := AccentFromEmotion(b, 0.70)
	if a.Hue < 25 || a.Hue > 50 {
		t.Fatalf("joy_party hue should be in orange range 25-50, got %f", a.Hue)
	}
	if a.Family != "positive" {
		t.Fatalf("joy_party family should be positive, got %s", a.Family)
	}
}

func TestCombatAccentIsRed(t *testing.T) {
	b := Basis{Label: "combat_force", Combat: 0.72, Pressure: 0.65, Roughness: 0.58, Brightness: 0.22, Serenity: 0.12, Joy: 0.10, Melancholy: 0.15}
	a := AccentFromEmotion(b, 0.70)
	hue := a.Hue
	if hue > 10 && hue < 350 {
		t.Fatalf("combat_force hue should be in red range 350-360 or 0-10, got %f", hue)
	}
	if a.Family != "danger" {
		t.Fatalf("combat_force family should be danger, got %s", a.Family)
	}
}

func TestMelancholyCalmAccentIsBlue(t *testing.T) {
	b := Basis{Label: "melancholy_calm", Melancholy: 0.55, Serenity: 0.62, Combat: 0.14, Pressure: 0.30, Brightness: 0.30, Joy: 0.25}
	a := AccentFromEmotion(b, 0.70)
	if a.Hue < 215 || a.Hue > 235 {
		t.Fatalf("melancholy_calm hue should be in blue range 215-235, got %f", a.Hue)
	}
	if a.Family != "melancholy" {
		t.Fatalf("melancholy_calm family should be melancholy, got %s", a.Family)
	}
}

func TestSereneCalmAccentIsTeal(t *testing.T) {
	b := Basis{Label: "serene_calm", Serenity: 0.72, Brightness: 0.45, Combat: 0.12, Pressure: 0.25, Melancholy: 0.32, Joy: 0.42, Smoothness: 0.70}
	a := AccentFromEmotion(b, 0.70)
	if a.Hue < 165 || a.Hue > 200 {
		t.Fatalf("serene_calm hue should be in teal range 165-200, got %f", a.Hue)
	}
	if a.Family != "calm" {
		t.Fatalf("serene_calm family should be calm, got %s", a.Family)
	}
}

func TestNightSmoothAccentIsPurpleNotRed(t *testing.T) {
	b := Basis{Label: "night_smooth", Serenity: 0.48, Melancholy: 0.38, Combat: 0.42, Pressure: 0.52, Roughness: 0.52, Brightness: 0.22, Density: 0.75}
	a := AccentFromEmotion(b, 0.70)
	if a.Hue < 250 || a.Hue > 300 {
		t.Fatalf("night_smooth hue should be in purple range 250-300, got %f", a.Hue)
	}
	if a.Family != "warm" {
		t.Fatalf("night_smooth family should be warm, got %s", a.Family)
	}
}

func TestLowConfidenceAccentIsDesaturated(t *testing.T) {
	b := Basis{Label: "joy_party", Joy: 0.55, Brightness: 0.58, Combat: 0.15, Serenity: 0.40, Pressure: 0.28}
	aHigh := AccentFromEmotion(b, 0.80)
	aLow := AccentFromEmotion(b, 0.30)
	if aLow.Saturation >= aHigh.Saturation {
		t.Fatalf("low confidence should reduce saturation: high=%.1f low=%.1f", aHigh.Saturation, aLow.Saturation)
	}
}

func TestDramaticArcAccentReflectsMixedState(t *testing.T) {
	b := Basis{Label: "dramatic_arc", Joy: 0.38, Melancholy: 0.52, Serenity: 0.51, Combat: 0.17, Pressure: 0.33, Brightness: 0.45}
	a := AccentFromEmotion(b, 0.70)
	if a.Family != "mixed" {
		t.Fatalf("dramatic_arc family should be mixed, got %s", a.Family)
	}
	if a.Hue < 260 || a.Hue > 320 {
		t.Fatalf("dramatic_arc hue should be in violet-red range 260-320, got %f", a.Hue)
	}
}

func TestSereneBrightAccentIsLightCyan(t *testing.T) {
	b := Basis{Label: "serene_bright", Serenity: 0.68, Brightness: 0.55, Combat: 0.10, Pressure: 0.22, Melancholy: 0.28, Joy: 0.45, Smoothness: 0.65}
	a := AccentFromEmotion(b, 0.70)
	if a.Hue < 160 || a.Hue > 190 {
		t.Fatalf("serene_bright hue should be in light cyan range 160-190, got %f", a.Hue)
	}
}

func TestDirtyElectroAccentIsRedMagenta(t *testing.T) {
	b := Basis{Label: "dirty_electro_combat", Roughness: 0.62, Pressure: 0.58, Combat: 0.52, Brightness: 0.25, Serenity: 0.15, Joy: 0.08}
	a := AccentFromEmotion(b, 0.70)
	if a.Hue < 335 || a.Hue > 360 {
		if a.Hue > 10 {
			t.Fatalf("dirty_electro_combat hue should be in red-magenta range 335-360, got %f", a.Hue)
		}
	}
}

func TestAccentSaturationIncreasesWithEnergy(t *testing.T) {
	calm := Basis{Label: "serene_calm", Serenity: 0.72, Combat: 0.12, Pressure: 0.25, Joy: 0.30, Brightness: 0.40}
	intense := Basis{Label: "combat_force", Serenity: 0.12, Combat: 0.72, Pressure: 0.65, Joy: 0.10, Brightness: 0.22}
	aCalm := AccentFromEmotion(calm, 0.70)
	aIntense := AccentFromEmotion(intense, 0.70)
	if aIntense.Saturation <= aCalm.Saturation {
		t.Fatalf("intense track should have higher saturation: calm=%.1f intense=%.1f", aCalm.Saturation, aIntense.Saturation)
	}
}

func TestAccentLightnessDecreasesWithCombat(t *testing.T) {
	serene := Basis{Label: "serene_calm", Serenity: 0.72, Combat: 0.12, Pressure: 0.25, Joy: 0.40, Brightness: 0.55}
	combat := Basis{Label: "combat_force", Serenity: 0.12, Combat: 0.72, Pressure: 0.65, Joy: 0.10, Brightness: 0.22}
	aSerene := AccentFromEmotion(serene, 0.70)
	aCombat := AccentFromEmotion(combat, 0.70)
	if aCombat.Lightness >= aSerene.Lightness {
		t.Fatalf("combat should have lower lightness: serene=%.1f combat=%.1f", aSerene.Lightness, aCombat.Lightness)
	}
}
