package emoflow

import (
	"strconv"
	"strings"
	"testing"

	"ray-player1/internal/library"
	"ray-player1/internal/rays"
)

func TestComputeDirectionIntensify(t *testing.T) {
	current := library.Track{ID: "a", Energy: 0.42, Aggressive: 0.28, Heaviness: 0.22, Danceability: 0.35, Engagement: 0.40, Valence: 0.50}
	next := library.Track{ID: "b", Energy: 0.84, Aggressive: 0.74, Heaviness: 0.68, Danceability: 0.62, Engagement: 0.75, Valence: 0.48}
	if got := computeDirection(current, &next, nil); got != Intensify {
		t.Fatalf("expected %q, got %q", Intensify, got)
	}
}

func TestBuildStateIncludesNeighborsAndPalette(t *testing.T) {
	prev := library.Track{ID: "prev", Title: "Prev", Artist: "A", Relaxed: 0.72, Softness: 0.66, Acousticness: 0.58, Valence: 0.46, Energy: 0.34, BPMPerceived: 92, TempoConfidence: 0.7}
	cur := library.Track{ID: "cur", Title: "Cur", Artist: "B", Happy: 0.64, Valence: 0.62, TimbreBrightness: 0.58, Danceability: 0.61, Party: 0.52, Energy: 0.55, BPMPerceived: 126, TempoConfidence: 0.9}
	next := library.Track{ID: "next", Title: "Next", Artist: "C", Aggressive: 0.70, Heaviness: 0.63, Energy: 0.78, Party: 0.69, Danceability: 0.58, Engagement: 0.67, BPMPerceived: 132, TempoConfidence: 0.85}
	queue := []rays.QueueItem{{TrackID: "cur", IsCurrent: true, Insight: rays.QueueInsight{Mode: "warm_up"}}, {TrackID: "next"}}
	state := BuildState(cur, &prev, &next, queue, DefaultSettings())
	if state.TrackID != "cur" {
		t.Fatalf("unexpected track id: %s", state.TrackID)
	}
	if state.Previous == nil || state.Next == nil {
		t.Fatalf("expected previous and next states to be present")
	}
	if state.Direction == "" {
		t.Fatalf("expected direction")
	}
	if state.Palette.Accent == "" || state.Palette.Glow == "" {
		t.Fatalf("expected palette to be populated: %+v", state.Palette)
	}
	if !strings.HasPrefix(state.Palette.Accent, "oklch(") || !strings.HasPrefix(state.Palette.Glow, "oklch(") {
		t.Fatalf("expected oklch palette strings, got %+v", state.Palette)
	}
	if state.Transition.Reason == "" {
		t.Fatalf("expected transition reason")
	}
	if state.Current.Vector.RhythmicPulse <= 0 {
		t.Fatalf("expected rhythmic pulse from tempo, got %+v", state.Current.Vector)
	}
}

func TestNormalizeSettingsDefaultsIntensity(t *testing.T) {
	s := NormalizeSettings(UISettings{Enabled: true, Intensity: 0})
	if s.Intensity <= 0 {
		t.Fatalf("expected default intensity, got %v", s.Intensity)
	}
}

func TestEnergeticDarkRockIsNotMelancholic(t *testing.T) {
	v := Vector{
		Energy:    0.83,
		Valence:   0.10,
		Darkness:  0.75,
		Calmness:  0.35,
		Softness:  0.07,
		Pulse:     0.62,
		Drive:     0.72,
		Intensity: 0.74,
	}
	label := explainMood(v, Stable)
	if strings.Contains(strings.ToLower(label), "меланх") {
		t.Fatalf("expected non-melancholic label, got %q", label)
	}
}

func TestAggressiveDriveMapsToRed(t *testing.T) {
	track := library.Track{
		Title:            "Aggressive drive",
		Danceability:     0.94,
		Energy:           0.68,
		Valence:          0.48,
		Relaxed:          0.50,
		Party:            0.40,
		Aggressive:       0.10,
		Heaviness:        0.05,
		Softness:         0.08,
		TimbreBrightness: 0.30,
		BPMPerceived:     115,
		TempoConfidence:  1,
	}
	state := BuildState(track, nil, nil, nil, DefaultSettings())
	h := extractHueForTest(state.Current.Palette.Accent)
	if h < 350 && h > 45 {
		t.Fatalf("expected red/orange hue, got hue %.1f accent=%s vector=%+v", h, state.Current.Palette.Accent, state.Current.Vector)
	}
}

func TestCalmAtmosphereMapsToBlue(t *testing.T) {
	track := library.Track{
		Title:            "Calm atmosphere",
		Danceability:     0.35,
		Energy:           0.28,
		Valence:          0.55,
		Relaxed:          0.85,
		Party:            0.05,
		Softness:         0.65,
		Instrumentalness: 0.70,
		Electronicness:   0.25,
		BPMPerceived:     90,
		TempoConfidence:  0.8,
	}
	state := BuildState(track, nil, nil, nil, DefaultSettings())
	h := extractHueForTest(state.Current.Palette.Accent)
	if h < 190 || h > 265 {
		t.Fatalf("expected blue/cyan hue, got hue %.1f accent=%s vector=%+v", h, state.Current.Palette.Accent, state.Current.Vector)
	}
}

func TestNeutralFallbackIsNotYellowGreen(t *testing.T) {
	track := library.Track{
		Title:           "Neutral",
		Danceability:    0.50,
		Energy:          0.50,
		Valence:         0.50,
		Relaxed:         0.50,
		Party:           0.20,
		Softness:        0.30,
		BPMPerceived:    100,
		TempoConfidence: 0.5,
	}
	state := BuildState(track, nil, nil, nil, DefaultSettings())
	h := extractHueForTest(state.Current.Palette.Accent)
	if h >= 55 && h <= 150 {
		t.Fatalf("expected neutral fallback away from yellow/green, got hue %.1f accent=%s vector=%+v", h, state.Current.Palette.Accent, state.Current.Vector)
	}
}

func TestMechanicalPressureIsRed(t *testing.T) {
	track := library.Track{
		Title:            "Mechanical",
		Danceability:     0.84,
		Energy:           0.48,
		Valence:          0.51,
		Relaxed:          0.78,
		Party:            0.21,
		Aggressive:       0.00,
		Heaviness:        0.00,
		Electronicness:   0.51,
		Instrumentalness: 0.54,
		Softness:         0.16,
		TimbreBrightness: 0.33,
		BPMPerceived:     115,
		TempoConfidence:  1,
	}
	state := BuildState(track, nil, nil, nil, DefaultSettings())
	if state.Vector.MechanicalPressure < 0.60 {
		t.Fatalf("mechanical pressure too low: %+v", state.Vector)
	}
	if state.Vector.Intensity < 0.50 {
		t.Fatalf("intensity too low: %+v", state.Vector)
	}
	if state.Vector.Calmness > 0.55 {
		t.Fatalf("calmness should not dominate: %+v", state.Vector)
	}
	h := extractHueForTest(state.Current.Palette.Accent)
	if h > 40 && h < 320 {
		t.Fatalf("expected red/orange hue, got hue %.1f accent=%s vector=%+v", h, state.Current.Palette.Accent, state.Current.Vector)
	}
}

func TestClubPressureIsMagenta(t *testing.T) {
	track := library.Track{
		Title:            "Club",
		Danceability:     0.94,
		Energy:           0.58,
		Valence:          0.52,
		Relaxed:          0.46,
		Party:            0.38,
		Aggressive:       0.00,
		Heaviness:        0.00,
		Electronicness:   0.18,
		Vocalness:        0.68,
		Softness:         0.09,
		TimbreBrightness: 0.10,
		BPMPerceived:     102,
		TempoConfidence:  0.69,
	}
	state := BuildState(track, nil, nil, nil, DefaultSettings())
	if state.Vector.ClubPressure < 0.60 {
		t.Fatalf("club pressure too low: %+v", state.Vector)
	}
	if state.Vector.Drive < 0.53 {
		t.Fatalf("drive too low: %+v", state.Vector)
	}
	if state.Vector.Calmness > 0.55 {
		t.Fatalf("calmness should not dominate: %+v", state.Vector)
	}
	h := extractHueForTest(state.Current.Palette.Accent)
	if h > 50 && h < 320 {
		t.Fatalf("expected club magenta/red-purple hue, got hue %.1f accent=%s vector=%+v", h, state.Current.Palette.Accent, state.Current.Vector)
	}
}

func TestRidersLikeTrackIsCoolGroove(t *testing.T) {
	tk := library.Track{
		Danceability:     0.87,
		Energy:           0.62,
		Valence:          0.52,
		Happy:            0.05,
		Relaxed:          0.59,
		Party:            0.42,
		Aggressive:       0.02,
		Acousticness:     0.02,
		Electronicness:   0.18,
		Instrumentalness: 0.52,
		Vocalness:        0.48,
		Melodicness:      0.13,
		Softness:         0.12,
		Heaviness:        0.00,
		TimbreBrightness: 0.14,
		BPMPerceived:     115,
		TempoConfidence:  0.71,
		SpectralCentroid: 1607,
		ZeroCrossingRate: 0.122,
	}

	m := moodBasisFromTrack(tk)

	if m.Edge > 0.58 {
		t.Fatalf("Riders should not be high-edge/dirty: %+v", m)
	}
	if m.Edge <= 0 {
		t.Fatalf("Riders edge should be computed: %+v", m)
	}
	if m.Edge > 0.60 {
		t.Fatalf("Riders should not be dirty/high-edge: %+v", m)
	}
	if m.Coolness < 0.30 {
		t.Fatalf("Riders should retain a cool groove signal: %+v", m)
	}
}

func TestDemonoidLikeTrackIsDirtyPressure(t *testing.T) {
	tk := library.Track{
		Danceability:     0.91,
		Energy:           0.53,
		Valence:          0.51,
		Happy:            0.07,
		Sad:              0.05,
		Relaxed:          0.73,
		Party:            0.28,
		Aggressive:       0.02,
		Acousticness:     0.05,
		Electronicness:   0.30,
		Instrumentalness: 0.47,
		Vocalness:        0.53,
		Melodicness:      0.02,
		Softness:         0.15,
		Heaviness:        0.01,
		TimbreBrightness: 0.26,
		BPMPerceived:     115,
		TempoConfidence:  1.0,
		SpectralCentroid: 1569,
		ZeroCrossingRate: 0.198,
	}

	m := moodBasisFromTrack(tk)

	if m.Edge < 0.58 {
		t.Fatalf("Demonoid should be high-edge/dirty pressure: %+v", m)
	}
	if m.Pressure < 0.45 {
		t.Fatalf("Demonoid should have pressure: %+v", m)
	}
	if m.Calmness > 0.50 {
		t.Fatalf("Demonoid should not be classified as calm: %+v", m)
	}
}

func extractHueForTest(value string) float64 {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(value, ")"), "oklch("))
	parts := strings.Fields(s)
	if len(parts) < 3 {
		return -1
	}
	huePart := strings.TrimSuffix(parts[2], "/")
	h, _ := strconv.ParseFloat(huePart, 64)
	return h
}
