package library

const CurrentAnalysisVersion = 17

const CurrentEssentiaModelVersion = "discogs-effnet-bs64-1+genre-head-v8-prepatched"

// Version 17:
// - keeps the production EffNet contract as already-patched [N,128,96] and
//   prevents the dynamic backend from patching the same mel data a second time;
// - matches TensorflowInputTempoCNN by feeding magnitude mel bands and
//   standardizing them across the patch batch (TensorNormalize axis=0);
// - preserves ambiguous but usable multi-label Discogs parent genres instead of
//   collapsing low-margin predictions to Unknown;
// - gates precise subgenres by generic score/separation evidence and aggregates
//   regression heads from their raw per-patch outputs;
// - removes class-name deny-lists and synthetic path-derived features from real
//   pending/error tracks, so model uncertainty remains explicit rather than hacked.
//
// Version 16:
// - distributes the same 45-second Essentia budget over five song regions so
//   drops, bridges and late high-energy sections affect whole-track features;
// - derives valence only from explicit positive/negative evidence instead of
//   treating non-sad probability as positive valence;
// - keeps calibrated semantic probabilities on their global scale and uses
//   library percentiles only for raw audio texture;
// - assigns ClusterID in perceptual emotion space; Discogs embeddings remain
//   available for direct style/timbre similarity during recommendation.
//
// Version 15:
// - reproduces TensorflowInputMusiCNN preprocessing for Discogs EffNet:
//   Slaney mel scale, linear/unit-triangle filters, power spectrum and
//   log10(1 + 10000 * mel-energy);
// - keeps the v14 multi-window genre/tempo robustness, but invalidates all
//   embeddings and heads produced with the former incompatible frontend.
