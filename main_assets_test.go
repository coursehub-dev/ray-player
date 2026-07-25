package main

import (
	"bytes"
	"testing"
)

func TestEmbeddedFrontendContainsIndex(t *testing.T) {
	data, err := assets.ReadFile("frontend/dist/index.html")
	if err != nil {
		t.Fatalf("embedded frontend index is unavailable: %v", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		t.Fatal("embedded frontend index is empty")
	}
}
