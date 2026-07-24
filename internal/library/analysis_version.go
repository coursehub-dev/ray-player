package library

const CurrentAnalysisVersion = 14

const CurrentEssentiaModelVersion = "discogs-effnet-bs64-1+genre-head-v6-multiwindow"

// Version 14:
// - fixes the Discogs pre-patched mel contract (no double patching);
// - validates genre quality over every patch and tightens genre confidence;
// - fuses Jamendo mood/theme signals and validates regression heads;
// - aligns TempoCNN preprocessing with Essentia (11.025 kHz, 1024/512, Slaney 40-band, 256/128, standard scaling) and rejects only exact high-confidence locks;
// - switches long-track Discogs analysis to start/centre/end windows.
