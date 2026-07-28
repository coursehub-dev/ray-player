//go:build !windows && !linux && !darwin

package externalmedia

func platformDownloadsDir(string) string { return "" }
