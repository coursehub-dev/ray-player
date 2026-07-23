package emotion

import (
	"fmt"
	"sort"
)

type LabelID string

const (
	LabelDirtyElectroCombat LabelID = "dirty_electro_combat"
	LabelCombatForce        LabelID = "combat_force"
	LabelTensePressure      LabelID = "tense_pressure"
	LabelMelancholyPressure LabelID = "melancholy_pressure"
	LabelDramaticArc        LabelID = "dramatic_arc"
	LabelJoyFunk            LabelID = "joy_funk"
	LabelUpliftDrive        LabelID = "uplift_drive"
	LabelJoyParty           LabelID = "joy_party"
	LabelSereneWarmGroove   LabelID = "serene_warm_groove"
	LabelNightSmooth        LabelID = "night_smooth"
	LabelSereneBright       LabelID = "serene_bright"
	LabelMelancholyGrief    LabelID = "melancholy_grief"
	LabelMelancholyCalm     LabelID = "melancholy_calm"
	LabelSereneCalm         LabelID = "serene_calm"
	LabelStreetSwagger      LabelID = "street_swagger"
	LabelNeutral            LabelID = "neutral"
)

type DecisionFamily string

const (
	FamilyDanger     DecisionFamily = "danger"
	FamilyPressure   DecisionFamily = "pressure"
	FamilyPositive   DecisionFamily = "positive"
	FamilyWarm       DecisionFamily = "warm"
	FamilyMelancholy DecisionFamily = "melancholy"
	FamilyCalm       DecisionFamily = "calm"
	FamilyMixed      DecisionFamily = "mixed"
	FamilyNeutral    DecisionFamily = "neutral"
)

type DecisionNode struct {
	Label    LabelID
	Family   DecisionFamily
	Priority int
	Score    func(Basis, Debug, Tuning) float64
	Gates    []DecisionGate
	Penalty  []DecisionPenalty
}

type DecisionGate struct {
	Name  string
	Check func(Basis, Debug, Tuning) (bool, string)
}

type DecisionPenalty struct {
	Name  string
	Value func(Basis, Debug, Tuning) (float64, string)
}

type DecisionResult struct {
	Label     string   `json:"label"`
	Score     float64  `json:"score"`
	RawScore  float64  `json:"rawScore"`
	Passed    bool     `json:"passed"`
	Family    string   `json:"family,omitempty"`
	Priority  int      `json:"priority,omitempty"`
	Reasons   []string `json:"reasons,omitempty"`
	Failed    []string `json:"failed,omitempty"`
	Penalties []string `json:"penalties,omitempty"`
	Sanity    []string `json:"sanity,omitempty"`
	BlockedBy []string `json:"blockedBy,omitempty"`
}

type DecisionAudit struct {
	Winner       string           `json:"winner"`
	WinnerScore  float64          `json:"winnerScore"`
	WinnerTrace  string           `json:"winnerTrace,omitempty"`
	Family       string           `json:"family,omitempty"`
	Results      []DecisionResult `json:"results"`
	Sanity       []string         `json:"sanity,omitempty"`
	AxisWarnings []string         `json:"axisWarnings,omitempty"`
}

func DecideLabel(b Basis, d Debug, cfg Tuning) DecisionAudit {
	nodes := DefaultDecisionDAG()
	results := make([]DecisionResult, 0, len(nodes))
	globalSanity := globalSanityWarnings(b, d)

	for _, node := range nodes {
		results = append(results, evalDecisionNode(node, b, d, cfg))
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Passed != results[j].Passed {
			return results[i].Passed
		}
		if results[i].Score == results[j].Score {
			return results[i].Priority < results[j].Priority
		}
		return results[i].Score > results[j].Score
	})

	winner := DecisionResult{
		Label:    string(LabelNeutral),
		Score:    0,
		RawScore: 0,
		Passed:   true,
		Family:   string(FamilyNeutral),
		Priority: 999,
		Reasons:  []string{"no confident label passed"},
	}
	if len(results) > 0 && results[0].Passed && results[0].Score >= 0.34 {
		winner = results[0]
	}

	return DecisionAudit{
		Winner:       winner.Label,
		WinnerScore:  winner.Score,
		WinnerTrace:  fmt.Sprintf("%s score=%.3f raw=%.3f", winner.Label, winner.Score, winner.RawScore),
		Family:       winner.Family,
		Results:      results,
		Sanity:       globalSanity,
		AxisWarnings: axisWarnings(b, d),
	}
}

func evalDecisionNode(node DecisionNode, b Basis, d Debug, cfg Tuning) DecisionResult {
	raw := clamp01(node.Score(b, d, cfg))
	score := raw
	res := DecisionResult{Label: string(node.Label), Family: string(node.Family), Priority: node.Priority, RawScore: raw, Score: raw, Passed: true}

	for _, gate := range node.Gates {
		ok, reason := gate.Check(b, d, cfg)
		if ok {
			res.Reasons = append(res.Reasons, "pass "+gate.Name)
			continue
		}
		res.Passed = false
		res.Failed = append(res.Failed, gate.Name+": "+reason)
	}

	for _, p := range node.Penalty {
		v, reason := p.Value(b, d, cfg)
		v = clamp01(v)
		if v <= 0 {
			continue
		}
		score = clamp01(score - v)
		res.Penalties = append(res.Penalties, fmt.Sprintf("%s -%.3f %s", p.Name, v, reason))
	}
	res.Score = score

	if res.Passed && score < 0.34 {
		res.Passed = false
		res.Failed = append(res.Failed, fmt.Sprintf("score %.3f < 0.34", score))
	}

	res.Sanity = labelSanityWarnings(LabelID(res.Label), b, d)
	if len(res.Sanity) > 0 {
		res.Passed = false
		res.BlockedBy = append(res.BlockedBy, res.Sanity...)
	}

	return res
}

func DefaultDecisionDAG() []DecisionNode {
	return []DecisionNode{
		dirtyElectroNode(),
		combatForceNode(),
		tensePressureNode(),
		melancholyPressureNode(),
		dramaticArcNode(),
		joyFunkNode(),
		upliftDriveNode(),
		joyPartyNode(),
		sereneWarmGrooveNode(),
		nightSmoothNode(),
		sereneBrightNode(),
		melancholyGriefNode(),
		melancholyCalmNode(),
		sereneCalmNode(),
		streetSwaggerNode(),
	}
}

func dirtyElectroNode() DecisionNode {
	return DecisionNode{
		Label: LabelDirtyElectroCombat, Family: FamilyDanger, Priority: 10,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return dirtyElectroCombatScore(b, d) },
		Gates: []DecisionGate{
			gate("dirtyElectro_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				heavyDirty := d.HeavyDrive >= 0.70 && b.Pressure >= 0.58 && b.Roughness >= 0.42 && d.DirtyElectro >= 0.37
				return d.DirtyElectro >= cfg.DirtyMinDirty || heavyDirty, fmt.Sprintf("dirtyElectro %.3f < %.3f heavyDrive=%.3f pressure=%.3f rough=%.3f", d.DirtyElectro, cfg.DirtyMinDirty, d.HeavyDrive, b.Pressure, b.Roughness)
			}),
			gate("pressure_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Pressure >= cfg.DirtyMinPressure, fmt.Sprintf("pressure %.3f < %.3f", b.Pressure, cfg.DirtyMinPressure)
			}),
			gate("roughness_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Roughness >= 0.36, fmt.Sprintf("roughness %.3f < 0.36", b.Roughness)
			}),
			gate("not_night_smooth", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				nightLike := d.WarmGroove >= 0.45 && b.Serenity >= 0.46 && d.VocalGrief >= 0.45 && d.EdgeDrive < 0.50
				return !nightLike, fmt.Sprintf("night-like warmGroove=%.3f serenity=%.3f vocalGrief=%.3f edgeDrive=%.3f", d.WarmGroove, b.Serenity, d.VocalGrief, d.EdgeDrive)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("clean_party_shield", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				if d.CleanParty >= 0.58 && d.JoyConfidence >= 0.58 && b.Joy >= 0.46 {
					return 0.18, "clean party evidence"
				}
				return 0, ""
			}),
		},
	}
}

func combatForceNode() DecisionNode {
	return DecisionNode{
		Label: LabelCombatForce, Family: FamilyDanger, Priority: 20,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return combatForceScore(b) },
		Gates: []DecisionGate{
			gate("combat_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Combat >= cfg.CombatMinCombat, fmt.Sprintf("combat %.3f < %.3f", b.Combat, cfg.CombatMinCombat)
			}),
			gate("pressure_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Pressure >= cfg.CombatMinPressure, fmt.Sprintf("pressure %.3f < %.3f", b.Pressure, cfg.CombatMinPressure)
			}),
			gate("roughness_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Roughness >= cfg.CombatMinRough, fmt.Sprintf("roughness %.3f < %.3f", b.Roughness, cfg.CombatMinRough)
			}),
			gate("not_clean_funk", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				heavyCombat := d.HeavyDrive >= 0.62 && b.Pressure >= 0.62 && b.Roughness >= 0.50
				nightLike := d.WarmGroove >= 0.40 && b.Serenity >= 0.46 && d.VocalGrief >= 0.50 && d.DirtyElectro < 0.52 && b.Combat < 0.56
				cleanFunk := d.CleanBright >= 0.52 &&
					d.CleanParty >= 0.56 &&
					d.JoyConfidence >= 0.50 &&
					b.Swagger >= 0.46 &&
					b.Joy >= 0.38 &&
					!heavyCombat &&
					!nightLike
				return !cleanFunk, fmt.Sprintf("clean funk shield cleanBright=%.3f cleanParty=%.3f joyConf=%.3f swagger=%.3f joy=%.3f heavyDrive=%.3f pressure=%.3f rough=%.3f nightLike=%v", d.CleanBright, d.CleanParty, d.JoyConfidence, b.Swagger, b.Joy, d.HeavyDrive, b.Pressure, b.Roughness, nightLike)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("night_noir_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				nightNoirHeavy := d.WarmGroove >= 0.40 &&
					b.Serenity >= 0.46 &&
					d.VocalGrief >= 0.50 &&
					d.DirtyElectro >= 0.48 && d.DirtyElectro < 0.52 &&
					d.EdgeDrive < 0.56 &&
					b.Combat >= 0.50 && b.Combat < 0.56 &&
					b.Roughness >= 0.48
				if nightNoirHeavy {
					return 0.10, fmt.Sprintf("night noir evidence warmGroove=%.3f serenity=%.3f vocalGrief=%.3f dirty=%.3f edge=%.3f combat=%.3f rough=%.3f", d.WarmGroove, b.Serenity, d.VocalGrief, d.DirtyElectro, d.EdgeDrive, b.Combat, b.Roughness)
				}
				return 0, ""
			}),
			penalty("clean_funk_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				heavyCombat := d.HeavyDrive >= 0.62 && b.Pressure >= 0.62 && b.Roughness >= 0.50
				cleanFunk := b.Swagger >= 0.46 &&
					b.Joy >= 0.40 &&
					d.CleanParty >= 0.56 &&
					d.CleanBright >= 0.52 &&
					d.JoyConfidence >= 0.48 &&
					d.DirtyElectro < 0.52 &&
					d.EdgeDrive < 0.56 &&
					!heavyCombat

				if cleanFunk {
					return 0.20, fmt.Sprintf("clean funk evidence swagger=%.3f joy=%.3f cleanParty=%.3f cleanBright=%.3f joyConf=%.3f dirty=%.3f edge=%.3f heavyDrive=%.3f", b.Swagger, b.Joy, d.CleanParty, d.CleanBright, d.JoyConfidence, d.DirtyElectro, d.EdgeDrive, d.HeavyDrive)
				}
				return 0, ""
			}),
			penalty("clean_positive_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				heavyCombat := d.HeavyDrive >= 0.62 && b.Pressure >= 0.62 && b.Roughness >= 0.50
				cleanPositive := (d.CleanBright >= 0.52 || d.CleanParty >= 0.52 || d.JoyConfidence >= 0.56) &&
					b.Motion >= 0.52 &&
					b.Joy >= 0.30 &&
					d.DirtyElectro < 0.50 &&
					d.EdgeDrive < 0.56 &&
					!heavyCombat

				if cleanPositive {
					return 0.16, fmt.Sprintf(
						"clean positive evidence cleanBright=%.3f cleanParty=%.3f joyConf=%.3f motion=%.3f joy=%.3f dirty=%.3f edge=%.3f heavyDrive=%.3f",
						d.CleanBright, d.CleanParty, d.JoyConfidence, b.Motion, b.Joy, d.DirtyElectro, d.EdgeDrive, d.HeavyDrive,
					)
				}
				return 0, ""
			}),
		},
	}
}

func tensePressureNode() DecisionNode {
	return DecisionNode{
		Label: LabelTensePressure, Family: FamilyPressure, Priority: 30,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return pressureScore(b) },
		Gates: []DecisionGate{
			gate("tensePressure_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return d.TensePressure >= 0.50, fmt.Sprintf("tensePressure %.3f < 0.50", d.TensePressure)
			}),
			gate("pressure_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Pressure >= 0.40, fmt.Sprintf("pressure %.3f < 0.40", b.Pressure)
			}),
			gate("not_dirty_electro", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return !(d.DirtyElectro >= cfg.DirtyMinDirty && b.Roughness >= 0.38 && b.Pressure >= cfg.DirtyMinPressure), "dirty electro evidence stronger"
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("full_combat_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				if b.Combat >= 0.50 && b.Roughness >= 0.48 && b.Pressure >= 0.50 {
					return 0.18, "full combat evidence"
				}
				return 0, ""
			}),
			penalty("heavy_combat_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				heavyCombat := d.HeavyDrive >= 0.62 && b.Combat >= 0.46 && b.Pressure >= 0.62 && b.Roughness >= 0.50
				if heavyCombat {
					return 0.08, fmt.Sprintf("heavy combat evidence heavyDrive=%.3f combat=%.3f pressure=%.3f rough=%.3f", d.HeavyDrive, b.Combat, b.Pressure, b.Roughness)
				}
				return 0, ""
			}),
			penalty("clean_funk_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				cleanFunkWindow := b.Swagger >= 0.46 &&
					b.Joy >= 0.42 &&
					b.Motion >= 0.50 &&
					d.CleanParty >= 0.56 &&
					d.CleanBright >= 0.50 && d.CleanBright < 0.62 &&
					d.JoyConfidence >= 0.48 && d.JoyConfidence < 0.56 &&
					d.DirtyElectro < 0.50

				if cleanFunkWindow {
					return 0.09, fmt.Sprintf(
						"clean funk window swagger=%.3f joy=%.3f motion=%.3f cleanParty=%.3f cleanBright=%.3f joyConf=%.3f dirty=%.3f",
						b.Swagger, b.Joy, b.Motion, d.CleanParty, d.CleanBright, d.JoyConfidence, d.DirtyElectro,
					)
				}
				return 0, ""
			}),
			penalty("clean_positive_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				cleanPositive := (d.CleanBright >= 0.50 || d.CleanParty >= 0.50 || d.JoyConfidence >= 0.55) &&
					b.Motion >= 0.50 &&
					d.DirtyElectro < 0.48 &&
					d.EdgeDrive < 0.54 &&
					b.Combat < 0.55

				if cleanPositive {
					return 0.14, fmt.Sprintf(
						"clean positive evidence cleanBright=%.3f cleanParty=%.3f joyConf=%.3f motion=%.3f combat=%.3f dirty=%.3f edge=%.3f",
						d.CleanBright, d.CleanParty, d.JoyConfidence, b.Motion, b.Combat, d.DirtyElectro, d.EdgeDrive,
					)
				}
				return 0, ""
			}),
		},
	}
}

func melancholyPressureNode() DecisionNode {
	return DecisionNode{
		Label: LabelMelancholyPressure, Family: FamilyPressure, Priority: 40,
		Score: func(b Basis, d Debug, cfg Tuning) float64 {
			return clamp01(0.34*b.Melancholy + 0.26*b.Pressure + 0.20*d.TensePressure + 0.12*b.Roughness + 0.08*d.VocalGrief)
		},
		Gates: []DecisionGate{
			gate("melancholy_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Melancholy >= 0.48, fmt.Sprintf("melancholy %.3f < 0.48", b.Melancholy)
			}),
			gate("pressure_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Pressure >= 0.42, fmt.Sprintf("pressure %.3f < 0.42", b.Pressure)
			}),
			gate("tensePressure_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return d.TensePressure >= 0.50, fmt.Sprintf("tensePressure %.3f < 0.50", d.TensePressure)
			}),
		},
	}
}

func dramaticArcNode() DecisionNode {
	return DecisionNode{
		Label: LabelDramaticArc, Family: FamilyMixed, Priority: 50,
		Score: func(b Basis, d Debug, cfg Tuning) float64 {
			oldScore := dramaticArcScore(b)
			mixedScore := dramaticMixedStateScore(b)
			return clamp01(0.35*oldScore + 0.65*mixedScore)
		},
		Gates: []DecisionGate{
			gate("dramaticArc_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				oldScore := dramaticArcScore(b)
				mixedScore := dramaticMixedStateScore(b)
				combined := 0.35*oldScore + 0.65*mixedScore
				return d.DramaticArc >= 0.50 || mixedScore >= 0.45 || combined >= 0.50, fmt.Sprintf("dramaticArc %.3f oldScore %.3f mixedScore %.3f combined %.3f", d.DramaticArc, oldScore, mixedScore, combined)
			}),
			gate("mixed_axes", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				mixed := b.Melancholy >= 0.42 && (b.Pressure >= 0.32 || b.Roughness >= 0.26) && b.Serenity >= 0.38
				return mixed, fmt.Sprintf("mel=%.3f pressure=%.3f rough=%.3f serenity=%.3f", b.Melancholy, b.Pressure, b.Roughness, b.Serenity)
			}),
			gate("not_pure_combat", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				pureCombat := b.Combat >= 0.58 && b.Serenity < 0.25
				return !pureCombat, fmt.Sprintf("combat=%.3f serenity=%.3f", b.Combat, b.Serenity)
			}),
			gate("not_pure_serene", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				pureSerene := b.Serenity >= 0.68 && b.Combat < 0.20 && b.Pressure < 0.25
				return !pureSerene, fmt.Sprintf("serenity=%.3f combat=%.3f pressure=%.3f", b.Serenity, b.Combat, b.Pressure)
			}),
		},
	}
}

func joyFunkNode() DecisionNode {
	return DecisionNode{
		Label: LabelJoyFunk, Family: FamilyPositive, Priority: 60,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return joyFunkScore(b) },
		Gates: []DecisionGate{
			gate("joy_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Joy >= 0.38, fmt.Sprintf("joy %.3f < 0.38", b.Joy)
			}),
			gate("swagger_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Swagger >= 0.44, fmt.Sprintf("swagger %.3f < 0.44", b.Swagger)
			}),
			gate("clean_evidence", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return d.CleanBright >= 0.44 || d.CleanParty >= 0.44 || d.JoyConfidence >= 0.52, fmt.Sprintf("cleanBright=%.3f cleanParty=%.3f joyConf=%.3f", d.CleanBright, d.CleanParty, d.JoyConfidence)
			}),
			gate("not_dirty_combat", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				trueDirtyCombat := b.Combat > 0.58 &&
					d.DirtyElectro > 0.50 &&
					d.EdgeDrive > 0.54 &&
					d.CleanParty < 0.56 &&
					d.CleanBright < 0.56

				return !trueDirtyCombat, fmt.Sprintf(
					"combat=%.3f dirty=%.3f edge=%.3f cleanParty=%.3f cleanBright=%.3f",
					b.Combat, d.DirtyElectro, d.EdgeDrive, d.CleanParty, d.CleanBright,
				)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("pure_party_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				pureParty := d.CleanParty >= 0.58 &&
					d.CleanBright >= 0.52 &&
					d.JoyConfidence >= 0.55 &&
					b.Swagger < 0.56 &&
					d.DirtyElectro < 0.46 &&
					d.EdgeDrive < 0.52

				if pureParty {
					return 0.14, fmt.Sprintf("pure party evidence cleanParty=%.3f cleanBright=%.3f joyConf=%.3f swagger=%.3f dirty=%.3f edge=%.3f", d.CleanParty, d.CleanBright, d.JoyConfidence, b.Swagger, d.DirtyElectro, d.EdgeDrive)
				}
				return 0, ""
			}),
		},
	}
}

func upliftDriveNode() DecisionNode {
	return DecisionNode{
		Label: LabelUpliftDrive, Family: FamilyPositive, Priority: 70,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return upliftDriveScore(b) },
		Gates: []DecisionGate{
			gate("joy_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Joy >= 0.38, fmt.Sprintf("joy %.3f < 0.38", b.Joy)
			}),
			gate("sprint_or_motion", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.SprintClean >= 0.44 || b.Motion >= 0.58, fmt.Sprintf("sprintClean=%.3f motion=%.3f", b.SprintClean, b.Motion)
			}),
			gate("not_combat", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Combat < 0.46 && b.Roughness < 0.46, fmt.Sprintf("combat=%.3f rough=%.3f", b.Combat, b.Roughness)
			}),
			gate("not_dirty_pressure", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				dirtyPressure := (d.DirtyElectro >= 0.42 && b.Pressure >= 0.52 && b.Roughness >= 0.40) ||
					(d.HeavyDrive >= 0.70 && b.Pressure >= 0.58 && b.Roughness >= 0.42)
				return !dirtyPressure, fmt.Sprintf("dirty=%.3f heavyDrive=%.3f pressure=%.3f rough=%.3f", d.DirtyElectro, d.HeavyDrive, b.Pressure, b.Roughness)
			}),
		},
	}
}

func joyPartyNode() DecisionNode {
	return DecisionNode{
		Label: LabelJoyParty, Family: FamilyPositive, Priority: 80,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return joyPartyScore(b) },
		Gates: []DecisionGate{
			gate("joy_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Joy >= cfg.JoyPartyMinJoy, fmt.Sprintf("joy %.3f < %.3f", b.Joy, cfg.JoyPartyMinJoy)
			}),
			gate("clean_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return d.CleanBright >= cfg.JoyPartyMinClean || d.CleanParty >= cfg.JoyPartyMinClean || d.JoyConfidence >= 0.55, fmt.Sprintf("cleanBright=%.3f cleanParty=%.3f joyConf=%.3f", d.CleanBright, d.CleanParty, d.JoyConfidence)
			}),
			gate("not_rough", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				cleanPartyShield := d.JoyConfidence >= 0.55 && (d.CleanParty >= 0.58 || d.CleanBright >= 0.60)

				roughLimit := cfg.JoyPartyMaxRough
				if cleanPartyShield {
					roughLimit += 0.30
				}

				return b.Roughness < roughLimit, fmt.Sprintf("roughness %.3f >= %.3f cleanPartyShield=%v cleanParty=%.3f cleanBright=%.3f joyConf=%.3f", b.Roughness, roughLimit, cleanPartyShield, d.CleanParty, d.CleanBright, d.JoyConfidence)
			}),
			gate("not_pressure", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				cleanPartyShield := d.JoyConfidence >= 0.55 && (d.CleanParty >= 0.58 || d.CleanBright >= 0.60)

				pressureLimit := cfg.JoyPartyMaxPressure
				if cleanPartyShield {
					pressureLimit += 0.22
				}

				return b.Pressure < pressureLimit, fmt.Sprintf("pressure %.3f >= %.3f cleanPartyShield=%v cleanParty=%.3f cleanBright=%.3f joyConf=%.3f", b.Pressure, pressureLimit, cleanPartyShield, d.CleanParty, d.CleanBright, d.JoyConfidence)
			}),
			gate("not_dirty", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				cleanPartyShield := d.JoyConfidence >= 0.55 && (d.CleanParty >= 0.58 || d.CleanBright >= 0.60)

				dirtyLimit := cfg.JoyPartyMaxDirty
				edgeLimit := cfg.JoyPartyMaxEdge
				if cleanPartyShield {
					dirtyLimit += 0.18
					edgeLimit += 0.30
				}

				return d.DirtyElectro < dirtyLimit && d.EdgeDrive < edgeLimit, fmt.Sprintf("dirty=%.3f edge=%.3f limits=%.3f/%.3f cleanPartyShield=%v cleanParty=%.3f cleanBright=%.3f joyConf=%.3f", d.DirtyElectro, d.EdgeDrive, dirtyLimit, edgeLimit, cleanPartyShield, d.CleanParty, d.CleanBright, d.JoyConfidence)
			}),
			gate("not_combat", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				cleanPartyShield := d.JoyConfidence >= 0.55 && (d.CleanParty >= 0.58 || d.CleanBright >= 0.60)

				combatLimit := cfg.JoyCombatFloor
				if cleanPartyShield {
					combatLimit += 0.20
				}

				return b.Combat < combatLimit, fmt.Sprintf("combat %.3f >= %.3f cleanPartyShield=%v cleanParty=%.3f cleanBright=%.3f joyConf=%.3f", b.Combat, combatLimit, cleanPartyShield, d.CleanParty, d.CleanBright, d.JoyConfidence)
			}),
		},
	}
}

func sereneWarmGrooveNode() DecisionNode {
	return DecisionNode{
		Label: LabelSereneWarmGroove, Family: FamilyWarm, Priority: 90,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return sereneWarmGrooveScore(b) },
		Gates: []DecisionGate{
			gate("warmGroove_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				warmRootedGroove := d.WarmGroove >= 0.54 &&
					b.Serenity >= 0.60 &&
					d.DirtyElectro < 0.28 &&
					b.Combat < 0.32
				return d.WarmGroove >= cfg.WarmGrooveMinWarmGroove || warmRootedGroove, fmt.Sprintf("warmGroove %.3f < %.3f warmRootedGroove=%v serenity=%.3f dirty=%.3f combat=%.3f", d.WarmGroove, cfg.WarmGrooveMinWarmGroove, warmRootedGroove, b.Serenity, d.DirtyElectro, b.Combat)
			}),
			gate("serenity_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Serenity >= 0.52, fmt.Sprintf("serenity %.3f < 0.52", b.Serenity)
			}),
			gate("not_combat", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Combat < 0.38 && d.DirtyElectro < 0.44, fmt.Sprintf("combat=%.3f dirty=%.3f", b.Combat, d.DirtyElectro)
			}),
		},
	}
}

func nightSmoothNode() DecisionNode {
	return DecisionNode{
		Label: LabelNightSmooth, Family: FamilyWarm, Priority: 100,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return nightSmoothScore(b, d) },
		Gates: []DecisionGate{
			gate("night_warm", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				nightCore := d.WarmGroove >= 0.42 && b.Serenity >= 0.44
				nightNoir := d.WarmGroove >= 0.40 && b.Serenity >= 0.46 && d.VocalGrief >= 0.50 && d.DirtyElectro < 0.52
				return nightCore || nightNoir, fmt.Sprintf("warmGroove=%.3f serenity=%.3f vocalGrief=%.3f dirty=%.3f", d.WarmGroove, b.Serenity, d.VocalGrief, d.DirtyElectro)
			}),
			gate("not_melancholy_pressure", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				pressureMel := b.Melancholy >= 0.48 && b.Pressure >= 0.42 && d.TensePressure >= 0.50
				return !pressureMel, fmt.Sprintf("melancholy=%.3f pressure=%.3f tense=%.3f", b.Melancholy, b.Pressure, d.TensePressure)
			}),
			gate("not_dirty", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				nightNoirHeavy := d.WarmGroove >= 0.40 &&
					b.Serenity >= 0.46 &&
					d.VocalGrief >= 0.50 &&
					d.DirtyElectro >= 0.48 && d.DirtyElectro < 0.52 &&
					d.EdgeDrive < 0.56 &&
					b.Combat >= 0.50 && b.Combat < 0.56 &&
					b.Roughness >= 0.48
				return d.DirtyElectro < 0.48 || d.EdgeDrive < 0.50 || nightNoirHeavy, fmt.Sprintf("dirty=%.3f edge=%.3f nightNoirHeavy=%v", d.DirtyElectro, d.EdgeDrive, nightNoirHeavy)
			}),
			gate("not_combat", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				nightNoirHeavy := d.WarmGroove >= 0.40 &&
					b.Serenity >= 0.46 &&
					d.VocalGrief >= 0.50 &&
					d.DirtyElectro >= 0.48 && d.DirtyElectro < 0.52 &&
					d.EdgeDrive < 0.56 &&
					b.Combat >= 0.50 && b.Combat < 0.56 &&
					b.Roughness >= 0.48
				return b.Combat < 0.44 || nightNoirHeavy, fmt.Sprintf("combat %.3f >= 0.44 nightNoirHeavy=%v", b.Combat, nightNoirHeavy)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("warm_rooted_groove_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				warmRootedGroove := d.WarmGroove >= 0.54 &&
					b.Serenity >= 0.60 &&
					d.DirtyElectro < 0.28 &&
					b.Combat < 0.32
				if warmRootedGroove {
					return 0.18, fmt.Sprintf("warm rooted groove warmGroove=%.3f serenity=%.3f dirty=%.3f combat=%.3f", d.WarmGroove, b.Serenity, d.DirtyElectro, b.Combat)
				}
				return 0, ""
			}),
			penalty("bright_atmospheric_calm_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				if isAtmosphericSereneCalm(b, d) {
					return 0.025, fmt.Sprintf("atmospheric calm pressure=%.3f serenity=%.3f smooth=%.3f", b.Pressure, b.Serenity, b.Smoothness)
				}
				return 0, ""
			}),
		},
	}
}

func sereneBrightNode() DecisionNode {
	return DecisionNode{
		Label: LabelSereneBright, Family: FamilyCalm, Priority: 110,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return sereneBrightScore(b) },
		Gates: []DecisionGate{
			gate("sereneBright_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return d.SereneBright >= 0.50 || b.Brightness >= 0.48, fmt.Sprintf("sereneBright=%.3f brightness=%.3f", d.SereneBright, b.Brightness)
			}),
			gate("not_pressure", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Pressure < 0.40 && b.Roughness < 0.32, fmt.Sprintf("pressure=%.3f rough=%.3f", b.Pressure, b.Roughness)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("calm_melancholy_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				if isCalmMelancholy(b, d, cfg) {
					return 0.04, fmt.Sprintf("calm melancholy mel=%.3f joy=%.3f", b.Melancholy, b.Joy)
				}
				return 0, ""
			}),
			penalty("dramatic_mixed_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				dramatic := dramaticMixedStateScore(b)
				serenityDominated := b.Serenity >= 0.65
				if dramatic >= 0.42 && !serenityDominated {
					return 0.15, fmt.Sprintf("dramatic mixed state score=%.3f", dramatic)
				}
				return 0, ""
			}),
		},
	}
}

func melancholyGriefNode() DecisionNode {
	return DecisionNode{
		Label: LabelMelancholyGrief, Family: FamilyMelancholy, Priority: 120,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return melancholyGriefScore(b, d) },
		Gates: []DecisionGate{
			gate("grief_dominant", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return isGriefDominant(b, d, cfg), fmt.Sprintf("melancholy=%.3f vocalGrief=%.3f dominance=%.3f", b.Melancholy, d.VocalGrief, griefDominanceScore(b, d))
			}),
			gate("not_pressure_melancholy", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				pressureMel := b.Pressure >= 0.44 && d.TensePressure >= 0.52 && b.Roughness >= 0.32
				return !pressureMel, fmt.Sprintf("pressure=%.3f tense=%.3f rough=%.3f", b.Pressure, d.TensePressure, b.Roughness)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("warm_rooted_groove_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				warmRootedGroove := d.WarmGroove >= 0.54 &&
					b.Serenity >= 0.60 &&
					d.DirtyElectro < 0.28 &&
					b.Combat < 0.32
				if warmRootedGroove {
					return 0.06, fmt.Sprintf("warm rooted groove warmGroove=%.3f serenity=%.3f dirty=%.3f combat=%.3f", d.WarmGroove, b.Serenity, d.DirtyElectro, b.Combat)
				}
				return 0, ""
			}),
		},
	}
}

func melancholyCalmNode() DecisionNode {
	return DecisionNode{
		Label: LabelMelancholyCalm, Family: FamilyMelancholy, Priority: 130,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return melancholyCalmScore(b) },
		Gates: []DecisionGate{
			gate("calm_melancholy", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return isCalmMelancholy(b, d, cfg), fmt.Sprintf(
					"mel=%.3f press=%.3f combat=%.3f griefDominant=%v joy=%.3f",
					b.Melancholy, b.Pressure, b.Combat, isGriefDominant(b, d, cfg), b.Joy,
				)
			}),
			gate("serenity_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Serenity >= 0.48, fmt.Sprintf("serenity %.3f < 0.48", b.Serenity)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("serene_calm_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				if b.Serenity >= 0.65 && b.Smoothness >= 0.70 && b.Melancholy >= 0.52 {
					return 0.04, fmt.Sprintf("serene-like serenity=%.3f smooth=%.3f mel=%.3f", b.Serenity, b.Smoothness, b.Melancholy)
				}
				return 0, ""
			}),
		},
	}
}

func sereneCalmNode() DecisionNode {
	return DecisionNode{
		Label: LabelSereneCalm, Family: FamilyCalm, Priority: 140,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return sereneCalmScore(b) },
		Gates: []DecisionGate{
			gate("serenity_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Serenity >= 0.58, fmt.Sprintf("serenity %.3f < 0.58", b.Serenity)
			}),
			gate("not_pressure", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				normalCalm := b.Pressure < 0.42 && b.Combat < 0.34 && b.Roughness < 0.34
				atmosphericCalm := isAtmosphericSereneCalm(b, d)
				return normalCalm || atmosphericCalm,
					fmt.Sprintf("pressure=%.3f combat=%.3f rough=%.3f atmospheric=%v", b.Pressure, b.Combat, b.Roughness, atmosphericCalm)
			}),
			gate("not_melancholy_calm", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				griefEvidence := b.Melancholy >= 0.54 && d.VocalGrief >= 0.58
				sereneCounter := hasSereneCounterEvidence(b, d)
				griefDominant := griefEvidence && !sereneCounter
				return !griefDominant,
					fmt.Sprintf(
						"melancholy=%.3f vocalGrief=%.3f sereneCounter=%t serenity=%.3f smooth=%.3f pressure=%.3f rough=%.3f combat=%.3f tense=%.3f",
						b.Melancholy, d.VocalGrief, sereneCounter,
						b.Serenity, b.Smoothness, b.Pressure, b.Roughness, b.Combat, d.TensePressure,
					)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("calm_melancholy_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				if isCalmMelancholy(b, d, cfg) {
					return 0.04, fmt.Sprintf("calm melancholy mel=%.3f joy=%.3f", b.Melancholy, b.Joy)
				}
				return 0, ""
			}),
			penalty("bright_clarity_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				clarity := brightClarityScore(b, d)
				orchestral := d.HeavyDrive >= 0.55
				if clarity >= 0.48 && b.Serenity >= 0.65 && orchestral {
					return 0.08, fmt.Sprintf("bright clarity score=%.3f serenity=%.3f", clarity, b.Serenity)
				}
				return 0, ""
			}),
		},
	}
}

func streetSwaggerNode() DecisionNode {
	return DecisionNode{
		Label: LabelStreetSwagger, Family: FamilyPositive, Priority: 150,
		Score: func(b Basis, d Debug, cfg Tuning) float64 { return streetSwaggerScore(b) },
		Gates: []DecisionGate{
			gate("swagger_min", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Swagger >= 0.48, fmt.Sprintf("swagger %.3f < 0.48", b.Swagger)
			}),
			gate("joy_cap", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Joy < 0.48, fmt.Sprintf("joy %.3f >= 0.48", b.Joy)
			}),
			gate("not_combat", func(b Basis, d Debug, cfg Tuning) (bool, string) {
				return b.Combat < 0.48, fmt.Sprintf("combat %.3f >= 0.48", b.Combat)
			}),
		},
		Penalty: []DecisionPenalty{
			penalty("heavy_dirty_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				heavyDirty := d.HeavyDrive >= 0.70 && b.Pressure >= 0.58 && b.Roughness >= 0.42 && d.DirtyElectro >= 0.37
				if heavyDirty {
					return 0.10, fmt.Sprintf("heavy dirty evidence heavyDrive=%.3f pressure=%.3f rough=%.3f dirty=%.3f", d.HeavyDrive, b.Pressure, b.Roughness, d.DirtyElectro)
				}
				return 0, ""
			}),
			penalty("uplift_shadow", func(b Basis, d Debug, cfg Tuning) (float64, string) {
				cleanUplift := b.Motion >= 0.58 &&
					b.Joy >= 0.44 &&
					b.Combat < 0.36 &&
					b.Pressure < 0.57 &&
					d.CleanParty >= 0.60 &&
					d.CleanBright >= 0.56 &&
					d.JoyConfidence >= 0.58 &&
					d.DirtyElectro < 0.34
				if cleanUplift {
					return 0.18, fmt.Sprintf("uplift drive evidence motion=%.3f joy=%.3f combat=%.3f pressure=%.3f cleanParty=%.3f cleanBright=%.3f joyConf=%.3f dirty=%.3f", b.Motion, b.Joy, b.Combat, b.Pressure, d.CleanParty, d.CleanBright, d.JoyConfidence, d.DirtyElectro)
				}
				return 0, ""
			}),
		},
	}
}

func gate(name string, fn func(Basis, Debug, Tuning) (bool, string)) DecisionGate {
	return DecisionGate{Name: name, Check: fn}
}

func penalty(name string, fn func(Basis, Debug, Tuning) (float64, string)) DecisionPenalty {
	return DecisionPenalty{Name: name, Value: fn}
}

func labelSanityWarnings(label LabelID, b Basis, d Debug) []string {
	out := []string{}
	switch label {
	case LabelJoyParty:
		if b.Combat > 0.60 {
			out = append(out, fmt.Sprintf("joy_party_but_combat_high %.3f", b.Combat))
		}
		if b.Pressure > 0.60 {
			out = append(out, fmt.Sprintf("joy_party_but_pressure_high %.3f", b.Pressure))
		}
		if b.Roughness > 0.60 {
			out = append(out, fmt.Sprintf("joy_party_but_rough_high %.3f", b.Roughness))
		}
	case LabelJoyFunk:
		if b.Combat > 0.62 && d.DirtyElectro > 0.48 {
			out = append(out, fmt.Sprintf("joy_funk_but_dirty_combat combat=%.3f dirty=%.3f", b.Combat, d.DirtyElectro))
		}
	case LabelDirtyElectroCombat:
		if d.WarmGroove >= 0.48 && b.Serenity >= 0.50 && b.Combat < 0.44 {
			out = append(out, "dirty_electro_but_warm_night_like")
		}
	case LabelSereneCalm, LabelNightSmooth, LabelSereneWarmGroove:
		nightNoirHeavy := d.WarmGroove >= 0.40 &&
			b.Serenity >= 0.46 &&
			d.VocalGrief >= 0.50 &&
			d.DirtyElectro >= 0.48 && d.DirtyElectro < 0.52 &&
			d.EdgeDrive < 0.56 &&
			b.Combat >= 0.50 && b.Combat < 0.56 &&
			b.Roughness >= 0.48
		if !nightNoirHeavy && (b.Combat > 0.46 || b.Pressure > 0.55) {
			out = append(out, fmt.Sprintf("calm_label_but_pressure combat=%.3f pressure=%.3f", b.Combat, b.Pressure))
		}
	}
	return out
}

func globalSanityWarnings(b Basis, d Debug) []string {
	out := []string{}
	if b.Joy > 0.50 && b.Combat > 0.50 {
		out = append(out, fmt.Sprintf("joy_and_combat_both_high joy=%.3f combat=%.3f", b.Joy, b.Combat))
	}
	if b.Serenity > 0.62 && b.Pressure > 0.52 {
		out = append(out, fmt.Sprintf("serenity_and_pressure_both_high serenity=%.3f pressure=%.3f", b.Serenity, b.Pressure))
	}
	return out
}

func axisWarnings(b Basis, d Debug) []string {
	out := []string{}
	if b.Combat < 0.30 && b.Pressure > 0.55 && b.Roughness > 0.45 {
		out = append(out, "pressure_rough_high_but_combat_low")
	}
	if b.Joy < 0.34 && d.CleanParty > 0.60 {
		out = append(out, "cleanParty_high_but_joy_low")
	}
	if b.Melancholy < 0.45 && d.VocalGrief > 0.70 {
		out = append(out, "vocalGrief_high_but_melancholy_low")
	}
	return out
}
