package library

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ray-player1/internal/analysis"
	"ray-player1/internal/db"
)

const minimumAudioFileSize = 1024

var supportedAudioExt = map[string]bool{
	".mp3":  true,
	".flac": true,
	".wav":  true,
	".m4a":  true,
	".aac":  true,
	".ogg":  true,
	".oga":  true,
	".opus": true,
	".aiff": true,
	".aif":  true,
	".wma":  true,
}

type scanCandidate struct {
	path           string
	normalizedPath string
	rootID         string
	meta           map[string]string
	metaSource     string
	info           os.FileInfo
	quickHash      string
	inode          string
}

func NormalizePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func QuickFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	h := sha1.New()
	_, _ = fmt.Fprintf(h, "size:%d;", info.Size())
	buf := make([]byte, 64*1024)
	n, _ := f.Read(buf)
	_, _ = h.Write(buf[:n])
	if info.Size() > int64(len(buf)) {
		_, _ = f.Seek(maxInt64(0, info.Size()-int64(len(buf))), io.SeekStart)
		n, _ = f.Read(buf)
		_, _ = h.Write(buf[:n])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func WaitUntilFileStable(ctx context.Context, path string) error {
	var lastSize int64 = -1
	stableCount := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("file is not stable yet")
		case <-ticker.C:
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			size := info.Size()
			if size > 0 && size == lastSize {
				stableCount++
				if stableCount >= 2 {
					return nil
				}
			} else {
				stableCount = 0
				lastSize = size
			}
		}
	}
}

func shouldSkipDir(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case ".git", ".svn", ".hg", "node_modules", "__macosx", ".spotlight-v100", ".trashes", "system volume information", "$recycle.bin":
		return true
	}
	return strings.HasPrefix(n, ".")
}

func isSupportedAudioExt(path string) bool {
	return supportedAudioExt[strings.ToLower(filepath.Ext(path))]
}

func fileInodeString(info os.FileInfo) string {
	return fmt.Sprintf("%d", info.ModTime().UnixNano())
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *Service) ListRoots() ([]LibraryRoot, error) {
	rows, err := s.store.ListLibraryRoots()
	if err != nil {
		return nil, err
	}
	out := make([]LibraryRoot, 0, len(rows))
	for _, row := range rows {
		out = append(out, LibraryRoot{ID: row.ID, Path: row.Path, LibraryType: row.LibraryType, Enabled: row.Enabled, Recursive: row.Recursive, LastScanStartedAt: row.LastScanStartedAt, LastScanFinishedAt: row.LastScanFinishedAt, LastScanError: row.LastScanError})
	}
	return out, nil
}

func (s *Service) ListFileErrors(limit int) ([]FileError, error) {
	rows, err := s.store.ListFileErrors(limit)
	if err != nil {
		return nil, err
	}
	out := make([]FileError, 0, len(rows))
	for _, row := range rows {
		out = append(out, FileError{ID: row.ID, TrackID: row.TrackID, Path: row.Path, LibraryType: row.LibraryType, Stage: row.Stage, Kind: row.Kind, Message: row.Message, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (s *Service) ImportPaths(paths []string, progress func(ImportProgress)) (ImportSummary, error) {
	summary := ImportSummary{InputCount: len(paths), SessionID: fmt.Sprintf("import-%d", time.Now().UnixNano())}
	session := db.ImportSessionRow{ID: summary.SessionID, LibraryType: "music", Status: "running", StartedAt: time.Now().Unix()}
	_ = s.store.StartImportSession(session)
	emit := func(status, current, message string) {
		session.Status = status
		session.ScannedCount = summary.Scanned
		session.AudioCount = summary.AudioFound
		session.NewCount = summary.Added
		session.UpdatedCount = summary.Updated
		session.UnchangedCount = summary.Unchanged
		session.SkippedCount = summary.Skipped
		session.ErrorCount = summary.Errors
		session.LastError = message
		_ = s.store.UpdateImportSession(session)
		if progress != nil {
			progress(ImportProgress{SessionID: summary.SessionID, Status: status, Scanned: summary.Scanned, Audio: summary.AudioFound, New: summary.Added, Updated: summary.Updated, Unchanged: summary.Unchanged, Skipped: summary.Skipped, Errors: summary.Errors, CurrentPath: current, Message: message})
		}
	}
	for _, input := range paths {
		info, err := os.Stat(input)
		if err != nil {
			summary.Errors++
			summary.FileErrors = append(summary.FileErrors, FileError{ID: fmt.Sprintf("err-%d", time.Now().UnixNano()), Path: input, LibraryType: "music", Stage: "scan", Kind: "file_not_found", Message: err.Error(), CreatedAt: time.Now().Unix()})
			emit("running", input, err.Error())
			continue
		}
		if info.IsDir() {
			root, err := s.ensureRoot(input)
			if err != nil {
				return summary, err
			}
			summary.Roots = append(summary.Roots, root)
			if err := s.scanRoot(context.Background(), root, &summary, emit); err != nil {
				emit("error", input, err.Error())
				return summary, err
			}
			continue
		}
		if !isSupportedAudioExt(input) {
			summary.Skipped++
			continue
		}
		summary.Scanned++
		summary.AudioFound++
		candidate, fileErr := s.prepareCandidate(context.Background(), input, "")
		if fileErr != nil {
			s.summaryAddError(&summary, "probe", input, fileErr)
			emit("running", input, fileErr.Error())
			continue
		}
		outcome, err := s.upsertCandidate(candidate)
		if err != nil {
			s.summaryAddError(&summary, "db", input, err)
			emit("running", input, err.Error())
			continue
		}
		s.applyOutcome(&summary, outcome)
		emit("running", input, outcome)
	}
	session.Status = "done"
	session.FinishedAt = time.Now().Unix()
	_ = s.store.UpdateImportSession(session)
	emit("done", "", "import finished")
	return summary, nil
}

func (s *Service) ensureRoot(path string) (LibraryRoot, error) {
	normalized, err := NormalizePath(path)
	if err != nil {
		return LibraryRoot{}, err
	}
	if existing, err := s.store.LookupLibraryRootByPath(normalized); err == nil {
		return LibraryRoot{ID: existing.ID, Path: existing.Path, LibraryType: existing.LibraryType, Enabled: existing.Enabled, Recursive: existing.Recursive, LastScanStartedAt: existing.LastScanStartedAt, LastScanFinishedAt: existing.LastScanFinishedAt, LastScanError: existing.LastScanError}, nil
	}
	root := LibraryRoot{ID: fmt.Sprintf("root-%x", sha1.Sum([]byte(normalized))), Path: normalized, LibraryType: "music", Enabled: true, Recursive: true}
	if err := s.store.UpsertLibraryRoot(db.LibraryRootRow{ID: root.ID, Path: root.Path, LibraryType: root.LibraryType, Enabled: root.Enabled, Recursive: root.Recursive}); err != nil {
		return LibraryRoot{}, err
	}
	return root, nil
}

func (s *Service) scanRoot(ctx context.Context, root LibraryRoot, summary *ImportSummary, emit func(status, current, message string)) error {
	seenAt := time.Now().Unix()
	startedAt := seenAt
	_ = s.store.UpdateLibraryRootScan(root.ID, startedAt, 0, "")
	walkErr := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			s.summaryAddError(summary, "scan", path, err)
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isSupportedAudioExt(path) {
			return nil
		}
		summary.Scanned++
		summary.AudioFound++
		emit("running", path, "scanning")
		candidate, fileErr := s.prepareCandidate(ctx, path, root.ID)
		if fileErr != nil {
			s.summaryAddError(summary, "probe", path, fileErr)
			return nil
		}
		outcome, err := s.upsertCandidate(candidate)
		if err != nil {
			s.summaryAddError(summary, "db", path, err)
			return nil
		}
		s.applyOutcome(summary, outcome)
		return nil
	})
	finishedAt := time.Now().Unix()
	if walkErr != nil {
		_ = s.store.UpdateLibraryRootScan(root.ID, startedAt, finishedAt, walkErr.Error())
		return walkErr
	}
	_ = s.store.MarkMissingTracksForRoot(root.ID, seenAt)
	_ = s.store.UpdateLibraryRootScan(root.ID, startedAt, finishedAt, "")
	return nil
}

func (s *Service) prepareCandidate(ctx context.Context, path string, rootID string) (scanCandidate, error) {
	normalized, err := NormalizePath(path)
	if err != nil {
		return scanCandidate{}, err
	}
	info, err := os.Stat(normalized)
	if err != nil {
		return scanCandidate{}, err
	}
	if info.IsDir() {
		return scanCandidate{}, fmt.Errorf("path is directory")
	}
	if info.Size() <= minimumAudioFileSize {
		return scanCandidate{}, fmt.Errorf("file too small")
	}
	waitCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if err := WaitUntilFileStable(waitCtx, normalized); err != nil {
		return scanCandidate{}, err
	}
	if _, err := analysis.ProbeAudioFile(ctx, analysis.FFprobePath(), normalized); err != nil {
		return scanCandidate{}, err
	}
	quickHash, err := QuickFileHash(normalized)
	if err != nil {
		return scanCandidate{}, err
	}
	meta, metaSource := readMetadata(normalized)
	return scanCandidate{path: normalized, normalizedPath: normalized, rootID: rootID, meta: meta, metaSource: metaSource, info: info, quickHash: quickHash, inode: fileInodeString(info)}, nil
}

func (s *Service) upsertCandidate(candidate scanCandidate) (string, error) {
	existing, err := s.store.GetTrackByNormalizedPath(candidate.normalizedPath)
	if err != nil && !dbIsNotFound(err) {
		return "", err
	}
	now := time.Now().Unix()
	if err == nil {
		changed := existing.FileSize != candidate.info.Size() || existing.FileMTime != candidate.info.ModTime().Unix() || strings.TrimSpace(existing.QuickHash) != strings.TrimSpace(candidate.quickHash) || existing.Path != candidate.path
		track := TrackFromRow(existing)
		track.Path = candidate.path
		track.NormalizedPath = candidate.normalizedPath
		track.LibraryRootID = candidate.rootID
		track.FileMissing = false
		track.FileSize = candidate.info.Size()
		track.FileMTime = candidate.info.ModTime().Unix()
		track.FileInode = candidate.inode
		track.QuickHash = candidate.quickHash
		track.LastSeenAt = now
		track.LastError = ""
		track.ImportStatus = string(ImportReady)
		if changed {
			track.AnalysisStatus = string(AnalysisQueued)
			track.AnalysisError = ""
			track.LastError = ""
		}
		if err := s.Upsert(track); err != nil {
			return "", err
		}
		if changed {
			s.beginIndexing(1, "rescanning")
			s.enqueueAnalysis(track.ID, track.Path, candidate.meta, candidate.metaSource)
			return "updated", nil
		}
		return "unchanged", nil
	}
	durationMs := quickDurationMs(candidate.path)
	track := buildTrack(candidate.path, candidate.meta, candidate.metaSource, pendingFeatures(), durationMs)
	track.NormalizedPath = candidate.normalizedPath
	track.LibraryRootID = candidate.rootID
	track.ImportStatus = string(ImportReady)
	track.AnalysisStatus = string(AnalysisQueued)
	track.FileMissing = false
	track.FileSize = candidate.info.Size()
	track.FileMTime = candidate.info.ModTime().Unix()
	track.FileInode = candidate.inode
	track.QuickHash = candidate.quickHash
	track.LastSeenAt = now
	track.AnalyzedLevel = 1
	if err := s.Upsert(track); err != nil {
		return "", err
	}
	s.beginIndexing(1, "importing")
	s.enqueueAnalysis(track.ID, track.Path, candidate.meta, candidate.metaSource)
	return "added", nil
}

func (s *Service) summaryAddError(summary *ImportSummary, stage, path string, err error) {
	summary.Errors++
	summary.Skipped++
	row := db.FileErrorRow{ID: fmt.Sprintf("err-%d", time.Now().UnixNano()), Path: path, LibraryType: "music", Stage: stage, Kind: classifyImportError(err), Message: err.Error(), CreatedAt: time.Now().Unix()}
	_ = s.store.AddFileError(row)
	summary.FileErrors = append(summary.FileErrors, FileError{ID: row.ID, Path: row.Path, LibraryType: row.LibraryType, Stage: row.Stage, Kind: row.Kind, Message: row.Message, CreatedAt: row.CreatedAt})
}

func classifyImportError(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission"):
		return "permission_denied"
	case strings.Contains(msg, "stable"):
		return "file_unstable"
	case strings.Contains(msg, "no audio stream"):
		return "no_audio_stream"
	case strings.Contains(msg, "ffprobe"):
		return "ffprobe_failed"
	default:
		return "analysis_failed"
	}
}

func (s *Service) applyOutcome(summary *ImportSummary, outcome string) {
	switch outcome {
	case "added":
		summary.Added++
	case "updated":
		summary.Updated++
	case "unchanged":
		summary.Unchanged++
		summary.AlreadyPresent++
	}
}

func dbIsNotFound(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "no rows")
}
