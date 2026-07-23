package main

import (
	"fmt"
	"strings"

	"ray-player1/internal/emotion"
)

type UltraShortBatchAudioProbeReport struct {
	Tool           string                  `json:"tool"`
	Version        string                  `json:"version"`
	Mode           string                  `json:"mode"`
	GeneratedAt    string                  `json:"generatedAt"`
	AudioDir       string                  `json:"audioDir"`
	FilesFound     int                     `json:"filesFound"`
	FilesOK        int                     `json:"filesOk"`
	FilesError     int                     `json:"filesError"`
	FormulaVersion string                  `json:"formulaVersion,omitempty"`
	Calibration    ShortCalibrationSummary `json:"calibration,omitempty"`
	LabelCounts    map[string]int          `json:"labelCounts,omitempty"`
	Warnings       map[string]int          `json:"warnings,omitempty"`
	Tracks         []UltraShortTrackReport `json:"tracks"`
}

type UltraShortTrackReport struct {
	File          string                  `json:"file"`
	Expected      string                  `json:"expected,omitempty"`
	Got           string                  `json:"got,omitempty"`
	Match         bool                    `json:"match,omitempty"`
	Compatible    bool                    `json:"compatible,omitempty"`
	Feeling       UltraShortFeeling       `json:"feeling"`
	ML            UltraShortML            `json:"ml"`
	Audio2        ShortAudioTexture2      `json:"audio2,omitempty"`
	Basis3        ShortBasis3             `json:"basis3"`
	Debug         Basis3Debug             `json:"debug,omitempty"`
	Decision      emotion.DecisionAudit   `json:"decision,omitempty"`
	ExpectedAudit *emotion.DecisionResult `json:"expectedAudit,omitempty"`
	Sanity        []string                `json:"sanity,omitempty"`
	LabelAudit    []LabelScore            `json:"labelAudit,omitempty"`
	Top           []LabelScore            `json:"top,omitempty"`
	Warn          []string                `json:"warn,omitempty"`
}

type UltraShortFeeling struct {
	Label      string  `json:"label"`
	Motion     float64 `json:"motion"`
	Density    float64 `json:"density"`
	Roughness  float64 `json:"roughness"`
	Brightness float64 `json:"brightness"`
	Smoothness float64 `json:"smoothness"`
	Pressure   float64 `json:"pressure"`
	Joy        float64 `json:"joy"`
	Melancholy float64 `json:"melancholy"`
	Serenity   float64 `json:"serenity"`
	Swagger    float64 `json:"swagger"`
	Combat     float64 `json:"combat"`
	Sprint     float64 `json:"sprint"`
}

type UltraShortML struct {
	Dance      float64 `json:"dance"`
	Valence    float64 `json:"valence"`
	Happy      float64 `json:"happy"`
	Sad        float64 `json:"sad"`
	Relaxed    float64 `json:"relaxed"`
	Party      float64 `json:"party"`
	Aggressive float64 `json:"aggressive"`
	Acoustic   float64 `json:"acoustic"`
	Electronic float64 `json:"electronic"`
	Vocal      float64 `json:"vocal"`
	Bright     float64 `json:"bright"`
	BPM        float64 `json:"bpm"`
	PBPM       float64 `json:"pbpm,omitempty"`
	TConf      float64 `json:"tConf"`
	TTrust     float64 `json:"tTrust,omitempty"`
}

func toUltraShortReport(r ShortBatchAudioProbeReport) UltraShortBatchAudioProbeReport {
	out := UltraShortBatchAudioProbeReport{
		Tool:           r.Tool,
		Version:        r.Version,
		Mode:           "ultrashort",
		GeneratedAt:    r.GeneratedAt,
		AudioDir:       r.AudioDir,
		FilesFound:     r.FilesFound,
		FilesOK:        r.FilesOK,
		FilesError:     r.FilesError,
		FormulaVersion: r.FormulaVersion,
		Calibration:    r.Summary.Calibration,
		LabelCounts:    r.Summary.LabelCounts,
		Warnings:       r.Summary.Warnings,
		Tracks:         make([]UltraShortTrackReport, 0, len(r.Tracks)),
	}
	for _, tr := range r.Tracks {
		out.Tracks = append(out.Tracks, toUltraShortTrack(tr))
	}
	return out
}

func toUltraShortTrack(tr ShortTrackProbeReport) UltraShortTrackReport {
	expectedAudit := expectedDecisionResult(tr.Expected, tr.Basis3Debug.Decision)
	return UltraShortTrackReport{
		File:       tr.File,
		Expected:   tr.Expected,
		Got:        tr.Basis3.Label,
		Match:      tr.Match,
		Compatible: tr.CompatibleMatch,
		Feeling: UltraShortFeeling{
			Label:      tr.Basis3.Label,
			Motion:     tr.Basis3.Motion,
			Density:    tr.Basis3.Density,
			Roughness:  tr.Basis3.Roughness,
			Brightness: tr.Basis3.Brightness,
			Smoothness: tr.Basis3.Smoothness,
			Pressure:   tr.Basis3.Pressure,
			Joy:        tr.Basis3.Joy,
			Melancholy: tr.Basis3.Melancholy,
			Serenity:   tr.Basis3.Serenity,
			Swagger:    tr.Basis3.Swagger,
			Combat:     tr.Basis3.Combat,
			Sprint:     tr.Basis3.SprintClean,
		},
		ML: UltraShortML{
			Dance:      tr.F.Dance,
			Valence:    tr.F.Val,
			Happy:      tr.F.Happy,
			Sad:        tr.F.Sad,
			Relaxed:    tr.F.Relax,
			Party:      tr.F.Party,
			Aggressive: tr.F.Aggr,
			Acoustic:   tr.F.Acoustic,
			Electronic: tr.F.Elec,
			Vocal:      tr.F.Vocal,
			Bright:     tr.F.Bright,
			BPM:        tr.BPM,
			PBPM:       tr.PBPM,
			TConf:      tr.TConf,
			TTrust:     tr.TTrust,
		},
		Audio2:        tr.Audio2,
		Basis3:        tr.Basis3,
		Debug:         tr.Basis3Debug,
		Decision:      tr.Basis3Debug.Decision,
		ExpectedAudit: expectedAudit,
		Sanity:        append([]string{}, tr.Basis3Debug.Decision.Sanity...),
		LabelAudit:    labelAuditForTrack(tr),
		Top:           tr.Basis3Debug.TopLabels,
		Warn:          tr.Warn,
	}
}

func expectedDecisionResult(expected string, audit emotion.DecisionAudit) *emotion.DecisionResult {
	if strings.TrimSpace(expected) == "" {
		return nil
	}
	for _, r := range audit.Results {
		if r.Label == expected {
			cp := r
			return &cp
		}
	}
	return &emotion.DecisionResult{Label: expected, Passed: false, Failed: []string{"expected label not present in decision DAG"}}
}

func labelAuditForTrack(tr ShortTrackProbeReport) []LabelScore {
	labels := []string{
		"dirty_electro_combat",
		"combat_force",
		"tense_pressure",
		"melancholy_pressure",
		"dramatic_arc",
		"joy_funk",
		"uplift_drive",
		"joy_party",
		"serene_warm_groove",
		"night_smooth",
		"serene_bright",
		"melancholy_grief",
		"melancholy_calm",
		"serene_calm",
		"street_swagger",
	}
	out := make([]LabelScore, 0, len(labels))
	for _, label := range labels {
		score, passed, failed, _ := expectedAuditDetails(tr, label)
		reason := ""
		if len(failed) > 0 {
			reason = failed[0]
		}
		out = append(out, LabelScore{Label: label, Score: r3(score), Passed: passed, Reason: reason})
	}
	return out
}

func expectedAuditDetails(tr ShortTrackProbeReport, label string) (score float64, passed bool, failed []string, sanity []string) {
	b := tr.Basis3
	d := tr.Basis3Debug
	sanity = sanity[:0]
	switch label {
	case "joy_party":
		score = clamp01(0.48*b.Joy + 0.24*b.Swagger + 0.12*b.Sprint + 0.18*b.Brightness + 0.06*tr.F.Party - 0.12*b.Combat - 0.04*b.Pressure)
		failed = appendIf(score < 0.48, failed, fmt.Sprintf("score %.3f < 0.48", score))
		failed = appendIf(b.Joy < 0.45, failed, fmt.Sprintf("joy %.3f < 0.45", b.Joy))
		failed = appendIf(b.Combat > 0.38, failed, fmt.Sprintf("combat %.3f > 0.38", b.Combat))
		failed = appendIf(b.Pressure > 0.48, failed, fmt.Sprintf("pressure %.3f > 0.48", b.Pressure))
		failed = appendIf(b.Roughness > 0.42, failed, fmt.Sprintf("roughness %.3f > 0.42", b.Roughness))
		failed = appendIf(d.CleanParty < 0.45 && d.CleanBright < 0.45, failed, fmt.Sprintf("cleanParty %.3f / cleanBright %.3f < 0.45", d.CleanParty, d.CleanBright))
		sanity = appendIf(b.Combat > 0.45 || b.Pressure > 0.52 || b.Roughness > 0.48, sanity, fmt.Sprintf("joy_party invalid_sanity combat=%.3f pressure=%.3f roughness=%.3f", b.Combat, b.Pressure, b.Roughness))
	case "joy_funk":
		score = clamp01(0.40*b.Joy + 0.26*b.Swagger + 0.20*b.Smoothness + 0.08*b.Motion + 0.08*b.Brightness + 0.04*b.Pulse - 0.08*b.Combat - 0.04*b.Density)
		failed = appendIf(score < 0.50, failed, fmt.Sprintf("score %.3f < 0.50", score))
		failed = appendIf(b.Swagger < 0.45, failed, fmt.Sprintf("swagger %.3f < 0.45", b.Swagger))
		failed = appendIf(b.Joy < 0.38, failed, fmt.Sprintf("joy %.3f < 0.38", b.Joy))
		failed = appendIf(b.Combat > 0.50, failed, fmt.Sprintf("combat %.3f > 0.50", b.Combat))
		failed = appendIf(d.DirtyElectro > 0.48, failed, fmt.Sprintf("dirtyElectro %.3f > 0.48", d.DirtyElectro))
		sanity = appendIf(b.Combat > 0.58 && d.DirtyElectro > 0.46, sanity, fmt.Sprintf("joy_funk invalid_sanity combat=%.3f dirtyElectro=%.3f", b.Combat, d.DirtyElectro))
	case "combat_force":
		score = clamp01(0.62*b.Combat + 0.18*b.Pressure + 0.10*b.Roughness + 0.06*b.Sprint + 0.04*b.Impact)
		failed = appendIf(score < 0.55, failed, fmt.Sprintf("score %.3f < 0.55", score))
		failed = appendIf(b.Combat < 0.45, failed, fmt.Sprintf("combat %.3f < 0.45", b.Combat))
		failed = appendIf(b.Pressure < 0.42, failed, fmt.Sprintf("pressure %.3f < 0.42", b.Pressure))
		failed = appendIf(b.Roughness < 0.40, failed, fmt.Sprintf("roughness %.3f < 0.40", b.Roughness))
		failed = appendIf(b.Joy > 0.48, failed, fmt.Sprintf("joy %.3f > 0.48", b.Joy))
	case "dirty_electro_combat":
		score = clamp01(0.56*b.Roughness + 0.22*b.Pressure + 0.12*b.Sprint + 0.06*b.Brightness + 0.04*d.DirtyElectro)
		failed = appendIf(score < 0.55, failed, fmt.Sprintf("score %.3f < 0.55", score))
		failed = appendIf(d.DirtyElectro < 0.42, failed, fmt.Sprintf("dirtyElectro %.3f < 0.42", d.DirtyElectro))
		failed = appendIf(b.Pressure < 0.42, failed, fmt.Sprintf("pressure %.3f < 0.42", b.Pressure))
		failed = appendIf(b.Roughness < 0.38, failed, fmt.Sprintf("roughness %.3f < 0.38", b.Roughness))
		failed = appendIf(d.CleanParty >= 0.45 && d.CleanBright >= 0.45, failed, fmt.Sprintf("clean shield %.3f/%.3f too high", d.CleanParty, d.CleanBright))
		sanity = appendIf(d.WarmGroove >= 0.45 && b.Serenity >= 0.46 && d.VocalGrief >= 0.45 && d.DirtyElectro < 0.42, sanity, fmt.Sprintf("dirty_electro invalid_sanity warmGroove=%.3f serenity=%.3f vocalGrief=%.3f", d.WarmGroove, b.Serenity, d.VocalGrief))
	case "tense_pressure":
		score = clamp01(0.46*b.Pressure + 0.30*b.Combat + 0.24*b.Roughness)
		failed = appendIf(score < 0.50, failed, fmt.Sprintf("score %.3f < 0.50", score))
		failed = appendIf(d.TensePressure < 0.50, failed, fmt.Sprintf("tensePressure %.3f < 0.50", d.TensePressure))
		failed = appendIf(b.Pressure < 0.42, failed, fmt.Sprintf("pressure %.3f < 0.42", b.Pressure))
		sanity = appendIf(d.TensePressure < 0.50 || b.Serenity > 0.68, sanity, fmt.Sprintf("tense_pressure invalid_sanity tensePressure=%.3f serenity=%.3f", d.TensePressure, b.Serenity))
	case "melancholy_grief":
		score = clamp01(0.46*b.Melancholy + 0.16*clamp01(0.5*b.Melancholy+0.3*(1-b.Joy)+0.2*b.Serenity) + 0.16*(1-b.Joy) + 0.10*b.Serenity + 0.08*(1-b.Serenity) + 0.04*d.VocalGrief)
		failed = appendIf(score < 0.52, failed, fmt.Sprintf("score %.3f < 0.52", score))
		failed = appendIf(b.Melancholy < 0.52, failed, fmt.Sprintf("melancholy %.3f < 0.52", b.Melancholy))
		failed = appendIf(d.VocalGrief < 0.58, failed, fmt.Sprintf("vocalGrief %.3f < 0.58", d.VocalGrief))
		failed = appendIf(b.Joy > 0.42, failed, fmt.Sprintf("joy %.3f > 0.42", b.Joy))
		failed = appendIf(b.Combat > 0.35, failed, fmt.Sprintf("combat %.3f > 0.35", b.Combat))
	case "melancholy_calm":
		score = clamp01(0.40*b.Melancholy + 0.24*b.Serenity + 0.16*b.Smoothness + 0.10*(1-b.Pressure) + 0.10*(1-b.Joy))
		failed = appendIf(score < 0.50, failed, fmt.Sprintf("score %.3f < 0.50", score))
		failed = appendIf(b.Melancholy < 0.52, failed, fmt.Sprintf("melancholy %.3f < 0.52", b.Melancholy))
		failed = appendIf(b.Serenity < 0.55, failed, fmt.Sprintf("serenity %.3f < 0.55", b.Serenity))
		failed = appendIf(b.Pressure > 0.42, failed, fmt.Sprintf("pressure %.3f > 0.42", b.Pressure))
		failed = appendIf(b.Combat > 0.32, failed, fmt.Sprintf("combat %.3f > 0.32", b.Combat))
	case "melancholy_pressure":
		score = clamp01(0.30*b.Melancholy + 0.34*b.Pressure + 0.12*b.Combat + 0.12*(1-b.Joy) + 0.12*b.Roughness)
		failed = appendIf(score < 0.50, failed, fmt.Sprintf("score %.3f < 0.50", score))
		failed = appendIf(b.Melancholy < 0.48, failed, fmt.Sprintf("melancholy %.3f < 0.48", b.Melancholy))
		failed = appendIf(b.Pressure < 0.42, failed, fmt.Sprintf("pressure %.3f < 0.42", b.Pressure))
		failed = appendIf(d.TensePressure < 0.50, failed, fmt.Sprintf("tensePressure %.3f < 0.50", d.TensePressure))
		failed = appendIf(b.Roughness < 0.32, failed, fmt.Sprintf("roughness %.3f < 0.32", b.Roughness))
	case "serene_calm":
		score = clamp01(0.44*b.Serenity + 0.16*b.Smoothness + 0.10*(1-b.Pressure) + 0.10*(1-b.Roughness) + 0.08*b.Joy)
		failed = appendIf(score < 0.50, failed, fmt.Sprintf("score %.3f < 0.50", score))
		failed = appendIf(b.Serenity < 0.58, failed, fmt.Sprintf("serenity %.3f < 0.58", b.Serenity))
		failed = appendIf(b.Pressure > 0.40, failed, fmt.Sprintf("pressure %.3f > 0.40", b.Pressure))
		failed = appendIf(b.Combat > 0.32, failed, fmt.Sprintf("combat %.3f > 0.32", b.Combat))
		failed = appendIf(b.Roughness > 0.28, failed, fmt.Sprintf("roughness %.3f > 0.28", b.Roughness))
	case "serene_warm_groove":
		score = clamp01(0.40*b.Serenity + 0.22*b.Joy + 0.18*b.Smoothness + 0.12*b.Brightness + 0.06*b.Motion + 0.04*(1-b.Combat))
		failed = appendIf(score < 0.50, failed, fmt.Sprintf("score %.3f < 0.50", score))
		failed = appendIf(d.WarmGroove < 0.55, failed, fmt.Sprintf("warmGroove %.3f < 0.55", d.WarmGroove))
		failed = appendIf(b.Serenity < 0.55, failed, fmt.Sprintf("serenity %.3f < 0.55", b.Serenity))
		failed = appendIf(b.Pressure > 0.45, failed, fmt.Sprintf("pressure %.3f > 0.45", b.Pressure))
		failed = appendIf(b.Combat > 0.35, failed, fmt.Sprintf("combat %.3f > 0.35", b.Combat))
	case "night_smooth":
		score = clamp01(0.38*b.Serenity + 0.20*(1-b.Brightness) + 0.18*b.Smoothness + 0.12*b.Melancholy + 0.08*(1-b.Pressure) + 0.04*(1-b.Combat))
		failed = appendIf(score < 0.50, failed, fmt.Sprintf("score %.3f < 0.50", score))
		failed = appendIf(b.Serenity < 0.48, failed, fmt.Sprintf("serenity %.3f < 0.48", b.Serenity))
		failed = appendIf(d.WarmGroove < 0.45, failed, fmt.Sprintf("warmGroove %.3f < 0.45", d.WarmGroove))
		failed = appendIf(b.Pressure > 0.48, failed, fmt.Sprintf("pressure %.3f > 0.48", b.Pressure))
		failed = appendIf(d.DirtyElectro > 0.42, failed, fmt.Sprintf("dirtyElectro %.3f > 0.42", d.DirtyElectro))
		failed = appendIf(b.Combat > 0.40, failed, fmt.Sprintf("combat %.3f > 0.40", b.Combat))
	case "serene_bright":
		score = clamp01(0.42*b.Serenity + 0.24*b.Brightness + 0.16*b.Smoothness + 0.06*b.Joy + 0.08*(1-b.Roughness) + 0.04*(1-b.Combat))
		failed = appendIf(score < 0.50, failed, fmt.Sprintf("score %.3f < 0.50", score))
		failed = appendIf(d.SereneBright < 0.50, failed, fmt.Sprintf("sereneBright %.3f < 0.50", d.SereneBright))
		failed = appendIf(b.Brightness < 0.42, failed, fmt.Sprintf("brightness %.3f < 0.42", b.Brightness))
		failed = appendIf(b.Roughness > 0.30, failed, fmt.Sprintf("roughness %.3f > 0.30", b.Roughness))
		failed = appendIf(b.Pressure > 0.38, failed, fmt.Sprintf("pressure %.3f > 0.38", b.Pressure))
	case "dramatic_arc":
		score = clamp01(0.34*b.Combat + 0.20*b.Pressure + 0.18*b.Melancholy + 0.16*b.Impact + 0.12*b.Sprint)
		failed = appendIf(score < 0.52, failed, fmt.Sprintf("score %.3f < 0.52", score))
		failed = appendIf(b.Melancholy < 0.45, failed, fmt.Sprintf("melancholy %.3f < 0.45", b.Melancholy))
		failed = appendIf(b.Pressure < 0.35 && b.Roughness < 0.35, failed, fmt.Sprintf("pressure %.3f / roughness %.3f too low", b.Pressure, b.Roughness))
	default:
		return 0, false, []string{fmt.Sprintf("unknown label %s", label)}, nil
	}
	if len(failed) == 0 {
		passed = true
	}
	if label == tr.Basis3.Label {
		passed = true
	}
	return score, passed, failed, sanity
}

func appendIf(cond bool, dst []string, msg string) []string {
	if cond {
		return append(dst, msg)
	}
	return dst
}
