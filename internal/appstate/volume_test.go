package appstate

import "testing"

func TestZeroVolumeEnablesMute(t *testing.T) {
	state := NormalizeVolumeState(PlayerState{
		Volume:            0,
		Muted:             false,
		LastNonZeroVolume: 0.63,
	})

	if !state.Muted {
		t.Fatal("zero volume must enable mute")
	}
	if EffectiveVolume(state) != 0 {
		t.Fatalf("effective volume=%v want=0", EffectiveVolume(state))
	}
}

func TestRestoreVolumeUsesLastNonZeroValue(t *testing.T) {
	state := PlayerState{
		Volume:            0,
		Muted:             true,
		LastNonZeroVolume: 0.72,
	}

	if got := RestoreVolume(state); got != 0.72 {
		t.Fatalf("restore volume=%v want=0.72", got)
	}
}

func TestRestoreVolumeUsesSafeDefault(t *testing.T) {
	state := PlayerState{
		Muted:             true,
		LastNonZeroVolume: 0,
	}

	if got := RestoreVolume(state); got != DefaultVolume {
		t.Fatalf("restore volume=%v want=%v", got, DefaultVolume)
	}
}

func TestNonMutedVolumeUpdatesLastNonZero(t *testing.T) {
	state := NormalizeVolumeState(PlayerState{
		Volume:            0.45,
		Muted:             false,
		LastNonZeroVolume: 0.80,
	})

	if state.LastNonZeroVolume != 0.45 {
		t.Fatalf("lastNonZero=%v want=0.45", state.LastNonZeroVolume)
	}
}

func TestEffectiveVolumeWhenMuted(t *testing.T) {
	state := PlayerState{
		Volume:            0.58,
		Muted:             true,
		LastNonZeroVolume: 0.58,
	}

	if got := EffectiveVolume(state); got != 0 {
		t.Fatalf("effective volume=%v want=0 when muted", got)
	}
}
