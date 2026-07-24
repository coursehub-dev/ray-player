package emotion

import (
	"math"
	"testing"
)

func TestNormalizeSemanticFeatureKeepsCoreClassifierScale(t *testing.T) {
	got, ok := NormalizeSemanticFeature("Energy", 0.73)
	if !ok {
		t.Fatal("Energy should be a semantic feature")
	}
	if math.Abs(got-0.73) > 1e-9 {
		t.Fatalf("energy=%.6f want=0.73", got)
	}
}

func TestNormalizeSemanticFeatureReducesWeakEvidenceTowardAbsence(t *testing.T) {
	got, ok := NormalizeSemanticFeature("Dreaminess", 0.10)
	if !ok {
		t.Fatal("Dreaminess should be a semantic feature")
	}
	if math.Abs(got-0.045) > 1e-9 {
		t.Fatalf("dreaminess=%.6f want=0.045", got)
	}
}

func TestNormalizeSemanticFeatureUsesNeutralMidpointForBipolarAxis(t *testing.T) {
	got, ok := NormalizeSemanticFeature("TimbreBrightness", 0.10)
	if !ok {
		t.Fatal("TimbreBrightness should be a semantic feature")
	}
	if math.Abs(got-0.24) > 1e-9 {
		t.Fatalf("brightness=%.6f want=0.24", got)
	}
}
