package onnx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func logModelIdentity(
	name string,
	path string,
) error {
	hash, err := fileSHA256(path)
	if err != nil {
		return fmt.Errorf(
			"hash model %s: %w",
			name,
			err,
		)
	}

	essentiaLog.I(
		"model identity name=%s path=%q sha256=%s",
		name,
		path,
		hash,
	)
	return nil
}
