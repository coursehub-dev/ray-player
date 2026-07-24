package library

const CurrentAnalysisVersion = 15

const CurrentEssentiaModelVersion = "discogs-effnet-bs64-1+genre-head-v7-musicnn"

// Version 15:
// - reproduces TensorflowInputMusiCNN preprocessing for Discogs EffNet:
//   Slaney mel scale, linear/unit-triangle filters, power spectrum and
//   log10(1 + 10000 * mel-energy);
// - keeps the v14 multi-window genre/tempo robustness, but invalidates all
//   embeddings and heads produced with the former incompatible frontend.
