package externalmedia

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type Status string

const (
	StatusFetchingMetadata Status = "fetching_metadata"
	StatusQueued           Status = "queued"
	StatusDownloading      Status = "downloading"
	StatusConverting       Status = "converting"
	StatusReady            Status = "ready"
	StatusError            Status = "error"
	StatusCanceled         Status = "canceled"
)

type LibraryType string

const (
	LibraryMusic   LibraryType = "music"
	LibraryPodcast LibraryType = "podcast"
)

type Metadata struct {
	ID          string  `json:"id"`
	Extractor   string  `json:"extractor"`
	WebpageURL  string  `json:"webpage_url"`
	Title       string  `json:"title"`
	Uploader    string  `json:"uploader"`
	Channel     string  `json:"channel"`
	Creator     string  `json:"creator"`
	Duration    float64 `json:"duration"`
	Thumbnail   string  `json:"thumbnail"`
	Description string  `json:"description"`
	UploadDate  string  `json:"upload_date"`
}

func (m Metadata) Author() string {
	for _, value := range []string{
		m.Uploader,
		m.Channel,
		m.Creator,
	} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

type Job struct {
	ID           string
	LibraryType  LibraryType
	ItemID       string
	URL          string
	SourceSite   string
	ExternalID   string
	Status       Status
	Progress     float64
	Title        string
	Uploader     string
	Duration     float64
	ThumbnailURL string
	OutputPath   string
	TempPath     string
	Bitrate      int
	Attempts     int
	MaxAttempts  int
	Error        string
	CreatedAt    int64
	UpdatedAt    int64
	StartedAt    int64
	FinishedAt   int64
}

type JobDTO struct {
	ID           string      `json:"id"`
	LibraryType  LibraryType `json:"libraryType"`
	ItemID       string      `json:"itemId"`
	URL          string      `json:"url"`
	SourceSite   string      `json:"sourceSite"`
	ExternalID   string      `json:"externalId"`
	Status       Status      `json:"status"`
	Progress     float64     `json:"progress"`
	Title        string      `json:"title"`
	Uploader     string      `json:"uploader"`
	Duration     float64     `json:"duration"`
	ThumbnailURL string      `json:"thumbnailUrl"`
	OutputPath   string      `json:"outputPath"`
	Error        string      `json:"error"`
	Attempts     int         `json:"attempts"`
	MaxAttempts  int         `json:"maxAttempts"`
}

func (j Job) DTO() JobDTO {
	return JobDTO{
		ID:           j.ID,
		LibraryType:  j.LibraryType,
		ItemID:       j.ItemID,
		URL:          j.URL,
		SourceSite:   j.SourceSite,
		ExternalID:   j.ExternalID,
		Status:       j.Status,
		Progress:     clamp01(j.Progress),
		Title:        j.Title,
		Uploader:     j.Uploader,
		Duration:     j.Duration,
		ThumbnailURL: j.ThumbnailURL,
		OutputPath:   j.OutputPath,
		Error:        j.Error,
		Attempts:     j.Attempts,
		MaxAttempts:  j.MaxAttempts,
	}
}

type Settings struct {
	YtDlpPath        string `json:"ytDlpPath"`
	FFmpegPath       string `json:"ffmpegPath"`
	YtDlpDownloadDir string `json:"ytDlpDownloadDir"`
}

type ToolCheckResult struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

func ValidateURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" &&
		parsed.Scheme != "https" {
		return errors.New("поддерживаются только http/https ссылки")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return errors.New("в ссылке отсутствует host")
	}
	return nil
}

func NormalizeLibraryType(value string) (LibraryType, error) {
	switch LibraryType(strings.TrimSpace(value)) {
	case LibraryMusic:
		return LibraryMusic, nil
	case LibraryPodcast:
		return LibraryPodcast, nil
	default:
		return "", fmt.Errorf(
			"unknown external library type %q",
			value,
		)
	}
}

func BitrateFor(kind LibraryType) int {
	if kind == LibraryPodcast {
		return 128
	}
	return 192
}

func OutputSubdirectory(kind LibraryType) string {
	if kind == LibraryPodcast {
		return "podcasts"
	}
	return "music"
}

func OutputFilename(meta Metadata) string {
	extractor := sanitizeFilenamePart(meta.Extractor)
	externalID := sanitizeFilenamePart(meta.ID)
	if extractor == "" {
		extractor = "external"
	}
	if externalID == "" {
		externalID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return extractor + "-" + externalID + ".mp3"
}

func OutputTemplate(dir string) string {
	return filepath.Join(
		dir,
		"%(extractor)s-%(id)s.%(ext)s",
	)
}

func sanitizeFilenamePart(value string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
