package emotion

import "testing"

func TestEvaluateTransitionSereneToExtremeCombatBlocked(t *testing.T) {
	prev := Result{Basis: Basis{Label: "serene_calm", Serenity: 0.72, Combat: 0.15, Pressure: 0.28, Roughness: 0.06}}
	next := Result{Basis: Basis{Label: "combat_force", Serenity: 0.12, Combat: 0.72, Pressure: 0.65, Roughness: 0.58}}
	tc := EvaluateTransition(prev, next)
	if tc.Allowed {
		t.Fatalf("serene_calm -> combat_force should be blocked, got %+v", tc)
	}
}

func TestEvaluateTransitionJoyToCombatWithoutBridgeBlocked(t *testing.T) {
	prev := Result{Basis: Basis{Label: "joy_party", Joy: 0.58, Serenity: 0.42, Combat: 0.18, Pressure: 0.30}}
	next := Result{Basis: Basis{Label: "combat_force", Joy: 0.10, Serenity: 0.12, Combat: 0.72, Pressure: 0.65, Roughness: 0.58}}
	tc := EvaluateTransition(prev, next)
	if tc.Allowed {
		t.Fatalf("joy_party -> combat_force without bridge should be blocked, got %+v", tc)
	}
}

func TestEvaluateTransitionMelancholyCalmToSereneCalmAllowed(t *testing.T) {
	prev := Result{Basis: Basis{Label: "melancholy_calm", Melancholy: 0.55, Serenity: 0.62, Combat: 0.14, Pressure: 0.30}}
	next := Result{Basis: Basis{Label: "serene_calm", Melancholy: 0.32, Serenity: 0.72, Combat: 0.12, Pressure: 0.25}}
	tc := EvaluateTransition(prev, next)
	if !tc.Allowed {
		t.Fatalf("melancholy_calm -> serene_calm should be allowed, got %+v", tc)
	}
	if tc.Penalty > 0.30 {
		t.Fatalf("melancholy -> serene should have low penalty, got %f", tc.Penalty)
	}
}

func TestEvaluateTransitionSameFamilyZeroPenalty(t *testing.T) {
	prev := Result{Basis: Basis{Label: "serene_calm", Serenity: 0.72, Combat: 0.15}}
	next := Result{Basis: Basis{Label: "serene_bright", Serenity: 0.68, Combat: 0.12}}
	tc := EvaluateTransition(prev, next)
	if !tc.Allowed || tc.Penalty != 0 {
		t.Fatalf("same family should have zero penalty, got %+v", tc)
	}
}

func TestEvaluateTransitionMelancholyPressureCanBridgeTowardCombat(t *testing.T) {
	prev := Result{Basis: Basis{Label: "melancholy_pressure", Melancholy: 0.52, Pressure: 0.48, Combat: 0.38, Serenity: 0.40}}
	next := Result{Basis: Basis{Label: "combat_force", Melancholy: 0.18, Pressure: 0.62, Combat: 0.65, Serenity: 0.15}}
	tc := EvaluateTransition(prev, next)
	if !tc.Allowed {
		t.Fatalf("melancholy_pressure -> combat_force should be allowed (bridge possible), got %+v", tc)
	}
}

func TestEvaluateTransitionDramaticArcActsAsBridge(t *testing.T) {
	prev := Result{Basis: Basis{Label: "serene_calm", Serenity: 0.72, Combat: 0.15, Pressure: 0.28}}
	dramatic := Result{Basis: Basis{Label: "dramatic_arc", Serenity: 0.45, Combat: 0.20, Pressure: 0.35, Melancholy: 0.52}}
	next := Result{Basis: Basis{Label: "tense_pressure", Serenity: 0.22, Combat: 0.42, Pressure: 0.55}}
	tcPrevDramatic := EvaluateTransition(prev, dramatic)
	tcDramaticNext := EvaluateTransition(dramatic, next)
	if !tcPrevDramatic.Allowed {
		t.Fatalf("serene_calm -> dramatic_arc should be allowed, got %+v", tcPrevDramatic)
	}
	if !tcDramaticNext.Allowed {
		t.Fatalf("dramatic_arc -> tense_pressure should be allowed, got %+v", tcDramaticNext)
	}
}

func TestEvaluateTransitionNightSmoothNotEqualCombat(t *testing.T) {
	prev := Result{Basis: Basis{Label: "night_smooth", Serenity: 0.48, Combat: 0.42, Pressure: 0.52, Roughness: 0.50, Density: 0.72}}
	next := Result{Basis: Basis{Label: "combat_force", Serenity: 0.12, Combat: 0.72, Pressure: 0.65, Roughness: 0.58}}
	tc := EvaluateTransition(prev, next)
	if tc.Allowed && tc.Penalty > 0.70 {
		t.Fatalf("night_smooth -> combat should have significant penalty due to textural similarity, got %+v", tc)
	}
}

func TestDistanceUsesLabelPenalty(t *testing.T) {
	a := Basis{Label: "serene_calm", Combat: 0.15, Pressure: 0.28, Serenity: 0.72, Roughness: 0.06, Joy: 0.40, Melancholy: 0.35, Swagger: 0.30, SprintClean: 0.25, Brightness: 0.45, Motion: 0.35}
	b := Basis{Label: "combat_force", Combat: 0.15, Pressure: 0.28, Serenity: 0.72, Roughness: 0.06, Joy: 0.40, Melancholy: 0.35, Swagger: 0.30, SprintClean: 0.25, Brightness: 0.45, Motion: 0.35}
	bSame := Basis{Label: "serene_bright", Combat: 0.15, Pressure: 0.28, Serenity: 0.72, Roughness: 0.06, Joy: 0.40, Melancholy: 0.35, Swagger: 0.30, SprintClean: 0.25, Brightness: 0.45, Motion: 0.35}
	distDiff := Distance(a, b)
	distSame := Distance(a, bSame)
	if distDiff <= distSame {
		t.Fatalf("different family labels should produce higher distance: diff=%f same=%f", distDiff, distSame)
	}
}

func TestBridgeScoreHigherForAdjacentFamilies(t *testing.T) {
	serene := Basis{Label: "serene_calm", Serenity: 0.65, Combat: 0.15, Pressure: 0.30, Melancholy: 0.35, Joy: 0.40}
	melCalm := Basis{Label: "melancholy_calm", Serenity: 0.58, Combat: 0.18, Pressure: 0.32, Melancholy: 0.52, Joy: 0.28}
	combat := Basis{Label: "combat_force", Serenity: 0.12, Combat: 0.72, Pressure: 0.65, Melancholy: 0.15, Joy: 0.10}
	bridgeAdjacent := BridgeScore(serene, melCalm)
	bridgeDistant := BridgeScore(serene, combat)
	if bridgeAdjacent <= bridgeDistant {
		t.Fatalf("adjacent families should have higher bridge score: adjacent=%f distant=%f", bridgeAdjacent, bridgeDistant)
	}
}
