package onnx

import (
	"math"
	"testing"
)

func TestDerivedValenceDoesNotTreatNotSadAsPositive(t *testing.T) {
	got := deriveValence(0.36, 0.07, 0.15, 0.85, 0.82)
	if got > 0.58 {
		t.Fatalf("aggressive track valence=%.3f want <= 0.58", got)
	}
}

func TestDerivedValenceSeparatesJoyAndGrief(t *testing.T) {
	joy := deriveValence(0.81, 0.22, 0.52, 0.61, 0.05)
	grief := deriveValence(0.08, 0.89, 0.95, 0.03, 0.01)
	if joy < 0.75 {
		t.Fatalf("joy valence=%.3f want >= 0.75", joy)
	}
	if grief > 0.30 {
		t.Fatalf("grief valence=%.3f want <= 0.30", grief)
	}
}

func TestDerivedEnergyUsesDanceEngagementAndAggression(t *testing.T) {
	combat := deriveEnergy(0.88, 0.30, 0.95, 1.00)
	ballad := deriveEnergy(0.26, 0.03, 0.01, 0.49)
	if combat < 0.75 {
		t.Fatalf("combat energy=%.3f want >= 0.75", combat)
	}
	if ballad > 0.20 {
		t.Fatalf("ballad energy=%.3f want <= 0.20", ballad)
	}
}

func TestWeightedClassEvidenceCombinesRelatedTagsAsUnion(t *testing.T) {
	classes := []string{"melodic", "emotional", "other"}
	probs := []float32{0.20, 0.30, 0.90}
	got := weightedClassEvidence(probs, classes, map[string]float64{
		"melodic":   1.0,
		"emotional": 0.5,
	})
	want := 1 - (1-0.20)*(1-0.15)
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("evidence=%.6f want=%.6f", got, want)
	}
}
