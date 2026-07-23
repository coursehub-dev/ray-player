package emotion

import "testing"

func TestDecisionDAGBlocksJoyPartyWhenCombatHigh(t *testing.T) {
	b := Basis{Motion: 0.70, Joy: 0.55, Combat: 0.58, Pressure: 0.58, Roughness: 0.55, Serenity: 0.35, Brightness: 0.55}
	d := Debug{CleanBright: 0.65, CleanParty: 0.65, JoyConfidence: 0.70, DirtyElectro: 0.25, EdgeDrive: 0.35}
	audit := DecideLabel(b, d, DefaultTuning())
	if audit.Winner == "joy_party" {
		t.Fatalf("joy_party must be impossible with high combat/pressure/roughness: %+v", audit)
	}
}

func TestDecisionDAGPrefersMelancholyPressureOverGrief(t *testing.T) {
	b := Basis{Melancholy: 0.55, Pressure: 0.50, Roughness: 0.42, Combat: 0.38, Serenity: 0.48, Joy: 0.20}
	d := Debug{TensePressure: 0.62, VocalGrief: 0.75}
	audit := DecideLabel(b, d, DefaultTuning())
	if audit.Winner != "melancholy_pressure" {
		t.Fatalf("Radiohead-like pressure melancholy should win, got %s audit=%+v", audit.Winner, audit)
	}
}

func TestDecisionDAGBlocksDirtyElectroForNightSmooth(t *testing.T) {
	b := Basis{Serenity: 0.52, Combat: 0.39, Pressure: 0.46, Roughness: 0.35, Joy: 0.25, Melancholy: 0.36}
	d := Debug{WarmGroove: 0.52, VocalGrief: 0.55, DirtyElectro: 0.50, EdgeDrive: 0.44}
	audit := DecideLabel(b, d, DefaultTuning())
	if audit.Winner == "dirty_electro_combat" {
		t.Fatalf("night smooth evidence should block dirty electro, audit=%+v", audit)
	}
}

func TestSereneCounterEvidenceAcceptsSmoothLowPressureVocalState(t *testing.T) {
	b := Basis{Serenity: 0.69, Smoothness: 0.74, Pressure: 0.33, Roughness: 0.04, Combat: 0.18}
	d := Debug{TensePressure: 0.40}
	if !hasSereneCounterEvidence(b, d) {
		t.Fatalf("ethereal serene vocal state should have serene counter evidence: serenity=%.3f smooth=%.3f", b.Serenity, b.Smoothness)
	}
}

func TestSereneCounterEvidenceRejectsHighPressureGriefState(t *testing.T) {
	b := Basis{Serenity: 0.54, Smoothness: 0.61, Pressure: 0.36, Roughness: 0.11, Combat: 0.17}
	d := Debug{TensePressure: 0.42}
	if hasSereneCounterEvidence(b, d) {
		t.Fatalf("Adele-like grief state should NOT have serene counter evidence: serenity=%.3f smooth=%.3f", b.Serenity, b.Smoothness)
	}
}

func TestSereneCounterEvidenceRejectsRoughOrCombatState(t *testing.T) {
	b := Basis{Serenity: 0.70, Smoothness: 0.65, Pressure: 0.30, Roughness: 0.42, Combat: 0.41}
	d := Debug{TensePressure: 0.35}
	if hasSereneCounterEvidence(b, d) {
		t.Fatalf("rough combat state should NOT pass: rough=%.3f combat=%.3f", b.Roughness, b.Combat)
	}
}

func TestSereneCounterEvidenceBoundarySerenityBelowMin(t *testing.T) {
	b := Basis{Serenity: 0.61, Smoothness: 0.70, Pressure: 0.30, Roughness: 0.05, Combat: 0.15}
	d := Debug{TensePressure: 0.35}
	if hasSereneCounterEvidence(b, d) {
		t.Fatalf("serenity below min should reject: serenity=%.3f", b.Serenity)
	}
}

func TestSereneCounterEvidenceBoundaryPressureAtMax(t *testing.T) {
	b := Basis{Serenity: 0.70, Smoothness: 0.70, Pressure: 0.42, Roughness: 0.05, Combat: 0.15}
	d := Debug{TensePressure: 0.35}
	if hasSereneCounterEvidence(b, d) {
		t.Fatalf("pressure at max should reject: pressure=%.3f", b.Pressure)
	}
}

func TestDecisionDAGReturnsExpectedAuditRows(t *testing.T) {
	b := Basis{Joy: 0.2, Combat: 0.55, Pressure: 0.55, Roughness: 0.50}
	d := Debug{DirtyElectro: 0.20, EdgeDrive: 0.55}
	audit := DecideLabel(b, d, DefaultTuning())
	if len(audit.Results) < 8 {
		t.Fatalf("DAG should evaluate all labels, got %d", len(audit.Results))
	}
	foundJoy := false
	for _, r := range audit.Results {
		if r.Label == "joy_party" {
			foundJoy = true
			if r.Passed {
				t.Fatalf("joy_party should not pass for combat track: %+v", r)
			}
			if len(r.Failed) == 0 && len(r.BlockedBy) == 0 {
				t.Fatalf("failed joy_party should explain why: %+v", r)
			}
		}
	}
	if !foundJoy {
		t.Fatalf("DAG audit must include joy_party row")
	}
}

func TestNightRootedIdentityHighForDensePressureRoughTrack(t *testing.T) {
	b := Basis{Density: 0.75, Pressure: 0.56, Roughness: 0.52, Combat: 0.54}
	identity := nightRootedIdentity(b)
	if identity < 0.55 {
		t.Fatalf("Snoop-like dense pressured rough track should have high night identity, got %.3f", identity)
	}
}

func TestNightRootedIdentityLowForEtherealCalmTrack(t *testing.T) {
	b := Basis{Density: 0.48, Pressure: 0.33, Roughness: 0.04, Combat: 0.18}
	identity := nightRootedIdentity(b)
	if identity > 0.35 {
		t.Fatalf("Enya-like ethereal calm track should have low night identity, got %.3f", identity)
	}
}

func TestNightRootedIdentityLowForPureMelancholyCalm(t *testing.T) {
	b := Basis{Density: 0.25, Pressure: 0.21, Roughness: 0.02, Combat: 0.12}
	identity := nightRootedIdentity(b)
	if identity > 0.25 {
		t.Fatalf("Beethoven-like pure calm should have low night identity, got %.3f", identity)
	}
}

func TestNightSmoothNeedsMoreThanDarkSereneSmooth(t *testing.T) {
	ethereal := Basis{Serenity: 0.69, Smoothness: 0.74, Brightness: 0.07, Melancholy: 0.56, Pressure: 0.33, Combat: 0.18, Density: 0.48, Roughness: 0.04}
	groove := Basis{Serenity: 0.47, Smoothness: 0.24, Brightness: 0.21, Melancholy: 0.36, Pressure: 0.56, Combat: 0.54, Density: 0.75, Roughness: 0.52}
	scoreEthereal := nightSmoothScore(ethereal, Debug{})
	scoreGroove := nightSmoothScore(groove, Debug{})
	if scoreEthereal >= scoreGroove {
		t.Fatalf("night_smooth should score groove track higher than ethereal: ethereal=%.3f groove=%.3f", scoreEthereal, scoreGroove)
	}
}

func TestGriefDominanceRequiresMoreThanVocalGrief(t *testing.T) {
	b := Basis{Melancholy: 0.60, Serenity: 0.60, Smoothness: 0.69, Pressure: 0.32, Roughness: 0.04}
	d := Debug{VocalGrief: 0.85}
	cfg := DefaultTuning()
	dominance := griefDominanceScore(b, d)
	if isGriefDominant(b, d, cfg) {
		t.Fatalf("Billie-like calm melancholy should NOT be grief dominant: dominance=%.3f", dominance)
	}
}

func TestGriefDominanceAcceptsStrongUnopposedGrief(t *testing.T) {
	b := Basis{Melancholy: 0.56, Serenity: 0.54, Smoothness: 0.61, Pressure: 0.36, Roughness: 0.11}
	d := Debug{VocalGrief: 0.75}
	cfg := DefaultTuning()
	dominance := griefDominanceScore(b, d)
	if !isGriefDominant(b, d, cfg) {
		t.Fatalf("Adele-like grief state should BE grief dominant: dominance=%.3f", dominance)
	}
}

func TestGriefDominanceRejectsSereneEtherealState(t *testing.T) {
	b := Basis{Melancholy: 0.56, Serenity: 0.69, Smoothness: 0.74, Pressure: 0.33, Roughness: 0.04}
	d := Debug{VocalGrief: 0.75}
	cfg := DefaultTuning()
	dominance := griefDominanceScore(b, d)
	if isGriefDominant(b, d, cfg) {
		t.Fatalf("Enya-like serene state should NOT be grief dominant: dominance=%.3f", dominance)
	}
}

func TestGriefDominanceScoreRanksAdeleAboveBillie(t *testing.T) {
	adele := Basis{Melancholy: 0.56, Serenity: 0.54, Smoothness: 0.61, Pressure: 0.36, Roughness: 0.11}
	billie := Basis{Melancholy: 0.60, Serenity: 0.60, Smoothness: 0.69, Pressure: 0.32, Roughness: 0.04}
	d := Debug{VocalGrief: 0.80}
	scoreAdele := griefDominanceScore(adele, d)
	scoreBillie := griefDominanceScore(billie, d)
	if scoreAdele <= scoreBillie {
		t.Fatalf("Adele should have higher grief dominance than Billie: adele=%.3f billie=%.3f", scoreAdele, scoreBillie)
	}
}

func TestCalmMelancholyAcceptsInstrumentalMelancholy(t *testing.T) {
	b := Basis{Melancholy: 0.602, Serenity: 0.726, Joy: 0.233, Smoothness: 0.845, Pressure: 0.214, Roughness: 0.021, Combat: 0.120}
	d := Debug{VocalGrief: 0.705}
	if !isCalmMelancholy(b, d, DefaultTuning()) {
		t.Fatalf("Beethoven-like instrumental melancholy should pass: mel=%.3f joy=%.3f", b.Melancholy, b.Joy)
	}
}

func TestCalmMelancholyAcceptsSoftVocalMelancholy(t *testing.T) {
	b := Basis{Melancholy: 0.602, Serenity: 0.602, Joy: 0.456, Smoothness: 0.689, Pressure: 0.318, Roughness: 0.039, Combat: 0.170}
	d := Debug{VocalGrief: 0.752}
	if !isCalmMelancholy(b, d, DefaultTuning()) {
		t.Fatalf("Billie-like soft vocal melancholy should pass: mel=%.3f joy=%.3f", b.Melancholy, b.Joy)
	}
}

func TestCalmMelancholyAcceptsJoyfulMelancholy(t *testing.T) {
	b := Basis{Melancholy: 0.490, Serenity: 0.700, Joy: 0.459, Pressure: 0.292, Combat: 0.113}
	d := Debug{VocalGrief: 0.763}
	if !isCalmMelancholy(b, d, DefaultTuning()) {
		t.Fatalf("Johnny-like calm melancholy with joy should pass: mel=%.3f joy=%.3f", b.Melancholy, b.Joy)
	}
}

func TestCalmMelancholyRejectsSereneNeutralCalm(t *testing.T) {
	b := Basis{Melancholy: 0.330, Serenity: 0.629, Joy: 0.472, Pressure: 0.424, Combat: 0.204}
	d := Debug{VocalGrief: 0.562}
	if isCalmMelancholy(b, d, DefaultTuning()) {
		t.Fatalf("Coldplay-like serene neutral calm should NOT pass: mel=%.3f", b.Melancholy)
	}
}

func TestCalmMelancholyRejectsGriefDominantState(t *testing.T) {
	b := Basis{Melancholy: 0.560, Serenity: 0.535, Joy: 0.357, Pressure: 0.359, Combat: 0.172}
	d := Debug{VocalGrief: 0.749}
	if isCalmMelancholy(b, d, DefaultTuning()) {
		t.Fatalf("Adele-like grief dominant state should NOT pass: mel=%.3f vocalGrief=%.3f", b.Melancholy, d.VocalGrief)
	}
}

func TestCalmMelancholyRejectsPressureDominantState(t *testing.T) {
	b := Basis{Melancholy: 0.568, Serenity: 0.486, Joy: 0.243, Pressure: 0.504, Combat: 0.370}
	d := Debug{VocalGrief: 0.680, TensePressure: 0.573}
	if isCalmMelancholy(b, d, DefaultTuning()) {
		t.Fatalf("Radiohead-like pressure dominant state should NOT pass: press=%.3f combat=%.3f", b.Pressure, b.Combat)
	}
}

func TestCalmMelancholyDoesNotCaptureSereneEtherealState(t *testing.T) {
	b := Basis{Melancholy: 0.560, Serenity: 0.692, Joy: 0.298, Pressure: 0.330, Combat: 0.179}
	d := Debug{VocalGrief: 0.746}
	if isCalmMelancholy(b, d, DefaultTuning()) {
		t.Fatalf("Enya-like serene ethereal state should NOT pass calm melancholy: mel=%.3f joy=%.3f", b.Melancholy, b.Joy)
	}
}

func TestCalmMelancholyDoesNotRequireVocals(t *testing.T) {
	b := Basis{Melancholy: 0.602, Serenity: 0.726, Joy: 0.233, Smoothness: 0.845, Pressure: 0.214, Roughness: 0.021, Combat: 0.120}
	d := Debug{VocalGrief: 0.0}
	if !isCalmMelancholy(b, d, DefaultTuning()) {
		t.Fatalf("instrumental melancholy should pass without vocals: mel=%.3f", b.Melancholy)
	}
}

func TestDramaticArcScoreQueenLikeProfileHigherThanPurePressureTrack(t *testing.T) {
	queen := Basis{Combat: 0.169, Pressure: 0.331, Melancholy: 0.560, Impact: 0.309, Sprint: 0.366, Joy: 0.376, Serenity: 0.510, Motion: 0.525, Density: 0.405, Smoothness: 0.585, Roughness: 0.129}
	radiohead := Basis{Combat: 0.370, Pressure: 0.504, Melancholy: 0.568, Impact: 0.420, Sprint: 0.380, Joy: 0.243, Serenity: 0.486, Motion: 0.440, Density: 0.550, Smoothness: 0.480, Roughness: 0.420}
	scoreQueen := dramaticArcScore(queen)
	scoreRadiohead := dramaticArcScore(radiohead)
	if scoreQueen >= scoreRadiohead {
		t.Fatalf("Queen-like dramatic profile should NOT beat Radiohead-like pressure: queen=%.3f radiohead=%.3f", scoreQueen, scoreRadiohead)
	}
}

func TestDramaticMixedStateScoreQueenLikeProfile(t *testing.T) {
	queen := Basis{Combat: 0.169, Pressure: 0.331, Melancholy: 0.560, Impact: 0.309, Sprint: 0.366, Joy: 0.376, Serenity: 0.510, Motion: 0.525, Density: 0.405, Smoothness: 0.585, Roughness: 0.129}
	score := dramaticMixedStateScore(queen)
	if score < 0.40 {
		t.Fatalf("Queen-like mixed dramatic profile should have significant dramatic mixed state score, got %.3f", score)
	}
}

func TestDramaticMixedStateScoreBlocksPureCombatTrack(t *testing.T) {
	combat := Basis{Combat: 0.65, Pressure: 0.58, Melancholy: 0.15, Impact: 0.52, Sprint: 0.44, Joy: 0.10, Serenity: 0.18, Motion: 0.40, Density: 0.70, Roughness: 0.55}
	score := dramaticMixedStateScore(combat)
	if score > 0.35 {
		t.Fatalf("pure combat track should NOT have high dramatic mixed state, got %.3f", score)
	}
}

func TestDramaticMixedStateScoreBlocksPureSereneTrack(t *testing.T) {
	serene := Basis{Combat: 0.12, Pressure: 0.21, Melancholy: 0.35, Impact: 0.18, Sprint: 0.22, Joy: 0.45, Serenity: 0.75, Motion: 0.30, Density: 0.25, Smoothness: 0.80, Roughness: 0.04}
	score := dramaticMixedStateScore(serene)
	if score > 0.30 {
		t.Fatalf("pure serene track should NOT have high dramatic mixed state, got %.3f", score)
	}
}

func TestNightSmoothClassifiesCorrectlyDespiteTexturalWeight(t *testing.T) {
	snoop := Basis{Serenity: 0.48, Smoothness: 0.26, Brightness: 0.22, Melancholy: 0.38, Pressure: 0.52, Combat: 0.42, Density: 0.75, Roughness: 0.52}
	d := Debug{WarmGroove: 0.56, VocalGrief: 0.55, DirtyElectro: 0.35, EdgeDrive: 0.44, TensePressure: 0.42}
	audit := DecideLabel(snoop, d, DefaultTuning())
	if audit.Winner != "night_smooth" {
		t.Fatalf("Snoop-like track should be night_smooth despite textural weight, got %s audit=%+v", audit.Winner, audit)
	}
}

func TestNightSmoothDoesNotCaptureCombatTrack(t *testing.T) {
	rammstein := Basis{Serenity: 0.12, Smoothness: 0.10, Brightness: 0.22, Melancholy: 0.15, Pressure: 0.68, Combat: 0.75, Density: 0.80, Roughness: 0.62}
	d := Debug{WarmGroove: 0.18, VocalGrief: 0.10, DirtyElectro: 0.22, EdgeDrive: 0.72, TensePressure: 0.65}
	audit := DecideLabel(rammstein, d, DefaultTuning())
	if audit.Winner == "night_smooth" {
		t.Fatalf("Rammstein-like track should NOT be night_smooth, got %+v", audit)
	}
}
