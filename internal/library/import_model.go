package library

import "time"

type ImportStatus string

type AnalysisStatus string

const (
	ImportPending ImportStatus = "pending"
	ImportReady   ImportStatus = "ready"
	ImportSkipped ImportStatus = "skipped"
	ImportMissing ImportStatus = "missing"
	ImportError   ImportStatus = "error"
)

const (
	AnalysisNone    AnalysisStatus = "none"
	AnalysisQueued  AnalysisStatus = "queued"
	AnalysisRunning AnalysisStatus = "running"
	AnalysisDone    AnalysisStatus = "done"
	AnalysisError   AnalysisStatus = "error"
)

type LibraryRoot struct {
	ID                 string `json:"id"`
	Path               string `json:"path"`
	LibraryType        string `json:"libraryType"`
	Enabled            bool   `json:"enabled"`
	Recursive          bool   `json:"recursive"`
	LastScanStartedAt  int64  `json:"lastScanStartedAt"`
	LastScanFinishedAt int64  `json:"lastScanFinishedAt"`
	LastScanError      string `json:"lastScanError"`
}

type FileError struct {
	ID          string `json:"id"`
	TrackID     string `json:"trackId"`
	Path        string `json:"path"`
	LibraryType string `json:"libraryType"`
	Stage       string `json:"stage"`
	Kind        string `json:"kind"`
	Message     string `json:"message"`
	CreatedAt   int64  `json:"createdAt"`
}

type ImportProgress struct {
	SessionID   string `json:"sessionId"`
	RootID      string `json:"rootId,omitempty"`
	RootPath    string `json:"rootPath,omitempty"`
	Status      string `json:"status"`
	Scanned     int    `json:"scanned"`
	Audio       int    `json:"audio"`
	New         int    `json:"new"`
	Updated     int    `json:"updated"`
	Unchanged   int    `json:"unchanged"`
	Skipped     int    `json:"skipped"`
	Errors      int    `json:"errors"`
	CurrentPath string `json:"currentPath,omitempty"`
	Message     string `json:"message,omitempty"`
}

type ImportSummary struct {
	SessionID      string        `json:"sessionId"`
	InputCount     int           `json:"inputCount"`
	Scanned        int           `json:"scanned"`
	AudioFound     int           `json:"audioFound"`
	Added          int           `json:"added"`
	Updated        int           `json:"updated"`
	Unchanged      int           `json:"unchanged"`
	Skipped        int           `json:"skipped"`
	Errors         int           `json:"errors"`
	AlreadyPresent int           `json:"alreadyPresent"`
	Roots          []LibraryRoot `json:"roots,omitempty"`
	FileErrors     []FileError   `json:"fileErrors,omitempty"`
}

func (t Track) IsUsableForRay() bool {
	return t.ImportStatus == string(ImportReady) &&
		t.AnalysisStatus == string(AnalysisDone) &&
		!t.FileMissing &&
		t.Path != "" &&
		len(t.Embedding) >= 8 &&
		t.PlaybackErrorCount < 3
}

func (t Track) IsReadyForPlayback() bool {
	return t.Path != "" && !t.FileMissing && t.ImportStatus == string(ImportReady)
}

func nowUnix() int64 { return time.Now().Unix() }
