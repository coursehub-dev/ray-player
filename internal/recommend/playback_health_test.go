package recommend

import (
	"testing"
	"time"

	"ray-player1/internal/library"
)

func TestPlaybackHealthPenaltyQuarantinesRepeatedFailures(t *testing.T) {
	healthy := library.Track{PlaybackErrorCount: 0}
	recent := library.Track{PlaybackErrorCount: 1, LastPlaybackErrorAt: time.Now().Unix()}
	quarantined := library.Track{PlaybackErrorCount: 3, LastPlaybackErrorAt: time.Now().Unix()}

	if playbackHealthPenalty(healthy) != 0 {
		t.Fatal("healthy track should have zero penalty")
	}
	if playbackHealthPenalty(recent) <= 0 {
		t.Fatal("recent failure should add penalty")
	}
	if playbackHealthPenalty(quarantined) < 2.0 {
		t.Fatal("quarantined track should have large penalty")
	}
}

func TestCanUseCandidateRejectsQuarantinedTrack(t *testing.T) {
	item := scored{track: library.Track{ID: "bad", PlaybackErrorCount: 3}}
	if canUseCandidate(item, nil, map[string]bool{}, library.Track{}, map[string]bool{}) {
		t.Fatal("quarantined track must be rejected")
	}
}
