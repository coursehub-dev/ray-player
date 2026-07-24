package main

import (
	"path/filepath"
	"strings"
	"testing"

	runtimeassets "ray-player1/internal/deps"
)

func TestONNXRuntimeAssetMatchesSupportedPlatforms(t *testing.T) {
	tests := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"darwin", "arm64", "osx-arm64-1.26.0.tgz"},
		{"linux", "amd64", "linux-x64-1.26.0.tgz"},
		{"linux", "arm64", "linux-aarch64-1.26.0.tgz"},
		{"windows", "amd64", "win-x64-1.26.0.zip"},
		{"windows", "arm64", "win-arm64-1.26.0.zip"},
	}
	for _, tc := range tests {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			asset, ok := runtimeassets.OnnxRuntimeAsset(tc.goos, tc.goarch)
			if !ok {
				t.Fatal("expected supported runtime asset")
			}
			if !strings.Contains(asset.Archive, tc.want) {
				t.Fatalf("archive=%q want substring %q", asset.Archive, tc.want)
			}
			if asset.Library == "" || asset.Member == "" {
				t.Fatalf("incomplete asset: %+v", asset)
			}
		})
	}
	if _, ok := runtimeassets.OnnxRuntimeAsset("darwin", "amd64"); ok {
		t.Fatal("darwin/amd64 must require an explicit/system runtime instead of a guessed archive")
	}
}

func TestStageAssetsRootMatchesBundleLayout(t *testing.T) {
	buildDir := filepath.Join("build", "bin")
	mac := stageAssetsRoot("darwin", buildDir, "ray-player1")
	wantMac := filepath.Join(buildDir, "ray-player1.app", "Contents", "Resources", "assets")
	if mac != wantMac {
		t.Fatalf("darwin root=%q want=%q", mac, wantMac)
	}
	windows := stageAssetsRoot("windows", buildDir, "ray-player1")
	wantWindows := filepath.Join(buildDir, "assets")
	if windows != wantWindows {
		t.Fatalf("windows root=%q want=%q", windows, wantWindows)
	}
}
