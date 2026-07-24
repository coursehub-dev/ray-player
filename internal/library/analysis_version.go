package library

const CurrentAnalysisVersion = 16

const CurrentEssentiaModelVersion = "discogs-effnet-bs64-1+genre-head-v7-musicnn"

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
