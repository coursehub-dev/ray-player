package emotion

// Tuning is the only place where the LLM is allowed to tune the emotion engine.
//
// Rules for calibration:
//   - change at most 3-5 numeric values per experiment;
//   - coefficient delta: max ±0.03 per experiment;
//   - threshold delta: max ±0.02 per experiment;
//   - do not change genre logic, DB schema, audio extraction or TempoCNN;
//   - do not tune to make one track correct if it breaks a whole quadrant.
type Tuning struct {
	Version string

	// Motion / activation.
	MotionDance       float64
	MotionGroove      float64
	MotionParty       float64
	MotionSprintTempo float64
	MotionDensity     float64

	// Density / body.
	DensityLoudness    float64
	DensityRMS         float64
	DensityDynamic     float64
	DensityLowBand     float64
	DensityHighPenalty float64

	// Texture / roughness / edge.
	EdgeZCR        float64
	EdgeCentroid   float64
	EdgeDensity    float64
	EdgeElectronic float64
	EdgeRelaxInv   float64
	EdgeValInv     float64

	RoughZCR        float64
	RoughFlatness   float64
	RoughFlux       float64
	RoughAggressive float64
	RoughRelaxInv   float64
	RoughOnset      float64
	RoughValInv     float64
	RoughCleanCut   float64

	// Brightness.
	BrightRolloff  float64
	BrightCentroid float64
	BrightTimbre   float64
	BrightHighBand float64
	BrightValence  float64

	// Smoothness.
	SmoothRelax      float64
	SmoothFlatInv    float64
	SmoothZCRInv     float64
	SmoothFluxInv    float64
	SmoothOnsetInv   float64
	SmoothInstrument float64
	SmoothRoughCut   float64
	SmoothImpactCut  float64

	// Pressure / impact.
	ImpactDensity    float64
	ImpactLowBand    float64
	ImpactOnset      float64
	ImpactRelaxInv   float64
	ImpactParty      float64
	PressureImpact   float64
	PressureRough    float64
	PressureMotion   float64
	PressureSprint   float64
	PressureRelaxInv float64

	// Joy.
	JoyValence     float64
	JoyCleanBright float64
	JoyBrightness  float64
	JoyDance       float64
	JoyParty       float64
	JoyGroove      float64
	JoyHappy       float64
	JoyCombatCut   float64
	JoyRoughCut    float64
	JoyPressureCut float64
	JoyEdgeCut     float64
	JoyDirtyCut    float64
	JoyCombatFloor float64

	// Melancholy.
	MelValInv    float64
	MelBrightInv float64
	MelVocal     float64
	MelMotionInv float64
	MelSmooth    float64
	MelSad       float64
	MelDynamic   float64
	MelWarmCut   float64
	MelJoyCut    float64

	// Serenity / warmth.
	SerenitySmooth      float64
	SerenityRelax       float64
	SerenityPressureInv float64
	SerenityPartyInv    float64
	SerenityRoughInv    float64
	SerenityLowBand     float64
	SerenityInstr       float64

	// Swagger / combat / sprint.
	SwaggerMotion     float64
	SwaggerGroove     float64
	SwaggerParty      float64
	SwaggerVocal      float64
	SwaggerDensity    float64
	SwaggerMelInv     float64
	SwaggerValence    float64
	CombatRough       float64
	CombatPressure    float64
	CombatImpact      float64
	CombatSmoothInv   float64
	CombatAggressive  float64
	CombatOnset       float64
	CombatJoyCut      float64
	SprintTempo       float64
	SprintMotion      float64
	SprintDensity     float64
	SprintElectronic  float64
	SprintParty       float64
	SprintSerenityInv float64
	SprintCombatCut   float64
	SprintRoughCut    float64

	// Label gates.
	JoyPartyMinJoy          float64
	JoyPartyMinClean        float64
	JoyPartyMaxRough        float64
	JoyPartyMaxPressure     float64
	JoyPartyMaxDirty        float64
	JoyPartyMaxEdge         float64
	CombatMinCombat         float64
	CombatMinPressure       float64
	CombatMinRough          float64
	DirtyMinDirty           float64
	DirtyMinPressure        float64
	MelGriefMinVocalGrief   float64
	MelGriefMinMelancholy   float64
	WarmGrooveMinWarmGroove float64
}

func DefaultTuning() Tuning {
	return Tuning{
		Version: "emotion-v6-stable-semantic-scale",

		MotionDance: 0.36, MotionGroove: 0.22, MotionParty: 0.16, MotionSprintTempo: 0.14, MotionDensity: 0.12,
		DensityLoudness: 0.36, DensityRMS: 0.34, DensityDynamic: 0.30, DensityLowBand: 0.00, DensityHighPenalty: 0.00,

		EdgeZCR: 0.30, EdgeCentroid: 0.24, EdgeDensity: 0.16, EdgeElectronic: 0.14, EdgeRelaxInv: 0.10, EdgeValInv: 0.06,
		RoughZCR: 0.24, RoughFlatness: 0.22, RoughFlux: 0.18, RoughAggressive: 0.12, RoughRelaxInv: 0.10, RoughOnset: 0.08, RoughValInv: 0.06, RoughCleanCut: 0.22,

		BrightRolloff: 0.34, BrightCentroid: 0.24, BrightTimbre: 0.18, BrightHighBand: 0.14, BrightValence: 0.10,

		SmoothRelax: 0.30, SmoothFlatInv: 0.20, SmoothZCRInv: 0.18, SmoothFluxInv: 0.14, SmoothOnsetInv: 0.10, SmoothInstrument: 0.08,
		SmoothRoughCut: 0.18, SmoothImpactCut: 0.12,

		ImpactDensity: 0.42, ImpactLowBand: 0.20, ImpactOnset: 0.16, ImpactRelaxInv: 0.12, ImpactParty: 0.10,
		PressureImpact: 0.34, PressureRough: 0.26, PressureMotion: 0.18, PressureSprint: 0.12, PressureRelaxInv: 0.10,

		JoyValence: 0.30, JoyCleanBright: 0.22, JoyBrightness: 0.16, JoyDance: 0.16, JoyParty: 0.10, JoyGroove: 0.06, JoyHappy: 0.04,
		JoyCombatCut: 0.10, JoyRoughCut: 0.08, JoyPressureCut: 0.06, JoyEdgeCut: 0.15, JoyDirtyCut: 0.10, JoyCombatFloor: 0.44,

		MelValInv: 0.30, MelBrightInv: 0.18, MelVocal: 0.16, MelMotionInv: 0.14, MelSmooth: 0.12, MelSad: 0.06, MelDynamic: 0.04,
		MelWarmCut: 0.18, MelJoyCut: 0.12,

		SerenitySmooth: 0.30, SerenityRelax: 0.20, SerenityPressureInv: 0.16, SerenityPartyInv: 0.12, SerenityRoughInv: 0.08,
		SerenityLowBand: 0.08, SerenityInstr: 0.06,

		SwaggerMotion: 0.26, SwaggerGroove: 0.20, SwaggerParty: 0.18, SwaggerVocal: 0.14, SwaggerDensity: 0.10, SwaggerMelInv: 0.08, SwaggerValence: 0.04,
		CombatRough: 0.30, CombatPressure: 0.24, CombatImpact: 0.20, CombatSmoothInv: 0.12, CombatAggressive: 0.08, CombatOnset: 0.06, CombatJoyCut: 0.18,
		SprintTempo: 0.28, SprintMotion: 0.24, SprintDensity: 0.16, SprintElectronic: 0.12, SprintParty: 0.10, SprintSerenityInv: 0.10, SprintCombatCut: 0.18, SprintRoughCut: 0.10,

		JoyPartyMinJoy: 0.44, JoyPartyMinClean: 0.50, JoyPartyMaxRough: 0.30, JoyPartyMaxPressure: 0.40, JoyPartyMaxDirty: 0.30, JoyPartyMaxEdge: 0.42,
		CombatMinCombat: 0.44, CombatMinPressure: 0.46, CombatMinRough: 0.48,
		DirtyMinDirty: 0.52, DirtyMinPressure: 0.48,
		MelGriefMinVocalGrief: 0.68, MelGriefMinMelancholy: 0.56,
		WarmGrooveMinWarmGroove: 0.70,
	}
}

func (t Tuning) Sanitized() Tuning {
	d := DefaultTuning()
	if t.Version == "" {
		t.Version = d.Version
	}
	return t
}
