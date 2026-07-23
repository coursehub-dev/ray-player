# audio_probe_batch

Offline model probe for `ray-player1`.

## First run

```bash
go run ./cmd/audio_probe_batch -config ./docs/audio_probe_config.json
```

If the config file is missing, the tool creates a template and exits.

## Batch run

```bash
go run ./cmd/audio_probe_batch \
  -config ./docs/audio_probe_config.json \
  -audio-dir "/Users/user/Downloads/music-test-2-pm" \
  -out "/Users/user/Downloads/music-test-2-pm/audio_probe_report.json"
```

Only `.mp3` files in the given folder are scanned. Subfolders are ignored.

## Output

The JSON report contains:

- audio decode info
- tempo/BPM data
- Essentia mel/base/genre/head outputs
- final features
- per-track warnings
- batch summary

## Short mode

For mass analysis use short mode:

```bash
go run ./cmd/audio_probe_batch \
  -config ./docs/audio_probe_config.json \
  -audio-dir "/Users/user/Downloads/nfs-test-100" \
  -out "/Users/user/Downloads/nfs-test-100/audio_probe_short.json" \
  -mode short \
  -pretty false
```

Short mode keeps only compact per-track values:

- tempo
- final features
- compact genre
- audio texture: centroid/zcr/rms/loudness
- selected head support values
- computed short basis
- compact warnings

Use this mode for 50–100 track reports when tuning formulas.

## Ultra-short calibration mode

For iterative tuning, use:

```bash
go run ./cmd/audio_probe_batch \
  -config ./docs/audio_probe_config.json \
  -audio-dir "/Users/user/Downloads/test-set1/" \
  -out "/Users/user/Downloads/test-set1/audio_probe_ultrashort.json" \
  -mode ultrashort \
  -pretty false \
  -expected ./docs/probe_expected_labels.json
```

Ultra-short keeps only:
- expected label
- detected label
- exact/compatible match
- ML heads
- audio2 descriptors
- basis3 values
- basis3 debug and top labels
