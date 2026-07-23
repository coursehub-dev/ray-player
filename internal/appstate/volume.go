package appstate

const (
	DefaultVolume        = 0.58
	MinimumRestoreVolume = 0.08
)

// NormalizeVolumeState приводит состояние громкости к согласованному виду:
// Volume=0 → Muted=true, LastNonZeroVolume сохраняется для восстановления.
func NormalizeVolumeState(state PlayerState) PlayerState {
	state.Volume = clampVolume(state.Volume)
	state.LastNonZeroVolume = clampVolume(state.LastNonZeroVolume)

	if state.LastNonZeroVolume <= 0 {
		state.LastNonZeroVolume = DefaultVolume
	}

	if state.Volume <= 0 {
		state.Volume = 0
		state.Muted = true
		return state
	}

	if state.Muted {
		// Muted вручную — Volume хранит уровень до mute.
		return state
	}

	// Не muted и Volume > 0 — обновляем LastNonZeroVolume.
	state.LastNonZeroVolume = state.Volume
	return state
}

// EffectiveVolume возвращает реальный gain для аудиодвижка (0 при mute).
func EffectiveVolume(state PlayerState) float64 {
	state = NormalizeVolumeState(state)
	if state.Muted {
		return 0
	}
	return state.Volume
}

// RestoreVolume возвращает громкость для восстановления после mute.
func RestoreVolume(state PlayerState) float64 {
	value := clampVolume(state.LastNonZeroVolume)
	if value < MinimumRestoreVolume {
		value = DefaultVolume
	}
	return value
}

func clampVolume(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
