package audio

func classifyPlaybackEnd(samples int64, playedMs int, hasFirst bool, streamErr bool) PlaybackEndReason {
	if streamErr {
		return PlaybackEndStreamError
	}
	if samples == 0 || !hasFirst {
		return PlaybackEndEmptyStream
	}
	if playedMs < 3000 {
		return PlaybackEndEmptyStream
	}
	return PlaybackEndNatural
}
