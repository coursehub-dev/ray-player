package emotion

import (
	"math"
	"sort"

	"ray-player1/internal/library"
)

type Normalizer interface {
	Norm(name string, value float64) float64
	NormWeighted(name string, value float64) float64
	Reliability(name string) float64
}

type Inputs struct {
	Dance, Valence, Happy, Sad, Relaxed, Party, Aggressive    float64
	Acousticness, Electronicness, Instrumentalness, Vocalness float64
	Brightness, Tonality, Melodicness, Softness, Heaviness    float64
	Dreaminess, Emotionality                                  float64

	BPM, BPMPerceived, TempoConfidence, TempoStability float64

	Loudness, RMS, ZeroCrossingRate, SpectralCentroid float64
	SpectralFlatness, SpectralRolloff85, SpectralFlux float64
	OnsetRate, DynamicRange                           float64
	LowBandRatio, MidBandRatio, HighBandRatio         float64
}

type Result struct {
	Basis Basis `json:"basis"`
	Debug Debug `json:"debug,omitempty"`
}

type Basis struct {
	Motion      float64 `json:"motion"`
	Pulse       float64 `json:"pulse"`
	Density     float64 `json:"density"`
	Roughness   float64 `json:"rough"`
	Brightness  float64 `json:"bright"`
	Smoothness  float64 `json:"smooth"`
	Impact      float64 `json:"impact"`
	Pressure    float64 `json:"pressure"`
	Joy         float64 `json:"joy"`
	Melancholy  float64 `json:"melancholy"`
	Serenity    float64 `json:"serenity"`
	Swagger     float64 `json:"swagger"`
	Combat      float64 `json:"combat"`
	Sprint      float64 `json:"sprint"`
	SprintClean float64 `json:"sprintClean"`
	Dreaminess  float64 `json:"dreaminess"`
	Label       string  `json:"label"`
}

type Debug struct {
	CleanBright       float64       `json:"cleanBright,omitempty"`
	CleanParty        float64       `json:"cleanParty,omitempty"`
	EdgeDrive         float64       `json:"edgeDrive,omitempty"`
	TextureConfidence float64       `json:"textureConfidence,omitempty"`
	Intimacy          float64       `json:"intimacy,omitempty"`
	RoughRaw          float64       `json:"roughRaw,omitempty"`
	CombatRaw         float64       `json:"combatRaw,omitempty"`
	JoyRaw            float64       `json:"joyRaw,omitempty"`
	JoyCombatCut      float64       `json:"joyCombatCut,omitempty"`
	JoyRoughCut       float64       `json:"joyRoughCut,omitempty"`
	JoyPressureCut    float64       `json:"joyPressureCut,omitempty"`
	JoyEdgeCut        float64       `json:"joyEdgeCut,omitempty"`
	JoyDirtyCut       float64       `json:"joyDirtyCut,omitempty"`
	JoyCleanBoost     float64       `json:"joyCleanBoost,omitempty"`
	MelancholyRaw     float64       `json:"melancholyRaw,omitempty"`
	SerenityRaw       float64       `json:"serenityRaw,omitempty"`
	DirtyElectro      float64       `json:"dirtyElectro,omitempty"`
	HeavyDrive        float64       `json:"heavyDrive,omitempty"`
	TensePressure     float64       `json:"tensePressure,omitempty"`
	WarmGroove        float64       `json:"warmGroove,omitempty"`
	SereneBright      float64       `json:"sereneBright,omitempty"`
	JoyConfidence     float64       `json:"joyConfidence,omitempty"`
	VocalGrief        float64       `json:"vocalGrief,omitempty"`
	DramaticArc       float64       `json:"dramaticArc,omitempty"`
	Decision          DecisionAudit `json:"decision,omitempty"`
	TopLabels         []LabelScore  `json:"topLabels,omitempty"`
}

type LabelScore struct {
	Label  string  `json:"label"`
	Score  float64 `json:"score"`
	Passed bool    `json:"passed"`
	Reason string  `json:"reason,omitempty"`
}

func Compute(t library.Track, n Normalizer) Result {
	return ComputeFromInputs(Inputs{
		Dance:             t.Danceability,
		Valence:           t.Valence,
		Happy:             t.Happy,
		Sad:               t.Sad,
		Relaxed:           t.Relaxed,
		Party:             t.Party,
		Aggressive:        t.Aggressive,
		Acousticness:      t.Acousticness,
		Electronicness:    t.Electronicness,
		Instrumentalness:  t.Instrumentalness,
		Vocalness:         t.Vocalness,
		Brightness:        t.TimbreBrightness,
		Tonality:          t.Tonality,
		Melodicness:       t.Melodicness,
		Softness:          t.Softness,
		Heaviness:         t.Heaviness,
		Dreaminess:        t.Dreaminess,
		Emotionality:      t.Emotionality,
		BPM:               t.Tempo,
		BPMPerceived:      t.BPMPerceived,
		TempoConfidence:   t.TempoConfidence,
		TempoStability:    t.TempoStability,
		Loudness:          t.Loudness,
		RMS:               t.RMS,
		ZeroCrossingRate:  t.ZeroCrossingRate,
		SpectralCentroid:  t.SpectralCentroid,
		SpectralFlatness:  t.SpectralFlatness,
		SpectralRolloff85: t.SpectralRolloff85,
		SpectralFlux:      t.SpectralFlux,
		OnsetRate:         t.OnsetRate,
		DynamicRange:      t.DynamicRange,
		LowBandRatio:      t.LowBandRatio,
		MidBandRatio:      t.MidBandRatio,
		HighBandRatio:     t.HighBandRatio,
	}, n)
}

func inputTextureConfidence(in Inputs, n Normalizer) float64 {
	present := 0.0
	total := 0.0
	add := func(name string, value float64) {
		total++
		if value != 0 {
			present++
		}
		if n != nil {
			present += 0.5 * n.Reliability(name)
			total += 0.5
		}
	}
	add("Loudness", in.Loudness)
	add("RMS", in.RMS)
	add("ZeroCrossingRate", in.ZeroCrossingRate)
	add("SpectralCentroid", in.SpectralCentroid)
	add("SpectralFlatness", in.SpectralFlatness)
	add("SpectralFlux", in.SpectralFlux)
	add("OnsetRate", in.OnsetRate)
	if total <= 0 {
		return 0
	}
	return clamp01(present / total)
}

func ComputeFromInputs(in Inputs, n Normalizer) Result {
	return ComputeFromInputsWithTuning(in, n, DefaultTuning())
}

func ComputeFromInputsWithTuning(in Inputs, n Normalizer, tuning Tuning) Result {
	cfg := tuning.Sanitized()
	raw := basisInputs{
		dance:          normWeighted(n, "Danceability", in.Dance),
		valence:        normWeighted(n, "Valence", in.Valence),
		happy:          normWeighted(n, "Happy", in.Happy),
		sad:            normWeighted(n, "Sad", in.Sad),
		relaxed:        normWeighted(n, "Relaxed", in.Relaxed),
		party:          normWeighted(n, "Party", in.Party),
		aggressive:     normWeighted(n, "Aggressive", in.Aggressive),
		acousticness:   normWeighted(n, "Acousticness", in.Acousticness),
		electronicness: normWeighted(n, "Electronicness", in.Electronicness),
		instrumental:   normWeighted(n, "Instrumentalness", in.Instrumentalness),
		vocalness:      normWeighted(n, "Vocalness", in.Vocalness),
		brightness:     normWeighted(n, "TimbreBrightness", in.Brightness),
		tonality:       normWeighted(n, "Tonality", in.Tonality),
		melodicness:    normWeighted(n, "Melodicness", in.Melodicness),
		softness:       normWeighted(n, "Softness", in.Softness),
		heaviness:      normWeighted(n, "Heaviness", in.Heaviness),
		dreaminess:     normWeighted(n, "Dreaminess", in.Dreaminess),
		emotionality:   normWeighted(n, "Emotionality", in.Emotionality),
		tempo:          tempoValue(in),
		loudness:       normWeighted(n, "Loudness", in.Loudness),
		rms:            normWeighted(n, "RMS", in.RMS),
		zcr:            normWeighted(n, "ZeroCrossingRate", in.ZeroCrossingRate),
		centroid:       normWeighted(n, "SpectralCentroid", in.SpectralCentroid),
		flatness:       normWeighted(n, "SpectralFlatness", in.SpectralFlatness),
		rolloff:        normWeighted(n, "SpectralRolloff85", in.SpectralRolloff85),
		flux:           normWeighted(n, "SpectralFlux", in.SpectralFlux),
		onset:          normWeighted(n, "OnsetRate", in.OnsetRate),
		dynamicRange:   normWeighted(n, "DynamicRange", in.DynamicRange),
		lowBand:        normWeighted(n, "LowBandRatio", in.LowBandRatio),
		midBand:        normWeighted(n, "MidBandRatio", in.MidBandRatio),
		highBand:       normWeighted(n, "HighBandRatio", in.HighBandRatio),
	}

	dance := raw.dance
	party := raw.party
	relax := raw.relaxed
	electronic := raw.electronicness
	aggr := raw.aggressive
	zcrN := raw.zcr
	centN := raw.centroid
	flatN := raw.flatness
	rollN := raw.rolloff
	fluxN := raw.flux
	onsetN := raw.onset
	textureConfidence := inputTextureConfidence(in, n)
	density := clamp01(cfg.DensityLoudness*raw.loudness + cfg.DensityRMS*raw.rms + cfg.DensityDynamic*raw.dynamicRange + cfg.DensityLowBand*raw.lowBand - cfg.DensityHighPenalty*raw.highBand)
	groove := tempoValue(in)
	sprintTempo := clamp01(0.62*tempoValue(in) + 0.38*onsetN)
	motion := clamp01(cfg.MotionDance*dance + cfg.MotionGroove*groove + cfg.MotionParty*party + cfg.MotionSprintTempo*sprintTempo + cfg.MotionDensity*density)
	brightness := clamp01(cfg.BrightRolloff*rollN + cfg.BrightCentroid*centN + cfg.BrightTimbre*raw.brightness + cfg.BrightHighBand*raw.highBand + cfg.BrightValence*raw.valence)
	edgeDrive := clamp01(cfg.EdgeZCR*zcrN + cfg.EdgeCentroid*centN + cfg.EdgeDensity*density + cfg.EdgeElectronic*electronic + cfg.EdgeRelaxInv*(1.0-relax) + cfg.EdgeValInv*(1.0-raw.valence))
	roughRaw := clamp01(cfg.RoughZCR*zcrN + cfg.RoughFlatness*flatN + cfg.RoughFlux*fluxN + cfg.RoughAggressive*aggr + cfg.RoughRelaxInv*(1.0-relax) + cfg.RoughOnset*onsetN + cfg.RoughValInv*(1.0-raw.valence))
	cleanBright := clamp01(0.34*raw.valence + 0.22*brightness + 0.18*dance + 0.14*party + 0.12*relax)
	roughness := clamp01(roughRaw - cfg.RoughCleanCut*cleanBright)
	if edgeDrive >= 0.48 && cleanBright < 0.58 {
		roughness = math.Max(roughness, clamp01(edgeDrive-0.12*cleanBright))
	}
	impact := clamp01(cfg.ImpactDensity*density + cfg.ImpactLowBand*raw.lowBand + cfg.ImpactOnset*onsetN + cfg.ImpactRelaxInv*(1.0-relax) + cfg.ImpactParty*party)
	pressure := clamp01(cfg.PressureImpact*impact + cfg.PressureRough*roughness + cfg.PressureMotion*motion + cfg.PressureSprint*sprintTempo + cfg.PressureRelaxInv*(1.0-relax))
	if edgeDrive >= 0.50 {
		pressure = math.Max(pressure, clamp01(0.52*edgeDrive+0.28*impact+0.20*roughness))
	}
	smoothness := clamp01(cfg.SmoothRelax*relax + cfg.SmoothFlatInv*(1.0-flatN) + cfg.SmoothZCRInv*(1.0-zcrN) + cfg.SmoothFluxInv*(1.0-fluxN) + cfg.SmoothOnsetInv*(1.0-onsetN) + cfg.SmoothInstrument*raw.instrumental)
	smoothness = clamp01(smoothness - cfg.SmoothRoughCut*roughness - cfg.SmoothImpactCut*impact)
	heavyDrive := clamp01(0.40*raw.lowBand + 0.24*raw.dynamicRange + 0.20*onsetN + 0.10*fluxN + 0.06*density)
	combatRaw := clamp01(0.34*(0.30*roughness+0.24*impact+0.18*pressure+0.12*onsetN+0.10*(1.0-smoothness)+0.06*(1.0-raw.valence)) + 0.24*roughness + 0.18*pressure + 0.12*impact + 0.08*aggr + 0.04*(1.0-raw.valence))
	joyRaw := clamp01(0.30*raw.valence + 0.20*cleanBright + 0.16*brightness + 0.14*dance + 0.10*party + 0.06*groove + 0.04*raw.happy)
	intimacy := clamp01(0.28*raw.vocalness + 0.22*raw.dynamicRange + 0.18*(1.0-density) + 0.14*(1.0-roughness) + 0.10*(1.0-party) + 0.08*(1.0-brightness))
	melancholyRaw := clamp01(0.30*(1.0-raw.valence) + 0.20*intimacy + 0.16*(1.0-brightness) + 0.12*raw.vocalness + 0.10*smoothness + 0.08*raw.sad + 0.04*raw.dynamicRange)
	warmGroove := clamp01(0.24*clamp01(0.30*smoothness+0.20*relax+0.16*(1.0-pressure)+0.12*(1.0-party)+0.08*(1.0-roughness)+0.08*raw.lowBand+0.06*raw.instrumental) + 0.22*relax + 0.18*groove + 0.14*dance + 0.10*raw.vocalness + 0.08*raw.lowBand + 0.04*(1.0-roughness) - 0.18*melancholyRaw - 0.08*(1.0-raw.valence))
	sereneBright := clamp01(0.30*clamp01(0.30*smoothness+0.20*relax+0.16*(1.0-pressure)+0.12*(1.0-party)+0.08*(1.0-roughness)+0.08*raw.lowBand+0.06*raw.instrumental) + 0.24*smoothness + 0.18*brightness + 0.12*raw.instrumental + 0.10*(1.0-pressure) + 0.06*(1.0-roughness))
	joyConfidence := clamp01(0.35*raw.valence + 0.25*cleanBright + 0.20*(1.0-roughness) + 0.20*(1.0-combatRaw))
	cleanParty := clamp01(0.30*joyRaw + 0.24*cleanBright + 0.20*joyConfidence + 0.14*dance + 0.08*raw.valence + 0.04*(1.0-combatRaw))
	dirtyElectro := clamp01(0.28*roughness + 0.24*pressure + 0.18*electronic + 0.14*fluxN + 0.10*onsetN + 0.06*density)
	dirtyElectro = clamp01(dirtyElectro - 0.18*cleanParty)
	if electronic > 0.35 && fluxN > 0.60 && pressure > 0.42 {
		dirtyElectro = math.Max(dirtyElectro, 0.48)
	}
	if edgeDrive >= 0.54 && electronic >= 0.30 && cleanBright < 0.56 {
		dirtyElectro = math.Max(dirtyElectro, clamp01(0.58*edgeDrive+0.24*electronic+0.18*pressure))
	}
	serenity := clamp01(0.30*smoothness + 0.20*relax + 0.16*(1.0-pressure) + 0.12*(1.0-party) + 0.08*(1.0-roughness) + 0.08*raw.lowBand + 0.06*raw.instrumental)
	tensePressure := clamp01(0.28*pressure + 0.24*impact + 0.18*onsetN + 0.12*raw.vocalness + 0.10*density + 0.08*(1.0-serenity))
	combat := clamp01(combatRaw - cfg.CombatJoyCut*clamp01(joyRaw))
	if roughness >= 0.50 && pressure >= 0.42 {
		combat = math.Max(combat, clamp01(0.50*roughness+0.50*pressure))
	}
	if edgeDrive >= 0.56 && pressure >= 0.40 && cleanBright < 0.58 {
		combat = math.Max(combat, clamp01(0.50*edgeDrive+0.28*pressure+0.22*roughness))
	}
	if cleanBright >= 0.60 && joyConfidence >= 0.58 && raw.valence >= 0.55 {
		combat *= 0.78
	}
	cleanJoyShield := clamp01(0.34*cleanBright + 0.34*cleanParty + 0.32*joyConfidence)
	joyCombatCut := cfg.JoyCombatCut * combatRaw * (1.0 - 0.25*cleanJoyShield)
	joyRoughCut := cfg.JoyRoughCut * roughness * (1.0 - 0.30*cleanJoyShield)
	joyPressureCut := cfg.JoyPressureCut * pressure * (1.0 - 0.22*cleanJoyShield)
	joyEdgeCut := cfg.JoyEdgeCut * edgeDrive * (1.0 - 0.25*cleanJoyShield)
	joyDirtyCut := cfg.JoyDirtyCut * dirtyElectro * (1.0 - 0.18*cleanJoyShield)
	joy := clamp01(joyRaw - joyCombatCut - joyRoughCut - joyPressureCut - joyEdgeCut - joyDirtyCut)
	joyCleanBoost := 0.0
	if cleanJoyShield >= 0.56 && pressure < 0.60 && dirtyElectro < 0.50 {
		boosted := clamp01(0.62*joyRaw + 0.38*cleanBright - 0.12*combatRaw)
		if boosted > joy {
			joyCleanBoost = boosted - joy
			joy = boosted
		}
	}
	if combat >= cfg.JoyCombatFloor {
		joy = math.Min(joy, 0.44)
	}
	if edgeDrive >= 0.60 && cleanBright < 0.60 {
		joy = math.Min(joy, 0.46)
	}
	melancholy := clamp01(melancholyRaw - cfg.MelWarmCut*warmGroove - cfg.MelJoyCut*joy)
	if warmGroove >= 0.64 && serenity >= 0.58 && pressure < 0.38 {
		melancholy = clamp01(melancholy - 0.22*warmGroove)
	}
	vocalGrief := clamp01(0.28*raw.vocalness + 0.22*intimacy + 0.18*(1.0-brightness) + 0.14*(1.0-roughness) + 0.10*raw.dynamicRange + 0.08*(1.0-raw.valence))
	if vocalGrief >= 0.66 && combat < 0.30 && joy < 0.45 {
		melancholy = math.Max(melancholy, 0.56)
	}
	dramaticArc := clamp01(0.22*raw.dynamicRange + 0.20*math.Abs(joy-melancholy) + 0.18*math.Abs(serenity-pressure) + 0.16*fluxN + 0.14*onsetN + 0.10*raw.vocalness)
	tensePressure = clamp01(tensePressure)
	swagger := clamp01(cfg.SwaggerMotion*motion + cfg.SwaggerGroove*groove + cfg.SwaggerParty*party + cfg.SwaggerVocal*raw.vocalness + cfg.SwaggerDensity*density + cfg.SwaggerMelInv*(1.0-melancholy) + cfg.SwaggerValence*raw.valence)
	swagger = clamp01(swagger - 0.20*combat)
	sprint := clamp01(cfg.SprintTempo*sprintTempo + cfg.SprintMotion*motion + cfg.SprintDensity*density + cfg.SprintElectronic*electronic + cfg.SprintParty*party + cfg.SprintSerenityInv*(1.0-serenity))
	sprintClean := clamp01(sprint - cfg.SprintCombatCut*combat - cfg.SprintRoughCut*roughness)

	basis := Basis{Motion: motion, Pulse: groove, Density: density, Roughness: roughness, Brightness: brightness, Smoothness: smoothness, Impact: impact, Pressure: pressure, Joy: joy, Melancholy: melancholy, Serenity: serenity, Swagger: swagger, Combat: combat, Sprint: sprint, SprintClean: sprintClean, Dreaminess: raw.dreaminess}
	debug := Debug{
		CleanBright:       clamp01(cleanBright),
		CleanParty:        clamp01(cleanParty),
		EdgeDrive:         clamp01(edgeDrive),
		TextureConfidence: clamp01(textureConfidence),
		Intimacy:          clamp01(intimacy),
		RoughRaw:          clamp01(roughRaw),
		CombatRaw:         clamp01(combatRaw),
		JoyRaw:            clamp01(joyRaw),
		JoyCombatCut:      clamp01(joyCombatCut),
		JoyRoughCut:       clamp01(joyRoughCut),
		JoyPressureCut:    clamp01(joyPressureCut),
		JoyEdgeCut:        clamp01(joyEdgeCut),
		JoyDirtyCut:       clamp01(joyDirtyCut),
		JoyCleanBoost:     clamp01(joyCleanBoost),
		MelancholyRaw:     clamp01(melancholyRaw),
		SerenityRaw:       clamp01(serenity),
		DirtyElectro:      clamp01(dirtyElectro),
		HeavyDrive:        clamp01(heavyDrive),
		TensePressure:     clamp01(tensePressure),
		WarmGroove:        clamp01(warmGroove),
		SereneBright:      clamp01(sereneBright),
		JoyConfidence:     clamp01(joyConfidence),
		VocalGrief:        clamp01(vocalGrief),
		DramaticArc:       clamp01(dramaticArc),
	}
	decision := DecideLabel(basis, debug, cfg)
	basis.Label = decision.Winner
	debug.Decision = decision
	debug.TopLabels = TopLabelScoresWithDebug(basis, debug)
	return Result{Basis: basis, Debug: debug}
}

// Label is a fallback for callers without Debug.
// Production code should prefer Compute/ComputeFromInputs and Basis.Label.
func Label(b Basis) string {
	if b.Label != "" {
		return b.Label
	}
	return fallbackScoreLabel(b)
}

func LabelWithDebug(b Basis, d Debug) string {
	return DecideLabel(b, d, DefaultTuning()).Winner
}

func TopLabelScores(b Basis) []LabelScore { return TopLabelScoresWithDebug(b, Debug{}) }

func TopLabelScoresWithDebug(b Basis, d Debug) []LabelScore {
	audit := DecideLabel(b, d, DefaultTuning())
	scores := make([]LabelScore, 0, len(audit.Results))
	for _, r := range audit.Results {
		scores = append(scores, LabelScore{Label: r.Label, Score: r.Score, Passed: r.Passed, Reason: firstReason(r)})
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score == scores[j].Score {
			return scores[i].Label < scores[j].Label
		}
		return scores[i].Score > scores[j].Score
	})
	return scores[:minInt(5, len(scores))]
}

func firstReason(r DecisionResult) string {
	if len(r.Failed) > 0 {
		return r.Failed[0]
	}
	if len(r.Penalties) > 0 {
		return r.Penalties[0]
	}
	if len(r.Reasons) > 0 {
		return r.Reasons[0]
	}
	return ""
}

func IsDramaticArc(b Basis) bool                   { return dramaticArcScore(b) > 0.58 }
func IsDramaticArcWithDebug(b Basis, d Debug) bool { return IsDramaticArc(b) && d.DramaticArc >= 0.58 }
func fallbackScoreLabel(b Basis) string {
	if b.Density >= 0.52 && b.Combat >= 0.46 && combatForceScore(b) >= 0.42 {
		return "combat_force"
	}
	if b.Pressure >= 0.50 && pressureScore(b) >= 0.42 {
		return "tense_pressure"
	}
	if b.Roughness >= 0.50 && b.Combat >= 0.46 && dirtyElectroCombatScore(b, Debug{}) >= 0.42 {
		return "dirty_electro_combat"
	}
	if b.Joy >= 0.36 && joyPartyScore(b) >= 0.42 && b.Pressure < 0.52 && b.Combat < 0.28 {
		return "joy_party"
	}
	if b.Joy >= 0.34 && joyFunkScore(b) >= 0.40 && b.Smoothness >= 0.34 && b.Pressure < 0.48 {
		return "joy_funk"
	}
	if b.SprintClean >= 0.44 && b.Pressure < 0.50 && upliftDriveScore(b) >= 0.42 {
		return "uplift_drive"
	}
	scores := []LabelScore{
		{Label: "combat_force", Score: combatForceScore(b)},
		{Label: "tense_pressure", Score: pressureScore(b)},
		{Label: "joy_party", Score: joyPartyScore(b)},
		{Label: "joy_funk", Score: joyFunkScore(b)},
		{Label: "uplift_drive", Score: upliftDriveScore(b)},
		{Label: "serene_warm_groove", Score: sereneWarmGrooveScore(b)},
		{Label: "serene_bright", Score: sereneBrightScore(b)},
		{Label: "night_smooth", Score: nightSmoothScore(b, Debug{})},
		{Label: "melancholy_calm", Score: melancholyCalmScore(b)},
		{Label: "melancholy_grief", Score: melancholyGriefScore(b, Debug{})},
		{Label: "serene_calm", Score: sereneCalmScore(b)},
		{Label: "street_swagger", Score: streetSwaggerScore(b)},
		{Label: "dramatic_arc", Score: dramaticArcScore(b)},
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score == scores[j].Score {
			return scores[i].Label < scores[j].Label
		}
		return scores[i].Score > scores[j].Score
	})
	if scores[0].Score < 0.38 {
		return "neutral"
	}
	return scores[0].Label
}

func IsDirtyElectroCombat(b Basis) bool { return IsDirtyElectroCombatWithDebug(b, Debug{}) }
func IsDirtyElectroCombatWithDebug(b Basis, d Debug) bool {
	cleanParty := d.CleanBright >= 0.58 && d.JoyConfidence >= 0.60 && b.Joy >= 0.52 && b.Combat < 0.32
	return dirtyElectroCombatScore(b, d) > 0.55 && !cleanParty
}
func IsCombatForce(b Basis) bool { return combatForceScore(b) > 0.57 }
func IsCombatForceWithDebug(b Basis, d Debug) bool {
	return combatForceScore(b) > 0.55 && !(d.CleanBright >= 0.55 && d.JoyConfidence >= 0.55 && b.Joy >= 0.42 && b.Swagger >= 0.48 && d.DirtyElectro < 0.38)
}
func IsTensePressure(b Basis) bool { return pressureScore(b) > 0.57 }
func IsTensePressureWithDebug(b Basis, d Debug) bool {
	return pressureScore(b) > 0.57 && d.TensePressure >= 0.58
}
func IsJoyParty(b Basis) bool { return joyPartyScore(b) > 0.48 }
func IsJoyPartyWithDebug(b Basis, d Debug) bool {
	cfg := DefaultTuning()
	return IsJoyParty(b) && (d.JoyConfidence >= 0.50 || d.CleanParty >= cfg.JoyPartyMinClean || d.CleanBright >= 0.58) && b.Roughness < cfg.JoyPartyMaxRough && b.Pressure < cfg.JoyPartyMaxPressure && d.DirtyElectro < cfg.JoyPartyMaxDirty && d.EdgeDrive < cfg.JoyPartyMaxEdge
}
func IsJoyFunk(b Basis) bool { return joyFunkScore(b) > 0.56 }
func IsJoyFunkWithDebug(b Basis, d Debug) bool {
	return IsJoyFunk(b) && d.CleanBright >= 0.52 && d.JoyConfidence >= 0.50 && d.DirtyElectro < 0.32 && b.Pressure < 0.60
}
func IsUpliftDrive(b Basis) bool { return upliftDriveScore(b) > 0.52 }
func IsUpliftDriveWithDebug(b Basis, d Debug) bool {
	return IsUpliftDrive(b) && b.SprintClean >= 0.40 && b.Joy >= 0.40 && b.Pressure < 0.62 && d.DirtyElectro < 0.52
}
func IsSereneWarmGroove(b Basis) bool { return sereneWarmGrooveScore(b) > 0.58 }
func IsSereneWarmGrooveWithDebug(b Basis, d Debug) bool {
	return IsSereneWarmGroove(b) && d.WarmGroove >= 0.55 && b.Serenity >= 0.46 && b.Joy < 0.52 && b.Pressure < 0.58
}
func IsSereneBright(b Basis) bool { return sereneBrightScore(b) > 0.56 }
func IsSereneBrightWithDebug(b Basis, d Debug) bool {
	return IsSereneBright(b) && d.SereneBright >= 0.60 && b.Serenity < 0.72 && b.Smoothness >= 0.52
}
func IsNightSmooth(b Basis) bool { return nightSmoothScore(b, Debug{}) > 0.54 }
func IsNightSmoothWithDebug(b Basis, d Debug) bool {
	return nightSmoothScore(b, d) > 0.54 && d.WarmGroove >= 0.56
}
func IsMelancholyCalm(b Basis) bool { return melancholyCalmScore(b) > 0.58 }
func IsMelancholyCalmWithDebug(b Basis, d Debug) bool {
	return IsMelancholyCalm(b) && d.WarmGroove < 0.64
}
func IsMelancholyPressure(b Basis) bool { return melancholyPressureScore(b) > 0.54 }
func IsMelancholyPressureWithDebug(b Basis, d Debug) bool {
	return IsMelancholyPressure(b) && b.Pressure >= 0.32 && b.Joy < 0.56 && b.Serenity < 0.68
}
func IsMelancholyGrief(b Basis) bool { return melancholyGriefScore(b, Debug{}) > 0.52 }
func IsMelancholyGriefWithDebug(b Basis, d Debug) bool {
	return melancholyGriefScore(b, d) > 0.52 && (d.VocalGrief >= 0.55 || b.VocalGrief() >= 0.55)
}
func IsSereneCalm(b Basis) bool { return sereneCalmScore(b) > 0.58 }
func IsSereneCalmWithDebug(b Basis, d Debug) bool {
	return IsSereneCalm(b) && d.WarmGroove < 0.66 && b.Pressure < 0.44
}
func IsStreetSwagger(b Basis) bool                   { return streetSwaggerScore(b) > 0.56 }
func IsStreetSwaggerWithDebug(b Basis, d Debug) bool { return IsStreetSwagger(b) && b.Joy < 0.48 }

func pressureScore(b Basis) float64 {
	return clamp01(0.46*b.Pressure + 0.30*b.Combat + 0.24*b.Roughness)
}
func combatForceScore(b Basis) float64 {
	return clamp01(0.62*b.Combat + 0.18*b.Pressure + 0.10*b.Roughness + 0.06*b.Sprint + 0.04*b.Impact)
}
func dirtyElectroCombatScore(b Basis, d Debug) float64 {
	return clamp01(0.56*b.Roughness + 0.22*b.Pressure + 0.12*b.Sprint + 0.06*b.Brightness + 0.04*d.DirtyElectro)
}
func joyPartyScore(b Basis) float64 {
	return clamp01(0.48*b.Joy + 0.24*b.Swagger + 0.12*b.Sprint + 0.18*b.Brightness + 0.06*b.Pulse - 0.12*b.Combat - 0.04*b.Pressure)
}
func joyFunkScore(b Basis) float64 {
	return clamp01(0.40*b.Joy + 0.26*b.Swagger + 0.20*b.Smoothness + 0.08*b.Motion + 0.08*b.Brightness + 0.04*b.Pulse - 0.08*b.Combat - 0.04*b.Density)
}
func upliftDriveScore(b Basis) float64 {
	return clamp01(0.54*b.SprintClean + 0.18*b.Motion + 0.16*b.Joy + 0.08*b.Swagger + 0.04*(1-b.Combat))
}
func sereneWarmGrooveScore(b Basis) float64 {
	return clamp01(0.40*b.Serenity + 0.22*b.Joy + 0.18*b.Smoothness + 0.12*b.Brightness + 0.06*b.Motion + 0.04*(1-b.Combat))
}
func sereneBrightScore(b Basis) float64 {
	return clamp01(0.42*b.Serenity + 0.24*b.Brightness + 0.16*b.Smoothness + 0.06*b.Joy + 0.08*(1-b.Roughness) + 0.04*(1-b.Combat))
}
func nightSmoothScore(b Basis, d Debug) float64 {
	identity := nightRootedIdentity(b)
	atmosphere := clamp01(
		0.30*(1-b.Brightness) +
			0.30*b.Melancholy +
			0.20*b.Serenity +
			0.20*b.Smoothness,
	)
	return clamp01(0.55*identity + 0.45*atmosphere)
}
func melancholyCalmScore(b Basis) float64 {
	return clamp01(0.40*b.Melancholy + 0.24*b.Serenity + 0.16*b.Smoothness + 0.10*(1-b.Pressure) + 0.10*(1-b.Joy))
}
func melancholyGriefScore(b Basis, d Debug) float64 {
	return clamp01(0.46*b.Melancholy + 0.16*clamp01(0.5*b.Melancholy+0.3*(1-b.Joy)+0.2*b.Serenity) + 0.16*(1-b.Joy) + 0.10*b.Dreaminess + 0.08*(1-b.Serenity) + 0.04*d.VocalGrief)
}
func melancholyPressureScore(b Basis) float64 {
	return clamp01(0.30*b.Melancholy + 0.34*b.Pressure + 0.12*b.Combat + 0.12*(1-b.Joy) + 0.12*b.Roughness)
}
func sereneCalmScore(b Basis) float64 {
	return clamp01(0.44*b.Serenity + 0.16*b.Smoothness + 0.10*(1-b.Pressure) + 0.10*(1-b.Roughness) + 0.08*b.Joy)
}
func streetSwaggerScore(b Basis) float64 {
	return clamp01(0.36*b.Swagger + 0.14*b.Joy + 0.22*b.Combat + 0.16*b.Motion + 0.12*b.Brightness)
}
func dramaticArcScore(b Basis) float64 {
	return clamp01(0.34*b.Combat + 0.20*b.Pressure + 0.18*b.Melancholy + 0.16*b.Impact + 0.12*b.Sprint)
}

func (b Basis) VocalGrief() float64 {
	return clamp01(0.5*b.Melancholy + 0.3*(1-b.Joy) + 0.2*b.Serenity)
}

func fallbackNorm(name string, v float64) float64 {
	switch name {
	case "Loudness":
		return clamp01((v + 60.0) / 35.0)
	case "RMS":
		return clamp01((v - 0.05) / 0.45)
	case "ZeroCrossingRate":
		return clamp01(v / 0.22)
	case "SpectralCentroid":
		return clamp01((v - 200.0) / 5000.0)
	case "SpectralRolloff85":
		return clamp01((v - 100.0) / 6000.0)
	case "SpectralFlatness":
		return clamp01(v / 0.08)
	case "SpectralFlux":
		return clamp01(v / 0.45)
	case "OnsetRate":
		return clamp01(v / 3.0)
	case "DynamicRange", "LowBandRatio", "MidBandRatio", "HighBandRatio":
		return clamp01(v)
	default:
		return clamp01(v)
	}
}

func norm(n Normalizer, name string, value float64) float64 {
	if n == nil {
		return fallbackNorm(name, value)
	}
	return clamp01(n.Norm(name, value))
}
func normWeighted(n Normalizer, name string, value float64) float64 {
	if n == nil {
		return fallbackNorm(name, value)
	}
	return clamp01(n.NormWeighted(name, value))
}

func tempoValue(in Inputs) float64 {
	bpm := in.BPMPerceived
	if bpm <= 0 {
		bpm = in.BPM
	}
	trust := tempoTrust(in)
	bestGroove := 0.0
	bestSprint := 0.0
	for _, x := range tempoCandidates(bpm) {
		bestGroove = math.Max(bestGroove, bell(x, 112, 40))
		bestSprint = math.Max(bestSprint, bell(x, 138, 42))
	}
	return clamp01(0.55*bestGroove*clamp(trust, 0.35, 1.0) + 0.45*bestSprint*clamp(trust, 0.35, 1.0))
}

func tempoTrust(in Inputs) float64 {
	if in.TempoConfidence <= 0 && in.TempoStability <= 0 {
		return 0.35
	}
	return clamp01(0.65*in.TempoConfidence + 0.35*in.TempoStability)
}
func tempoCandidates(bpm float64) []float64 {
	if bpm <= 0 {
		return nil
	}
	out := []float64{bpm}
	if bpm < 95 {
		out = append(out, bpm*2)
	}
	if bpm > 150 {
		out = append(out, bpm/2)
	}
	return out
}
func bell(x, center, width float64) float64 { return clamp01(1 - math.Abs(x-center)/width) }
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

type basisInputs struct {
	dance, valence, happy, sad, relaxed, party, aggressive, acousticness, electronicness, instrumental, vocalness, brightness, tonality, melodicness, softness, heaviness, dreaminess, emotionality, tempo, loudness, rms, zcr, centroid, flatness, rolloff, flux, onset, dynamicRange, lowBand, midBand, highBand float64
}

func (b basisInputs) BPMTrust() float64 { return clamp01(0.65*b.tempo + 0.35*b.onset) }

func r3(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	return math.Round(x*1000) / 1000
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
