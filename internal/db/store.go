package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ray-player1/internal/logx"

	"ray-player1/internal/onnx"

	_ "modernc.org/sqlite"
)

var dbLog = logx.New("db")

type TrackRow struct {
	ID                   string
	Path                 string
	Title                string
	Artist               string
	Album                string
	Genre                string
	GenrePrimary         string
	GenreDetail          string
	GenreTagsJSON        string
	GenreLabel           string
	DurationMs           int
	DurationLabel        string
	Folder               string
	FileName             string
	Tempo                float64
	BPMPerceived         float64
	TempoConfidence      float64
	TempoStability       float64
	BPMHalf              float64
	BPMDouble            float64
	TempoSource          string
	TempoModelVersion    string
	TempoAnalyzedAt      int64
	TempoError           string
	Energy               float64
	Danceability         float64
	Valence              float64
	Acousticness         float64
	Electronicness       float64
	Instrumentalness     float64
	Vocalness            float64
	Happy                float64
	Sad                  float64
	Relaxed              float64
	Party                float64
	Aggressive           float64
	TimbreBrightness     float64
	Tonality             float64
	Approachability      float64
	Engagement           float64
	Melodicness          float64
	Softness             float64
	Heaviness            float64
	Dreaminess           float64
	Emotionality         float64
	Loudness             float64
	SpectralCentroid     float64
	ZeroCrossingRate     float64
	RMS                  float64
	SpectralFlatness     float64
	SpectralRolloff85    float64
	SpectralFlux         float64
	OnsetRate            float64
	DynamicRange         float64
	LowBandRatio         float64
	MidBandRatio         float64
	HighBandRatio        float64
	ClusterID            int
	PlayCount            int
	SkipCount            int
	CompleteCount        int
	MetadataSource       string
	AnalyzedLevel        int
	AnalysisVersion      int
	AnalyzedAt           int64
	AnalysisError        string
	EssentiaModelVersion string
	NormalizedPath       string
	LibraryRootID        string
	ImportStatus         string
	AnalysisStatus       string
	FileMissing          bool
	FileSize             int64
	FileMTime            int64
	FileInode            string
	QuickHash            string
	LastSeenAt           int64
	LastError            string
	Embedding            []float32
	TextEmbedding        []float32
	PlaybackErrorCount   int
	LastPlaybackError    string
	LastPlaybackErrorAt  int64
	AddedAt              int64

	SourceType       string
	SourceURL        string
	SourceSite       string
	ExternalID       string
	DownloadStatus   string
	DownloadProgress float64
	DownloadError    string
	DownloadAttempts int
	DownloadedAt     int64
}

type SearchHit struct {
	TrackRow
	BM25  float64
	Ngram float64
	Final float64
}

type HistoryRow struct {
	TrackID       string
	PositionMs    int
	Progress      float64
	ProgressLabel string
	PlayedAtLabel string
	PlayedAtUnix  int64
}

type RaySummaryRow struct {
	ID               string
	Name             string
	TrackCount       int
	CurrentTrackID   string
	CurrentTrackName string
	ResumeLabel      string
	PositionMs       int
	Active           bool
}

type RayTrackRow struct {
	TrackID       string
	Title         string
	Subtitle      string
	Artist        string
	Album         string
	GenrePrimary  string
	GenreLabel    string
	GenreDetail   string
	GenreTags     []onnx.GenreTag
	DurationMs    int
	DurationLabel string
	IsCurrent     bool
	Reason        string
	Bucket        string
	Strategy      string
	Score         float64
	PositionIndex int
}

type AppStateRow struct {
	CurrentTrackID          string
	PositionMs              int
	Volume                  float64
	Playing                 bool
	CurrentRayID            string
	OnnxRuntimePath         string
	MiniLMModelDir          string
	EssentiaModelDir        string
	FFmpegPath              string
	FFprobePath             string
	RepeatRay               bool
	ExtendRay               bool
	EmoFlowUIEnabled        bool
	EmoFlowUIIntensity      float64
	EmoFlowUIAnimateTrack   bool
	EmoFlowUIRespectReduced bool
}

type PlaybackSessionRow struct {
	ID             string
	Status         string
	CurrentTrackID string
	CurrentPath    string
	PositionMs     int
	DurationMs     int
	QueueID        string
	QueueIndex     int
	QueueJSON      string
	RayID          string
	RaySeedTrackID string
	RayMode        string
	RayConfigJSON  string
	UpdatedAt      int64
	LastError      string
}

type StrategyStat struct {
	Strategy string
	Plays    int
	Reward   float64
}

type TrackFeedbackRow struct {
	TrackID        string
	LastPlayedAt   int64
	LastSkippedAt  int64
	AvgCompletion  float64
	Affinity       float64
	PlayEvents     int
	SkipEvents     int
	CompleteEvents int
	LastEventType  string
}

type LibraryRootRow struct {
	ID                 string
	Path               string
	LibraryType        string
	Enabled            bool
	Recursive          bool
	LastScanStartedAt  int64
	LastScanFinishedAt int64
	LastScanError      string
	CreatedAt          int64
	UpdatedAt          int64
}

type ImportSessionRow struct {
	ID             string
	LibraryRootID  string
	LibraryType    string
	Status         string
	StartedAt      int64
	FinishedAt     int64
	ScannedCount   int
	AudioCount     int
	NewCount       int
	UpdatedCount   int
	UnchangedCount int
	SkippedCount   int
	ErrorCount     int
	LastError      string
}

type FileErrorRow struct {
	ID          string
	TrackID     string
	Path        string
	LibraryType string
	Stage       string
	Kind        string
	Message     string
	CreatedAt   int64
}

type Store struct {
	db   *sql.DB
	path string
}

func Open(appName string) (*Store, error) {
	root, err := resolveWritableRoot(appName)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "ray-player.db")
	return OpenAtPath(path)
}

func OpenAtPath(path string) (*Store, error) {
	dbLog.I("storage path=%s", filepath.Dir(path))
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := applySQLitePragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.ensureHealthy(); err != nil {
		if isCorruptionError(err) {
			dbLog.I("corruption detected path=%s err=%v; recreating database", path, err)
			if resetErr := s.resetDatabase(); resetErr != nil {
				_ = db.Close()
				return nil, resetErr
			}
		} else {
			_ = db.Close()
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func applySQLitePragmas(db *sql.DB) error {
	for _, pragma := range []string{"PRAGMA journal_mode=WAL;", "PRAGMA synchronous=NORMAL;", "PRAGMA foreign_keys=ON;", "PRAGMA busy_timeout=5000;", "PRAGMA cache_size=-4000;"} {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureHealthy() error {
	var result string
	if err := s.db.QueryRow(`PRAGMA quick_check;`).Scan(&result); err != nil {
		return err
	}
	if strings.TrimSpace(strings.ToLower(result)) != "ok" {
		return fmt.Errorf("sqlite integrity check failed: %s", result)
	}
	return nil
}

func (s *Store) resetDatabase() error {
	if s == nil {
		return nil
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	stamp := time.Now().Format("20060102-150405")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := s.path + suffix
		if _, err := os.Stat(src); err == nil {
			dst := fmt.Sprintf("%s.corrupt-%s%s", s.path, stamp, suffix)
			if err := os.Rename(src, dst); err != nil {
				_ = os.Remove(src)
			}
		}
	}
	newDB, err := sql.Open("sqlite", s.path)
	if err != nil {
		return err
	}
	newDB.SetMaxOpenConns(1)
	if err := applySQLitePragmas(newDB); err != nil {
		_ = newDB.Close()
		return err
	}
	s.db = newDB
	if err := s.migrate(); err != nil {
		_ = newDB.Close()
		return err
	}
	if err := s.ensureHealthy(); err != nil {
		_ = newDB.Close()
		return err
	}
	return nil
}

func isCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "malformed") || strings.Contains(msg, "corrupt") || strings.Contains(msg, "disk image")
}

func (s *Store) StoragePath() string {
	if s == nil {
		return ""
	}
	return filepath.Dir(s.path)
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tracks (
			id TEXT PRIMARY KEY,
			path TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			artist TEXT NOT NULL,
			album TEXT NOT NULL,
			genre TEXT NOT NULL,
			genre_primary TEXT NOT NULL DEFAULT '',
			genre_detail TEXT NOT NULL DEFAULT '',
			genre_tags TEXT NOT NULL DEFAULT '',
			genre_label TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL,
			duration_label TEXT NOT NULL,
			folder TEXT NOT NULL,
			file_name TEXT NOT NULL,
			tempo REAL NOT NULL DEFAULT 0,
			bpm_perceived REAL NOT NULL DEFAULT 0,
			tempo_confidence REAL NOT NULL DEFAULT 0,
			tempo_stability REAL NOT NULL DEFAULT 0,
			bpm_half REAL NOT NULL DEFAULT 0,
			bpm_double REAL NOT NULL DEFAULT 0,
			tempo_source TEXT NOT NULL DEFAULT '',
			tempo_model_version TEXT NOT NULL DEFAULT '',
			tempo_analyzed_at INTEGER NOT NULL DEFAULT 0,
			tempo_error TEXT NOT NULL DEFAULT '',
			energy REAL NOT NULL DEFAULT 0,
			danceability REAL NOT NULL DEFAULT 0,
			valence REAL NOT NULL DEFAULT 0,
			acousticness REAL NOT NULL DEFAULT 0,
			electronicness REAL NOT NULL DEFAULT 0,
			instrumentalness REAL NOT NULL DEFAULT 0,
			vocalness REAL NOT NULL DEFAULT 0,
			happy REAL NOT NULL DEFAULT 0,
			sad REAL NOT NULL DEFAULT 0,
			relaxed REAL NOT NULL DEFAULT 0,
			party REAL NOT NULL DEFAULT 0,
			aggressive REAL NOT NULL DEFAULT 0,
			timbre_brightness REAL NOT NULL DEFAULT 0,
			tonality REAL NOT NULL DEFAULT 0,
			approachability REAL NOT NULL DEFAULT 0,
			engagement REAL NOT NULL DEFAULT 0,
			melodicness REAL NOT NULL DEFAULT 0,
			softness REAL NOT NULL DEFAULT 0,
			heaviness REAL NOT NULL DEFAULT 0,
			dreaminess REAL NOT NULL DEFAULT 0,
			emotionality REAL NOT NULL DEFAULT 0,
			loudness REAL NOT NULL DEFAULT 0,
			spectral_centroid REAL NOT NULL DEFAULT 0,
			zero_crossing_rate REAL NOT NULL DEFAULT 0,
			rms REAL NOT NULL DEFAULT 0,
			spectral_flatness REAL NOT NULL DEFAULT 0,
			spectral_rolloff85 REAL NOT NULL DEFAULT 0,
			spectral_flux REAL NOT NULL DEFAULT 0,
			onset_rate REAL NOT NULL DEFAULT 0,
			dynamic_range REAL NOT NULL DEFAULT 0,
			low_band_ratio REAL NOT NULL DEFAULT 0,
			mid_band_ratio REAL NOT NULL DEFAULT 0,
			high_band_ratio REAL NOT NULL DEFAULT 0,
			cluster_id INTEGER NOT NULL DEFAULT 0,
			play_count INTEGER NOT NULL DEFAULT 0,
			skip_count INTEGER NOT NULL DEFAULT 0,
			complete_count INTEGER NOT NULL DEFAULT 0,
			metadata_source TEXT NOT NULL DEFAULT 'filename',
			analyzed_level INTEGER NOT NULL DEFAULT 0,
			analysis_version INTEGER NOT NULL DEFAULT 0,
			analyzed_at INTEGER NOT NULL DEFAULT 0,
			analysis_error TEXT NOT NULL DEFAULT '',
			essentia_model_version TEXT NOT NULL DEFAULT '',
			normalized_path TEXT NOT NULL DEFAULT '',
			library_root_id TEXT NOT NULL DEFAULT '',
			import_status TEXT NOT NULL DEFAULT 'ready',
			analysis_status TEXT NOT NULL DEFAULT 'none',
			file_missing INTEGER NOT NULL DEFAULT 0,
			file_size INTEGER NOT NULL DEFAULT 0,
			file_mtime INTEGER NOT NULL DEFAULT 0,
			file_inode TEXT NOT NULL DEFAULT '',
			quick_hash TEXT NOT NULL DEFAULT '',
			last_seen_at INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			embedding BLOB,
			text_embedding BLOB,
			playback_error_count INTEGER NOT NULL DEFAULT 0,
			last_playback_error TEXT NOT NULL DEFAULT '',
			last_playback_error_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch())
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS tracks_fts USING fts5(title, artist, album, genre, file_name, folder, content='tracks');`,
		`CREATE TABLE IF NOT EXISTS track_ngrams (
			track_id TEXT NOT NULL,
			gram TEXT NOT NULL,
			PRIMARY KEY(track_id, gram),
			FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_track_ngrams_gram ON track_ngrams(gram);`,
		`CREATE TABLE IF NOT EXISTS play_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			track_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			position_ms INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		);`,
		`CREATE INDEX IF NOT EXISTS idx_play_events_track ON play_events(track_id, created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS track_feedback (
			track_id TEXT PRIMARY KEY,
			last_played_at INTEGER NOT NULL DEFAULT 0,
			last_skipped_at INTEGER NOT NULL DEFAULT 0,
			avg_completion REAL NOT NULL DEFAULT 0,
			affinity REAL NOT NULL DEFAULT 0,
			play_events INTEGER NOT NULL DEFAULT 0,
			skip_events INTEGER NOT NULL DEFAULT 0,
			complete_events INTEGER NOT NULL DEFAULT 0,
			last_event_type TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS rays (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			seed_track_id TEXT NOT NULL,
			current_track_id TEXT NOT NULL,
			resume_label TEXT NOT NULL DEFAULT '',
			position_ms INTEGER NOT NULL DEFAULT 0,
			active INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		);`,
		`CREATE TABLE IF NOT EXISTS ray_tracks (
			ray_id TEXT NOT NULL,
			track_id TEXT NOT NULL,
			position_index INTEGER NOT NULL,
			subtitle TEXT NOT NULL,
			reason TEXT NOT NULL,
			is_current INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY(ray_id, position_index),
			FOREIGN KEY(ray_id) REFERENCES rays(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS app_state (
			id INTEGER PRIMARY KEY CHECK(id=1),
			current_track_id TEXT NOT NULL DEFAULT '',
			position_ms INTEGER NOT NULL DEFAULT 0,
			volume REAL NOT NULL DEFAULT 0.58,
			playing INTEGER NOT NULL DEFAULT 0,
			current_ray_id TEXT NOT NULL DEFAULT '',
			onnx_runtime_path TEXT NOT NULL DEFAULT '',
			minilm_model_dir TEXT NOT NULL DEFAULT '',
			essentia_model_dir TEXT NOT NULL DEFAULT '',
			ffmpeg_path TEXT NOT NULL DEFAULT 'ffmpeg',
			repeat_ray INTEGER NOT NULL DEFAULT 1,
			extend_ray INTEGER NOT NULL DEFAULT 0,
			emoflow_ui_enabled INTEGER NOT NULL DEFAULT 1,
			emoflow_ui_intensity REAL NOT NULL DEFAULT 1.0,
			emoflow_ui_animate_track INTEGER NOT NULL DEFAULT 1,
			emoflow_ui_respect_reduced INTEGER NOT NULL DEFAULT 1
		);`,
		`INSERT OR IGNORE INTO app_state(id) VALUES(1);`,
		`CREATE TABLE IF NOT EXISTS playback_session (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			current_track_id TEXT NOT NULL DEFAULT '',
			current_path TEXT NOT NULL DEFAULT '',
			position_ms INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			queue_id TEXT NOT NULL DEFAULT '',
			queue_index INTEGER NOT NULL DEFAULT -1,
			queue_json TEXT NOT NULL DEFAULT '',
			ray_id TEXT NOT NULL DEFAULT '',
			ray_seed_track_id TEXT NOT NULL DEFAULT '',
			ray_mode TEXT NOT NULL DEFAULT '',
			ray_config_json TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL,
			last_error TEXT NOT NULL DEFAULT ''
		);`,
		`INSERT OR IGNORE INTO playback_session(
			id, status, updated_at
		) VALUES(
			'last', 'stopped', 0
		);`,
		`CREATE TABLE IF NOT EXISTS strategy_stats (
			strategy TEXT PRIMARY KEY,
			plays INTEGER NOT NULL DEFAULT 0,
			reward REAL NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS library_roots (
			id TEXT PRIMARY KEY,
			path TEXT NOT NULL UNIQUE,
			library_type TEXT NOT NULL DEFAULT 'music',
			enabled INTEGER NOT NULL DEFAULT 1,
			recursive INTEGER NOT NULL DEFAULT 1,
			last_scan_started_at INTEGER NOT NULL DEFAULT 0,
			last_scan_finished_at INTEGER NOT NULL DEFAULT 0,
			last_scan_error TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch())
		);`,
		`CREATE TABLE IF NOT EXISTS import_sessions (
			id TEXT PRIMARY KEY,
			library_root_id TEXT NOT NULL DEFAULT '',
			library_type TEXT NOT NULL DEFAULT 'music',
			status TEXT NOT NULL,
			started_at INTEGER NOT NULL,
			finished_at INTEGER NOT NULL DEFAULT 0,
			scanned_count INTEGER NOT NULL DEFAULT 0,
			audio_count INTEGER NOT NULL DEFAULT 0,
			new_count INTEGER NOT NULL DEFAULT 0,
			updated_count INTEGER NOT NULL DEFAULT 0,
			unchanged_count INTEGER NOT NULL DEFAULT 0,
			skipped_count INTEGER NOT NULL DEFAULT 0,
			error_count INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS file_errors (
			id TEXT PRIMARY KEY,
			track_id TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL,
			library_type TEXT NOT NULL DEFAULT 'music',
			stage TEXT NOT NULL,
			kind TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_file_errors_created_at ON file_errors(created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS podcast_items (
			id TEXT PRIMARY KEY,
			path TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			author TEXT NOT NULL DEFAULT '',
			series TEXT NOT NULL DEFAULT '',
			folder TEXT NOT NULL DEFAULT '',
			duration REAL NOT NULL DEFAULT 0,
			file_size INTEGER NOT NULL DEFAULT 0,
			added_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			transcript_path TEXT NOT NULL DEFAULT '',
			transcript_status TEXT NOT NULL DEFAULT 'none',
			semantic_status TEXT NOT NULL DEFAULT 'none',
			summary TEXT NOT NULL DEFAULT '',
			topics_json TEXT NOT NULL DEFAULT '',
			keywords_json TEXT NOT NULL DEFAULT '',
			title_embedding BLOB,
			transcript_embedding BLOB,
			combined_embedding BLOB,
			last_position REAL NOT NULL DEFAULT 0,
			completed_ratio REAL NOT NULL DEFAULT 0,
			is_completed INTEGER NOT NULL DEFAULT 0,
			play_count INTEGER NOT NULL DEFAULT 0,
			skip_count INTEGER NOT NULL DEFAULT 0,
			last_played_at INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_podcast_items_folder
			ON podcast_items(folder, title);`,
		`CREATE INDEX IF NOT EXISTS idx_podcast_items_series
			ON podcast_items(series, title);`,
		`CREATE INDEX IF NOT EXISTS idx_podcast_items_progress
			ON podcast_items(is_completed, completed_ratio DESC);`,
		`UPDATE podcast_items
		 SET semantic_status = 'metadata_ready'
		 WHERE semantic_status IS NULL
		    OR semantic_status IN ('', 'none', 'pending');`,
		`CREATE TABLE IF NOT EXISTS podcast_history (
			id TEXT PRIMARY KEY,
			item_id TEXT NOT NULL,
			ray_id TEXT NOT NULL DEFAULT '',
			started_at INTEGER NOT NULL,
			ended_at INTEGER NOT NULL DEFAULT 0,
			start_position REAL NOT NULL DEFAULT 0,
			end_position REAL NOT NULL DEFAULT 0,
			listened_seconds REAL NOT NULL DEFAULT 0,
			completed_ratio REAL NOT NULL DEFAULT 0,
			source TEXT NOT NULL DEFAULT '',
			end_reason TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(item_id) REFERENCES podcast_items(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_podcast_history_item
			ON podcast_history(item_id, started_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_podcast_history_started
			ON podcast_history(started_at DESC);`,
		`CREATE TABLE IF NOT EXISTS podcast_rays (
			id TEXT PRIMARY KEY,
			seed_item_id TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			mode TEXT NOT NULL,
			content_mode TEXT NOT NULL DEFAULT 'recommended',
			sort_mode TEXT NOT NULL DEFAULT 'recommended',
			config_json TEXT NOT NULL DEFAULT '',
			is_manual_order INTEGER NOT NULL DEFAULT 0,
			manual_updated_at INTEGER NOT NULL DEFAULT 0,
			parent_ray_id TEXT NOT NULL DEFAULT '',
			revision INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			topic_vector BLOB,
			topic_summary TEXT NOT NULL DEFAULT '',
			folder_scope TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(seed_item_id) REFERENCES podcast_items(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS podcast_ray_items (
			ray_id TEXT NOT NULL,
			item_id TEXT NOT NULL,
			position_index INTEGER NOT NULL,
			original_position INTEGER NOT NULL DEFAULT 0,
			reason TEXT NOT NULL DEFAULT '',
			score REAL NOT NULL DEFAULT 0,
			semantic_score REAL NOT NULL DEFAULT 0,
			folder_score REAL NOT NULL DEFAULT 0,
			novelty_score REAL NOT NULL DEFAULT 0,
			resume_score REAL NOT NULL DEFAULT 0,
			added_by TEXT NOT NULL DEFAULT 'generator',
			PRIMARY KEY(ray_id, position_index),
			FOREIGN KEY(ray_id) REFERENCES podcast_rays(id) ON DELETE CASCADE,
			FOREIGN KEY(item_id) REFERENCES podcast_items(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS transition_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prev_track_id TEXT NOT NULL,
			next_track_id TEXT NOT NULL,
			strategy TEXT NOT NULL DEFAULT '',
			bucket TEXT NOT NULL DEFAULT '',
			transition_type TEXT NOT NULL DEFAULT '',
			energy_direction TEXT NOT NULL DEFAULT '',
			is_bridge INTEGER NOT NULL DEFAULT 0,
			is_discovery INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		);`,
		`CREATE INDEX IF NOT EXISTS idx_transition_events_prev ON transition_events(prev_track_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_transition_events_next ON transition_events(next_track_id, created_at DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.migrateTracksFTS(); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE tracks ADD COLUMN analyzed_level INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN genre_primary TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN genre_detail TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN genre_tags TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN genre_label TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE rays ADD COLUMN position_ms INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN analysis_version INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN analyzed_at INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN analysis_error TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN essentia_model_version TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN normalized_path TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN library_root_id TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN import_status TEXT NOT NULL DEFAULT 'ready';`,
		`ALTER TABLE tracks ADD COLUMN analysis_status TEXT NOT NULL DEFAULT 'none';`,
		`ALTER TABLE tracks ADD COLUMN file_missing INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN file_size INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN file_mtime INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN file_inode TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN quick_hash TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN last_seen_at INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN last_error TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN text_embedding BLOB;`,
		`ALTER TABLE tracks ADD COLUMN playback_error_count INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN last_playback_error TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN last_playback_error_at INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN electronicness REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN vocalness REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN happy REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN rms REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN spectral_flatness REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN spectral_rolloff85 REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN spectral_flux REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN onset_rate REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN dynamic_range REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN low_band_ratio REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN mid_band_ratio REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN high_band_ratio REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN relaxed REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN party REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN aggressive REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN timbre_brightness REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN tonality REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN approachability REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN engagement REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN melodicness REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN softness REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN heaviness REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN dreaminess REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN emotionality REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN bpm_perceived REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN tempo_confidence REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN tempo_stability REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN bpm_half REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN bpm_double REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN tempo_source TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN tempo_model_version TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE tracks ADD COLUMN tempo_analyzed_at INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE tracks ADD COLUMN tempo_error TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE ray_tracks ADD COLUMN bucket TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE ray_tracks ADD COLUMN strategy TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE ray_tracks ADD COLUMN score REAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE app_state ADD COLUMN onnx_runtime_path TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE app_state ADD COLUMN minilm_model_dir TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE app_state ADD COLUMN essentia_model_dir TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE app_state ADD COLUMN ffmpeg_path TEXT NOT NULL DEFAULT 'ffmpeg';`,
		`ALTER TABLE app_state ADD COLUMN ffprobe_path TEXT NOT NULL DEFAULT 'ffprobe';`,
		`ALTER TABLE app_state ADD COLUMN repeat_ray INTEGER NOT NULL DEFAULT 1;`,
		`ALTER TABLE app_state ADD COLUMN extend_ray INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE app_state ADD COLUMN emoflow_ui_enabled INTEGER NOT NULL DEFAULT 1;`,
		`ALTER TABLE app_state ADD COLUMN emoflow_ui_intensity REAL NOT NULL DEFAULT 1.0;`,
		`ALTER TABLE app_state ADD COLUMN emoflow_ui_animate_track INTEGER NOT NULL DEFAULT 1;`,
		`ALTER TABLE app_state ADD COLUMN emoflow_ui_respect_reduced INTEGER NOT NULL DEFAULT 1;`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_root_seen ON tracks(library_root_id, last_seen_at);`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_normalized_path ON tracks(normalized_path);`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			errLower := strings.ToLower(err.Error())
			if strings.Contains(errLower, "duplicate column") || strings.Contains(errLower, "no such column") {
				continue
			}
			return err
		}
	}

	podcastColumns := []struct {
		table      string
		column     string
		definition string
	}{
		{
			table:      "podcast_items",
			column:     "modified_at",
			definition: "INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_rays",
			column:     "content_mode",
			definition: "TEXT NOT NULL DEFAULT 'recommended'",
		},
		{
			table:      "podcast_rays",
			column:     "sort_mode",
			definition: "TEXT NOT NULL DEFAULT 'recommended'",
		},
		{
			table:      "podcast_rays",
			column:     "config_json",
			definition: "TEXT NOT NULL DEFAULT ''",
		},
		{
			table:      "podcast_rays",
			column:     "is_manual_order",
			definition: "INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_rays",
			column:     "manual_updated_at",
			definition: "INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_rays",
			column:     "parent_ray_id",
			definition: "TEXT NOT NULL DEFAULT ''",
		},
		{
			table:      "podcast_rays",
			column:     "revision",
			definition: "INTEGER NOT NULL DEFAULT 1",
		},
		{
			table:      "podcast_history",
			column:     "ray_id",
			definition: "TEXT NOT NULL DEFAULT ''",
		},
		{
			table:      "podcast_history",
			column:     "end_reason",
			definition: "TEXT NOT NULL DEFAULT ''",
		},
		{
			table:      "podcast_history",
			column:     "updated_at",
			definition: "INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_ray_items",
			column:     "original_position",
			definition: "INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_ray_items",
			column:     "semantic_score",
			definition: "REAL NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_ray_items",
			column:     "folder_score",
			definition: "REAL NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_ray_items",
			column:     "novelty_score",
			definition: "REAL NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_ray_items",
			column:     "resume_score",
			definition: "REAL NOT NULL DEFAULT 0",
		},
		{
			table:      "podcast_ray_items",
			column:     "added_by",
			definition: "TEXT NOT NULL DEFAULT 'generator'",
		},
	}

	for _, column := range podcastColumns {
		if err := ensureColumn(
			s.db,
			column.table,
			column.column,
			column.definition,
		); err != nil {
			return err
		}
	}

	if _, err := s.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_podcast_ray_item_unique
		ON podcast_ray_items(ray_id, item_id)
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_podcast_history_started
		ON podcast_history(started_at DESC)
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		UPDATE podcast_history
		SET updated_at = started_at
		WHERE updated_at = 0
	`); err != nil {
		return err
	}

	musicRayColumns := []struct {
		table      string
		column     string
		definition string
	}{
		{
			table:      "rays",
			column:     "content_mode",
			definition: "TEXT NOT NULL DEFAULT 'stable'",
		},
		{
			table:      "rays",
			column:     "sort_mode",
			definition: "TEXT NOT NULL DEFAULT 'recommended'",
		},
		{
			table:      "rays",
			column:     "is_manual_order",
			definition: "INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:      "rays",
			column:     "manual_updated_at",
			definition: "INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:      "rays",
			column:     "parent_ray_id",
			definition: "TEXT NOT NULL DEFAULT ''",
		},
		{
			table:      "rays",
			column:     "revision",
			definition: "INTEGER NOT NULL DEFAULT 1",
		},
		{
			table:      "ray_tracks",
			column:     "original_position",
			definition: "INTEGER NOT NULL DEFAULT 0",
		},
	}

	for _, column := range musicRayColumns {
		if err := ensureColumn(
			s.db,
			column.table,
			column.column,
			column.definition,
		); err != nil {
			return err
		}
	}

	// Meta key-value store
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS app_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`); err != nil {
		return err
	}

	externalColumns := []struct {
		table      string
		column     string
		definition string
	}{
		{"tracks", "source_type", "TEXT NOT NULL DEFAULT 'local'"},
		{"tracks", "source_url", "TEXT NOT NULL DEFAULT ''"},
		{"tracks", "source_site", "TEXT NOT NULL DEFAULT ''"},
		{"tracks", "external_id", "TEXT NOT NULL DEFAULT ''"},
		{"tracks", "download_status", "TEXT NOT NULL DEFAULT 'ready'"},
		{"tracks", "download_progress", "REAL NOT NULL DEFAULT 1"},
		{"tracks", "download_error", "TEXT NOT NULL DEFAULT ''"},
		{"tracks", "download_attempts", "INTEGER NOT NULL DEFAULT 0"},
		{"tracks", "downloaded_at", "INTEGER NOT NULL DEFAULT 0"},

		{"podcast_items", "source_type", "TEXT NOT NULL DEFAULT 'local'"},
		{"podcast_items", "source_url", "TEXT NOT NULL DEFAULT ''"},
		{"podcast_items", "source_site", "TEXT NOT NULL DEFAULT ''"},
		{"podcast_items", "external_id", "TEXT NOT NULL DEFAULT ''"},
		{"podcast_items", "download_status", "TEXT NOT NULL DEFAULT 'ready'"},
		{"podcast_items", "download_progress", "REAL NOT NULL DEFAULT 1"},
		{"podcast_items", "download_error", "TEXT NOT NULL DEFAULT ''"},
		{"podcast_items", "download_attempts", "INTEGER NOT NULL DEFAULT 0"},
		{"podcast_items", "downloaded_at", "INTEGER NOT NULL DEFAULT 0"},
	}

	for _, column := range externalColumns {
		if err := ensureColumn(
			s.db,
			column.table,
			column.column,
			column.definition,
		); err != nil {
			return err
		}
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS external_download_jobs (
			id TEXT PRIMARY KEY,
			library_type TEXT NOT NULL,
			item_id TEXT NOT NULL,
			url TEXT NOT NULL,
			source_site TEXT NOT NULL DEFAULT '',
			external_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			progress REAL NOT NULL DEFAULT 0,
			title TEXT NOT NULL DEFAULT '',
			uploader TEXT NOT NULL DEFAULT '',
			duration REAL NOT NULL DEFAULT 0,
			thumbnail_url TEXT NOT NULL DEFAULT '',
			output_path TEXT NOT NULL DEFAULT '',
			temp_path TEXT NOT NULL DEFAULT '',
			bitrate INTEGER NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 3,
			error TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			started_at INTEGER NOT NULL DEFAULT 0,
			finished_at INTEGER NOT NULL DEFAULT 0
		);

		CREATE INDEX IF NOT EXISTS idx_external_download_jobs_status
		ON external_download_jobs(status, created_at);

		CREATE INDEX IF NOT EXISTS idx_external_download_jobs_item
		ON external_download_jobs(library_type, item_id);

		CREATE INDEX IF NOT EXISTS idx_tracks_external_source
		ON tracks(source_type, source_site, external_id);

		CREATE INDEX IF NOT EXISTS idx_podcasts_external_source
		ON podcast_items(source_type, source_site, external_id);
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		UPDATE external_download_jobs
		SET status = 'queued',
		    error = '',
		    updated_at = unixepoch()
		WHERE status IN (
			'fetching_metadata',
			'downloading',
			'converting'
		)
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		UPDATE tracks
		SET analysis_status = 'pending',
		    analysis_version = 0,
		    analysis_error = 'invalid or stale ONNX analysis result',
		    embedding = CASE
		        WHEN embedding IS NOT NULL AND length(embedding) != ? THEN NULL
		        ELSE embedding
		    END,
		    genre = CASE WHEN genre = '' THEN 'Unknown' ELSE genre END,
		    genre_primary = '',
		    genre_detail = '',
		    genre_tags = '',
		    genre_label = ''
		WHERE source_type != 'pending_external'
		  AND (
		    (embedding IS NOT NULL AND length(embedding) != ?)
		    OR genre = ''
		  )
	`,
		1280*4, 1280*4,
	); err != nil {
		return err
	}

	return nil
}

func (s *Store) GetMeta(key string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM app_meta WHERE key = ?`,
		key,
	).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO app_meta(key, value, updated_at)
		VALUES(?, ?, unixepoch())
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`, key, value)
	return err
}

func (s *Store) UpsertTrack(t TrackRow, grams []string) error {
	for attempt := 0; attempt < 2; attempt++ {
		if err := s.upsertTrackOnce(t, grams); err != nil {
			if attempt == 0 && isCorruptionError(err) {
				dbLog.I("upsert corruption detected id=%s err=%v; resetting database", t.ID, err)
				if resetErr := s.resetDatabase(); resetErr != nil {
					return resetErr
				}
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("upsert failed after database reset for id=%s", t.ID)
}

func (s *Store) UpdateTrackCluster(id string, clusterID int) error {
	_, err := s.db.Exec(`UPDATE tracks SET cluster_id = ?, updated_at = unixepoch() WHERE id = ?`, clusterID, id)
	if err != nil {
		dbLog.I("update cluster failed id=%s cluster=%d err=%v", id, clusterID, err)
	}
	return err
}

func (s *Store) upsertTrackOnce(t TrackRow, grams []string) error {
	start := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		dbLog.I("upsert begin failed id=%s err=%v", t.ID, err)
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO tracks(id, path, title, artist, album, genre, genre_primary, genre_detail, genre_tags, genre_label, duration_ms, duration_label, folder, file_name, tempo, bpm_perceived, tempo_confidence, tempo_stability, bpm_half, bpm_double, tempo_source, tempo_model_version, tempo_analyzed_at, tempo_error, energy, danceability, valence, acousticness, electronicness, instrumentalness, vocalness, happy, sad, relaxed, party, aggressive, timbre_brightness, tonality, approachability, engagement, melodicness, softness, heaviness, dreaminess, emotionality, loudness, spectral_centroid, zero_crossing_rate, rms, spectral_flatness, spectral_rolloff85, spectral_flux, onset_rate, dynamic_range, low_band_ratio, mid_band_ratio, high_band_ratio, cluster_id, play_count, skip_count, complete_count, metadata_source, analyzed_level, analysis_version, analyzed_at, analysis_error, essentia_model_version, normalized_path, library_root_id, import_status, analysis_status, file_missing, file_size, file_mtime, file_inode, quick_hash, last_seen_at, last_error, embedding, text_embedding, playback_error_count, last_playback_error, last_playback_error_at, updated_at)
	VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,unixepoch())
	ON CONFLICT(id) DO UPDATE SET path=excluded.path, title=excluded.title, artist=excluded.artist, album=excluded.album, genre=excluded.genre, genre_primary=excluded.genre_primary, genre_detail=excluded.genre_detail, genre_tags=excluded.genre_tags, genre_label=excluded.genre_label, duration_ms=excluded.duration_ms, duration_label=excluded.duration_label, folder=excluded.folder, file_name=excluded.file_name, tempo=excluded.tempo, bpm_perceived=excluded.bpm_perceived, tempo_confidence=excluded.tempo_confidence, tempo_stability=excluded.tempo_stability, bpm_half=excluded.bpm_half, bpm_double=excluded.bpm_double, tempo_source=excluded.tempo_source, tempo_model_version=excluded.tempo_model_version, tempo_analyzed_at=excluded.tempo_analyzed_at, tempo_error=excluded.tempo_error, energy=excluded.energy, danceability=excluded.danceability, valence=excluded.valence, acousticness=excluded.acousticness, electronicness=excluded.electronicness, instrumentalness=excluded.instrumentalness, vocalness=excluded.vocalness, happy=excluded.happy, sad=excluded.sad, relaxed=excluded.relaxed, party=excluded.party, aggressive=excluded.aggressive, timbre_brightness=excluded.timbre_brightness, tonality=excluded.tonality, approachability=excluded.approachability, engagement=excluded.engagement, melodicness=excluded.melodicness, softness=excluded.softness, heaviness=excluded.heaviness, dreaminess=excluded.dreaminess, emotionality=excluded.emotionality, loudness=excluded.loudness, spectral_centroid=excluded.spectral_centroid, zero_crossing_rate=excluded.zero_crossing_rate, rms=excluded.rms, cluster_id=excluded.cluster_id, play_count=excluded.play_count, skip_count=excluded.skip_count, complete_count=excluded.complete_count, metadata_source=excluded.metadata_source, analyzed_level=excluded.analyzed_level, analysis_version=excluded.analysis_version, analyzed_at=excluded.analyzed_at, analysis_error=excluded.analysis_error, essentia_model_version=excluded.essentia_model_version, normalized_path=excluded.normalized_path, library_root_id=excluded.library_root_id, import_status=excluded.import_status, analysis_status=excluded.analysis_status, file_missing=excluded.file_missing, file_size=excluded.file_size, file_mtime=excluded.file_mtime, file_inode=excluded.file_inode, quick_hash=excluded.quick_hash, last_seen_at=excluded.last_seen_at, last_error=excluded.last_error, embedding=excluded.embedding, text_embedding=excluded.text_embedding, playback_error_count=excluded.playback_error_count, last_playback_error=excluded.last_playback_error, last_playback_error_at=excluded.last_playback_error_at, spectral_flatness=excluded.spectral_flatness, spectral_rolloff85=excluded.spectral_rolloff85, spectral_flux=excluded.spectral_flux, onset_rate=excluded.onset_rate, dynamic_range=excluded.dynamic_range, low_band_ratio=excluded.low_band_ratio, mid_band_ratio=excluded.mid_band_ratio, high_band_ratio=excluded.high_band_ratio, updated_at=unixepoch()`,
		t.ID, t.Path, t.Title, t.Artist, t.Album, t.Genre, t.GenrePrimary, t.GenreDetail, t.GenreTagsJSON, t.GenreLabel, t.DurationMs, t.DurationLabel, t.Folder, t.FileName, t.Tempo, t.BPMPerceived, t.TempoConfidence, t.TempoStability, t.BPMHalf, t.BPMDouble, t.TempoSource, t.TempoModelVersion, t.TempoAnalyzedAt, t.TempoError, t.Energy, t.Danceability, t.Valence, t.Acousticness, t.Electronicness, t.Instrumentalness, t.Vocalness, t.Happy, t.Sad, t.Relaxed, t.Party, t.Aggressive, t.TimbreBrightness, t.Tonality, t.Approachability, t.Engagement, t.Melodicness, t.Softness, t.Heaviness, t.Dreaminess, t.Emotionality, t.Loudness, t.SpectralCentroid, t.ZeroCrossingRate, t.RMS, t.SpectralFlatness, t.SpectralRolloff85, t.SpectralFlux, t.OnsetRate, t.DynamicRange, t.LowBandRatio, t.MidBandRatio, t.HighBandRatio, t.ClusterID, t.PlayCount, t.SkipCount, t.CompleteCount, t.MetadataSource, t.AnalyzedLevel, t.AnalysisVersion, t.AnalyzedAt, t.AnalysisError, t.EssentiaModelVersion, t.NormalizedPath, t.LibraryRootID, t.ImportStatus, t.AnalysisStatus, boolInt(t.FileMissing), t.FileSize, t.FileMTime, t.FileInode, t.QuickHash, t.LastSeenAt, t.LastError, floatsToBytes(t.Embedding), floatsToBytes(t.TextEmbedding), t.PlaybackErrorCount, t.LastPlaybackError, t.LastPlaybackErrorAt); err != nil {
		dbLog.I("upsert tracks failed id=%s err=%v", t.ID, err)
		return err
	}
	if _, err = tx.Exec(`INSERT INTO tracks_fts(tracks_fts) VALUES('rebuild')`); err != nil {
		dbLog.I("upsert fts rebuild failed id=%s err=%v", t.ID, err)
		return err
	}
	if _, err = tx.Exec(`DELETE FROM track_ngrams WHERE track_id = ?`, t.ID); err != nil {
		dbLog.I("upsert ngrams delete failed id=%s err=%v", t.ID, err)
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO track_ngrams(track_id, gram) VALUES(?,?)`)
	if err != nil {
		dbLog.I("upsert ngrams prepare failed id=%s err=%v", t.ID, err)
		return err
	}
	defer stmt.Close()
	for _, g := range grams {
		if _, err = stmt.Exec(t.ID, g); err != nil {
			dbLog.I("upsert ngrams insert failed id=%s gram=%q err=%v", t.ID, g, err)
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		dbLog.I("upsert commit failed id=%s err=%v", t.ID, err)
		return err
	}
	dbLog.I("upsert done id=%s genrePrimary=%q genreLabel=%q genreTags=%s grams=%d ms=%d", t.ID, t.GenrePrimary, t.GenreLabel, t.GenreTagsJSON, len(grams), time.Since(start).Milliseconds())
	return nil
}

func (s *Store) ListTracks() ([]TrackRow, error) {
	rows, err := s.db.Query(`SELECT ` + trackSelectColumns + ` FROM tracks ORDER BY lower(title) ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrackRow
	for rows.Next() {
		t, err := scanTrackRow(rows)
		if err != nil {
			return nil, err
		}
		dbLog.I("scan track id=%s genrePrimary=%q genreLabel=%q genreTags=%s", t.ID, t.GenrePrimary, t.GenreLabel, t.GenreTagsJSON)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTrack(id string) (TrackRow, error) {
	row := s.db.QueryRow(`SELECT `+trackSelectColumns+` FROM tracks WHERE id = ?`, id)
	t, err := scanTrackRow(row)
	if err != nil {
		return TrackRow{}, err
	}
	dbLog.I("scan track id=%s genrePrimary=%q genreLabel=%q genreTags=%s", t.ID, t.GenrePrimary, t.GenreLabel, t.GenreTagsJSON)
	return t, nil
}

func (s *Store) TrackByPath(path string) (TrackRow, bool, error) {
	path = filepath.Clean(path)
	row := s.db.QueryRow(`SELECT `+trackSelectColumns+` FROM tracks WHERE path = ? OR normalized_path = ? LIMIT 1`, path, path)
	t, err := scanTrackRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TrackRow{}, false, nil
	}
	if err != nil {
		return TrackRow{}, false, err
	}
	return t, true, nil
}

func (s *Store) SearchTracks(query string, grams []string, limit int) ([]SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		rows, err := s.db.Query(`SELECT `+trackSelectColumns+` FROM tracks ORDER BY (play_count * 2 + complete_count - skip_count) DESC, updated_at DESC, lower(title) ASC LIMIT ?`, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := make([]SearchHit, 0, limit)
		for rows.Next() {
			t, err := scanTrackRow(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, SearchHit{TrackRow: t, Final: float64(t.PlayCount*2 + t.CompleteCount - t.SkipCount)})
		}
		return out, rows.Err()
	}
	ftsQuery := buildFTSQuery(query)
	res := map[string]*SearchHit{}
	rows, err := s.db.Query(`SELECT `+prefixColumns("t.")+`, bm25(tracks_fts, 10.0, 7.0, 3.0, 2.0, 1.5, 0.5, 1.0) FROM tracks_fts JOIN tracks t ON t.rowid = tracks_fts.rowid WHERE tracks_fts MATCH ? ORDER BY bm25(tracks_fts, 10.0, 7.0, 3.0, 2.0, 1.5, 0.5, 1.0) LIMIT ?`, ftsQuery, limit*3)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			track, bm25, err := scanTrackRowWithScore(rows)
			if err != nil {
				return nil, err
			}
			res[track.ID] = &SearchHit{TrackRow: track, BM25: -bm25}
		}
	}
	if len(grams) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(grams)), ",")
		args := make([]any, 0, len(grams)+1)
		for _, g := range grams {
			args = append(args, g)
		}
		args = append(args, limit*4)
		nrows, err := s.db.Query(`SELECT `+prefixColumns("t.")+`, COUNT(*) AS grams_hit FROM track_ngrams ng JOIN tracks t ON t.id = ng.track_id WHERE ng.gram IN (`+placeholders+`) GROUP BY t.id ORDER BY grams_hit DESC LIMIT ?`, args...)
		if err == nil {
			defer nrows.Close()
			for nrows.Next() {
				track, hits, err := scanTrackRowWithHits(nrows)
				if err != nil {
					return nil, err
				}
				item, ok := res[track.ID]
				if !ok {
					item = &SearchHit{TrackRow: track}
					res[track.ID] = item
				}
				item.Ngram = float64(hits) / float64(max(1, len(grams)))
			}
		}
	}
	out := make([]SearchHit, 0, len(res))
	for _, item := range res {
		exact := 0.0
		lower := strings.ToLower(query)
		if strings.Contains(strings.ToLower(item.Title), lower) {
			exact += 0.18
		}
		if strings.HasPrefix(strings.ToLower(item.Title), lower) {
			exact += 0.2
		}
		pop := float64(item.PlayCount) * 0.03
		item.Final = item.BM25*0.72 + item.Ngram*0.18 + exact + pop
		out = append(out, *item)
	}
	sortSearch(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) RecordEvent(trackID, eventType string, positionMs, durationMs int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO play_events(track_id, event_type, position_ms, duration_ms, created_at) VALUES(?,?,?,?,unixepoch())`, trackID, eventType, positionMs, durationMs); err != nil {
		return err
	}
	switch eventType {
	case "play_start":
		_, err = tx.Exec(`UPDATE tracks SET play_count = play_count + 1, updated_at = unixepoch() WHERE id = ?`, trackID)
	case "skip", "early_skip", "late_skip", "manual_next":
		_, err = tx.Exec(`UPDATE tracks SET skip_count = skip_count + 1, updated_at = unixepoch() WHERE id = ?`, trackID)
	case "play_complete":
		_, err = tx.Exec(`UPDATE tracks SET complete_count = complete_count + 1, updated_at = unixepoch() WHERE id = ?`, trackID)
	case "playback_failed", "technical_skip", "playback_started":
		err = nil
	}
	if err != nil {
		return err
	}
	if err := s.upsertTrackFeedback(tx, trackID, eventType, positionMs, durationMs); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) History(limit int) ([]HistoryRow, error) {
	rows, err := s.db.Query(`SELECT track_id, position_ms, CASE WHEN duration_ms > 0 THEN CAST(position_ms AS REAL)/duration_ms ELSE 0 END, created_at FROM play_events WHERE event_type IN ('play_start','seek','play_complete') ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HistoryRow{}
	seen := map[string]bool{}
	for rows.Next() {
		var h HistoryRow
		if err := rows.Scan(&h.TrackID, &h.PositionMs, &h.Progress, &h.PlayedAtUnix); err != nil {
			return nil, err
		}
		if seen[h.TrackID] {
			continue
		}
		seen[h.TrackID] = true
		h.ProgressLabel = formatProgressLabel(h.PositionMs)
		h.PlayedAtLabel = humanTime(h.PlayedAtUnix)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) RecordTransitionEvent(prevTrackID, nextTrackID, strategy, bucket, transitionType, energyDirection string, isBridge, isDiscovery bool) error {
	_, err := s.db.Exec(`INSERT INTO transition_events(prev_track_id, next_track_id, strategy, bucket, transition_type, energy_direction, is_bridge, is_discovery, created_at) VALUES(?,?,?,?,?,?,?,?,unixepoch())`, prevTrackID, nextTrackID, strategy, bucket, transitionType, energyDirection, boolInt(isBridge), boolInt(isDiscovery))
	return err
}

func (s *Store) SaveRayState(id string, contentMode, sortMode string, isManualOrder bool, manualUpdatedAt int64, parentRayID string, revision int) error {
	_, err := s.db.Exec(`UPDATE rays SET content_mode=?, sort_mode=?, is_manual_order=?, manual_updated_at=?, parent_ray_id=?, revision=? WHERE id=?`,
		contentMode, sortMode, boolToInt(isManualOrder), manualUpdatedAt, parentRayID, revision, id)
	return err
}

func (s *Store) SaveRay(id, name, seedTrackID, currentTrackID, resumeLabel string, positionMs int, active bool, tracks []RayTrackRow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if active {
		if _, err = tx.Exec(`UPDATE rays SET active = 0`); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`INSERT INTO rays(id, name, seed_track_id, current_track_id, resume_label, position_ms, active, created_at) VALUES(?,?,?,?,?,?,?,unixepoch()) ON CONFLICT(id) DO UPDATE SET name=excluded.name, seed_track_id=excluded.seed_track_id, current_track_id=excluded.current_track_id, resume_label=excluded.resume_label, position_ms=excluded.position_ms, active=excluded.active`, id, name, seedTrackID, currentTrackID, resumeLabel, positionMs, boolInt(active))
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM ray_tracks WHERE ray_id = ?`, id); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO ray_tracks(ray_id, track_id, position_index, subtitle, reason, bucket, strategy, score, is_current) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range tracks {
		if _, err = stmt.Exec(id, item.TrackID, item.PositionIndex, item.Subtitle, item.Reason, item.Bucket, item.Strategy, item.Score, boolInt(item.IsCurrent)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListRays() ([]RaySummaryRow, error) {
	rows, err := s.db.Query(`SELECT r.id, r.name, (SELECT COUNT(*) FROM ray_tracks rt WHERE rt.ray_id = r.id), r.current_track_id, COALESCE(t.title, r.name), r.resume_label, r.position_ms, r.active FROM rays r LEFT JOIN tracks t ON t.id = r.current_track_id ORDER BY r.active DESC, r.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RaySummaryRow
	for rows.Next() {
		var r RaySummaryRow
		var active int
		if err := rows.Scan(&r.ID, &r.Name, &r.TrackCount, &r.CurrentTrackID, &r.CurrentTrackName, &r.ResumeLabel, &r.PositionMs, &active); err != nil {
			return nil, err
		}
		r.Active = active == 1
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) GetRayQueue(rayID string) ([]RayTrackRow, error) {
	rows, err := s.db.Query(`
		SELECT
			rt.track_id,
			t.title,
			rt.subtitle,
			COALESCE(t.artist, ''),
			COALESCE(t.album, ''),
			COALESCE(t.genre_primary, ''),
			COALESCE(t.genre_label, ''),
			COALESCE(t.genre_detail, ''),
			COALESCE(t.genre_tags, '[]'),
			COALESCE(t.duration_ms, 0),
			COALESCE(t.duration_label, ''),
			rt.is_current,
			COALESCE(rt.reason, ''),
			COALESCE(rt.bucket, ''),
			COALESCE(rt.strategy, ''),
			COALESCE(rt.score, 0),
			rt.position_index
		FROM ray_tracks rt
		JOIN tracks t ON t.id = rt.track_id
		WHERE rt.ray_id = ?
		ORDER BY rt.position_index ASC
	`, rayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RayTrackRow
	for rows.Next() {
		var q RayTrackRow
		var current int
		var genreTagsJSON string

		if err := rows.Scan(
			&q.TrackID,
			&q.Title,
			&q.Subtitle,
			&q.Artist,
			&q.Album,
			&q.GenrePrimary,
			&q.GenreLabel,
			&q.GenreDetail,
			&genreTagsJSON,
			&q.DurationMs,
			&q.DurationLabel,
			&current,
			&q.Reason,
			&q.Bucket,
			&q.Strategy,
			&q.Score,
			&q.PositionIndex,
		); err != nil {
			return nil, err
		}

		q.IsCurrent = current == 1
		q.GenreTags = []onnx.GenreTag{}
		if strings.TrimSpace(genreTagsJSON) != "" {
			if err := json.Unmarshal([]byte(genreTagsJSON), &q.GenreTags); err != nil {
				q.GenreTags = []onnx.GenreTag{}
			}
		}
		out = append(out, q)
	}

	return out, rows.Err()
}

func (s *Store) GetRaySeedTrackID(rayID string) (string, error) {
	var seedTrackID string
	err := s.db.QueryRow(
		`SELECT seed_track_id FROM rays WHERE id = ?`,
		rayID,
	).Scan(&seedTrackID)
	return seedTrackID, err
}

func (s *Store) UpdateRayState(rayID, trackID string, positionMs int, resumeLabel string) error {
	_, err := s.db.Exec(`UPDATE rays SET current_track_id=?, position_ms=?, resume_label=? WHERE id=?`, trackID, positionMs, resumeLabel, rayID)
	return err
}

func (s *Store) SetAppState(st AppStateRow) error {
	_, err := s.db.Exec(`UPDATE app_state SET current_track_id=?, position_ms=?, volume=?, playing=?, current_ray_id=? WHERE id=1`, st.CurrentTrackID, st.PositionMs, st.Volume, boolInt(st.Playing), st.CurrentRayID)
	return err
}

func (s *Store) SetAppSettings(st AppStateRow) error {
	if strings.TrimSpace(st.FFmpegPath) == "" {
		st.FFmpegPath = "ffmpeg"
	}
	if strings.TrimSpace(st.FFprobePath) == "" {
		st.FFprobePath = "ffprobe"
	}
	_, err := s.db.Exec(`UPDATE app_state SET onnx_runtime_path=?, minilm_model_dir=?, essentia_model_dir=?, ffmpeg_path=?, ffprobe_path=?, repeat_ray=?, extend_ray=?, emoflow_ui_enabled=?, emoflow_ui_intensity=?, emoflow_ui_animate_track=?, emoflow_ui_respect_reduced=? WHERE id=1`, st.OnnxRuntimePath, st.MiniLMModelDir, st.EssentiaModelDir, st.FFmpegPath, st.FFprobePath, boolInt(st.RepeatRay), boolInt(st.ExtendRay), boolInt(st.EmoFlowUIEnabled), st.EmoFlowUIIntensity, boolInt(st.EmoFlowUIAnimateTrack), boolInt(st.EmoFlowUIRespectReduced))
	return err
}

func (s *Store) GetAppState() (AppStateRow, error) {
	var st AppStateRow
	var playing, repeatRay, extendRay, emoEnabled, emoAnimate, emoReduced int
	err := s.db.QueryRow(`SELECT current_track_id, position_ms, volume, playing, current_ray_id, onnx_runtime_path, minilm_model_dir, essentia_model_dir, ffmpeg_path, ffprobe_path, repeat_ray, extend_ray, emoflow_ui_enabled, emoflow_ui_intensity, emoflow_ui_animate_track, emoflow_ui_respect_reduced FROM app_state WHERE id=1`).Scan(&st.CurrentTrackID, &st.PositionMs, &st.Volume, &playing, &st.CurrentRayID, &st.OnnxRuntimePath, &st.MiniLMModelDir, &st.EssentiaModelDir, &st.FFmpegPath, &st.FFprobePath, &repeatRay, &extendRay, &emoEnabled, &st.EmoFlowUIIntensity, &emoAnimate, &emoReduced)
	st.Playing = playing == 1
	st.RepeatRay = repeatRay != 0
	st.ExtendRay = extendRay == 1
	st.EmoFlowUIEnabled = emoEnabled == 1
	st.EmoFlowUIAnimateTrack = emoAnimate == 1
	st.EmoFlowUIRespectReduced = emoReduced == 1
	if st.EmoFlowUIIntensity <= 0 {
		st.EmoFlowUIIntensity = 1.0
	}
	return st, err
}

func (s *Store) SavePlaybackSession(st PlaybackSessionRow) error {
	if strings.TrimSpace(st.ID) == "" {
		st.ID = "last"
	}
	if st.UpdatedAt == 0 {
		st.UpdatedAt = time.Now().UnixMilli()
	}

	_, err := s.db.Exec(`
		INSERT INTO playback_session(
			id, status, current_track_id, current_path,
			position_ms, duration_ms,
			queue_id, queue_index, queue_json,
			ray_id, ray_seed_track_id, ray_mode, ray_config_json,
			updated_at, last_error
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status,
			current_track_id=excluded.current_track_id,
			current_path=excluded.current_path,
			position_ms=excluded.position_ms,
			duration_ms=excluded.duration_ms,
			queue_id=excluded.queue_id,
			queue_index=excluded.queue_index,
			queue_json=excluded.queue_json,
			ray_id=excluded.ray_id,
			ray_seed_track_id=excluded.ray_seed_track_id,
			ray_mode=excluded.ray_mode,
			ray_config_json=excluded.ray_config_json,
			updated_at=excluded.updated_at,
			last_error=excluded.last_error
	`, st.ID, st.Status, st.CurrentTrackID, st.CurrentPath,
		st.PositionMs, st.DurationMs,
		st.QueueID, st.QueueIndex, st.QueueJSON,
		st.RayID, st.RaySeedTrackID, st.RayMode, st.RayConfigJSON,
		st.UpdatedAt, st.LastError)
	return err
}

func (s *Store) LoadLastPlaybackSession() (PlaybackSessionRow, error) {
	var st PlaybackSessionRow
	err := s.db.QueryRow(`
		SELECT id, status, current_track_id, current_path,
			position_ms, duration_ms,
			queue_id, queue_index, queue_json,
			ray_id, ray_seed_track_id, ray_mode, ray_config_json,
			updated_at, last_error
		FROM playback_session
		WHERE id = 'last'
	`).Scan(&st.ID, &st.Status, &st.CurrentTrackID, &st.CurrentPath,
		&st.PositionMs, &st.DurationMs,
		&st.QueueID, &st.QueueIndex, &st.QueueJSON,
		&st.RayID, &st.RaySeedTrackID, &st.RayMode, &st.RayConfigJSON,
		&st.UpdatedAt, &st.LastError)
	return st, err
}

func (s *Store) ListTrackFeedback() (map[string]TrackFeedbackRow, error) {
	rows, err := s.db.Query(`SELECT track_id, last_played_at, last_skipped_at, avg_completion, affinity, play_events, skip_events, complete_events, last_event_type FROM track_feedback`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]TrackFeedbackRow{}
	for rows.Next() {
		var item TrackFeedbackRow
		if err := rows.Scan(&item.TrackID, &item.LastPlayedAt, &item.LastSkippedAt, &item.AvgCompletion, &item.Affinity, &item.PlayEvents, &item.SkipEvents, &item.CompleteEvents, &item.LastEventType); err != nil {
			return nil, err
		}
		out[item.TrackID] = item
	}
	return out, rows.Err()
}

func (s *Store) RecentTrackIDs(limit int) ([]string, error) {
	rows, err := s.db.Query(`SELECT track_id FROM play_events WHERE event_type IN ('play_start','play_30s','play_half','play_80','play_complete','resume_ray') ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) ListStrategyStats() (map[string]StrategyStat, error) {
	rows, err := s.db.Query(`SELECT strategy, plays, reward FROM strategy_stats`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]StrategyStat{}
	for rows.Next() {
		var st StrategyStat
		if err := rows.Scan(&st.Strategy, &st.Plays, &st.Reward); err != nil {
			return nil, err
		}
		out[st.Strategy] = st
	}
	return out, rows.Err()
}

func (s *Store) UpdateStrategyReward(strategy string, reward float64) error {
	_, err := s.db.Exec(`INSERT INTO strategy_stats(strategy, plays, reward) VALUES(?, 1, ?) ON CONFLICT(strategy) DO UPDATE SET plays = plays + 1, reward = reward + excluded.reward`, strategy, reward)
	return err
}

func (s *Store) upsertTrackFeedback(tx *sql.Tx, trackID, eventType string, positionMs, durationMs int) error {
	var current TrackFeedbackRow
	err := tx.QueryRow(`SELECT track_id, last_played_at, last_skipped_at, avg_completion, affinity, play_events, skip_events, complete_events, last_event_type FROM track_feedback WHERE track_id = ?`, trackID).Scan(&current.TrackID, &current.LastPlayedAt, &current.LastSkippedAt, &current.AvgCompletion, &current.Affinity, &current.PlayEvents, &current.SkipEvents, &current.CompleteEvents, &current.LastEventType)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	now := time.Now().Unix()
	completion := 0.0
	if durationMs > 0 {
		completion = math.Min(1, math.Max(0, float64(positionMs)/float64(durationMs)))
	}
	playCount := current.PlayEvents
	if shouldCountPlayEvent(eventType) {
		playCount++
		current.AvgCompletion = ((current.AvgCompletion * float64(max(0, current.PlayEvents))) + completion) / float64(playCount)
		current.LastPlayedAt = now
	}
	if shouldCountSkipEvent(eventType) {
		current.SkipEvents++
		current.LastSkippedAt = now
	}
	if eventType == "play_complete" {
		current.CompleteEvents++
	}
	current.PlayEvents = playCount
	current.Affinity = clampUnitRange(current.Affinity + feedbackReward(eventType, completion)*0.18)
	current.LastEventType = eventType
	_, err = tx.Exec(`INSERT INTO track_feedback(track_id, last_played_at, last_skipped_at, avg_completion, affinity, play_events, skip_events, complete_events, last_event_type)
	VALUES(?,?,?,?,?,?,?,?,?)
	ON CONFLICT(track_id) DO UPDATE SET
		last_played_at=excluded.last_played_at,
		last_skipped_at=excluded.last_skipped_at,
		avg_completion=excluded.avg_completion,
		affinity=excluded.affinity,
		play_events=excluded.play_events,
		skip_events=excluded.skip_events,
		complete_events=excluded.complete_events,
		last_event_type=excluded.last_event_type`, trackID, current.LastPlayedAt, current.LastSkippedAt, current.AvgCompletion, current.Affinity, current.PlayEvents, current.SkipEvents, current.CompleteEvents, current.LastEventType)
	return err
}

func shouldCountPlayEvent(eventType string) bool {
	switch eventType {
	case "play_start", "play_30s", "play_half", "play_80", "play_complete", "resume_ray":
		return true
	case "playback_failed", "technical_skip", "playback_started":
		return false
	default:
		return false
	}
}

func shouldCountSkipEvent(eventType string) bool {
	switch eventType {
	case "skip", "early_skip", "late_skip", "manual_next":
		return true
	case "playback_failed", "technical_skip", "playback_started":
		return false
	default:
		return false
	}
}

func feedbackReward(eventType string, completion float64) float64 {
	switch eventType {
	case "play_start":
		return 0.05
	case "play_30s":
		return 0.10
	case "play_half":
		return 0.35
	case "play_80":
		return 0.70
	case "play_complete":
		return 1.0
	case "resume_ray":
		return 0.40
	case "seek_forward":
		return -0.15
	case "seek_backward":
		return 0.20
	case "jump_in_queue":
		return 0.10
	case "manual_next":
		if completion >= 0.60 {
			return -0.10
		}
		if completion < 0.30 {
			return -0.80
		}
		return -0.40
	case "late_skip":
		return -0.35
	case "skip", "early_skip":
		return -1.0
	case "playback_failed", "technical_skip", "playback_started":
		return 0
	default:
		return 0
	}
}

func clampUnitRange(v float64) float64 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}

func (s *Store) MarkPlaybackFailed(trackID, kind, message string) error {
	_, err := s.db.Exec(`UPDATE tracks SET playback_error_count = playback_error_count + 1, last_playback_error = ?, last_playback_error_at = ?, updated_at = unixepoch() WHERE id = ?`, kind+": "+message, time.Now().Unix(), trackID)
	return err
}

func (s *Store) MarkPlaybackSucceeded(trackID string) error {
	_, err := s.db.Exec(`UPDATE tracks SET playback_error_count = 0, last_playback_error = '', last_playback_error_at = 0, updated_at = unixepoch() WHERE id = ?`, trackID)
	return err
}

func buildFTSQuery(query string) string {
	parts := strings.Fields(strings.ToLower(query))
	if len(parts) == 0 {
		return ""
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, p+"*")
	}
	return strings.Join(out, " AND ")
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func humanTime(ts int64) string {
	now := time.Now().Unix()
	diff := now - ts
	if diff < 86400 {
		return "сегодня"
	}
	if diff < 172800 {
		return "вчера"
	}
	return time.Unix(ts, 0).Format("02.01.2006")
}
func formatProgressLabel(ms int) string {
	return fmt.Sprintf("остановлено на %d:%02d", ms/60000, (ms/1000)%60)
}

func (s *Store) migrateTracksFTS() error {
	var createSQL string
	err := s.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'tracks_fts'`).Scan(&createSQL)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	createSQLLower := strings.ToLower(createSQL)
	staleFTS := strings.Contains(createSQLLower, "content=''") || strings.Contains(createSQLLower, "tags")
	if staleFTS {
		if err := s.dropStaleTracksFTS(); err != nil {
			return fmt.Errorf("drop stale tracks_fts: %w", err)
		}
	}
	if _, err := s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS tracks_fts USING fts5(title, artist, album, genre, file_name, folder, content='tracks')`); err != nil {
		return fmt.Errorf("create tracks_fts: %w", err)
	}
	if _, err := s.db.Exec(`INSERT INTO tracks_fts(rowid, title, artist, album, genre, file_name, folder)
		SELECT rowid, title, artist, album, genre, file_name, folder
		FROM tracks
		WHERE rowid NOT IN (SELECT rowid FROM tracks_fts)`); err != nil {
		return fmt.Errorf("backfill tracks_fts: %w", err)
	}
	return nil
}

func (s *Store) dropStaleTracksFTS() error {
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS tracks_fts`,
		`DROP TABLE IF EXISTS tracks_fts_data`,
		`DROP TABLE IF EXISTS tracks_fts_idx`,
		`DROP TABLE IF EXISTS tracks_fts_docsize`,
		`DROP TABLE IF EXISTS tracks_fts_config`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	var leftOver int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'tracks_fts%'`).Scan(&leftOver); err != nil {
		return err
	}
	if leftOver == 0 {
		return nil
	}
	if _, err := s.db.Exec(`PRAGMA writable_schema=ON`); err != nil {
		return err
	}
	defer func() {
		_, _ = s.db.Exec(`PRAGMA writable_schema=OFF`)
	}()
	if _, err := s.db.Exec(`DELETE FROM sqlite_master WHERE name = 'tracks_fts' OR name LIKE 'tracks_fts_%'`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM sqlite_sequence WHERE name LIKE 'tracks_fts%'`); err != nil {
		return err
	}
	return nil
}

func resolveWritableRoot(appName string) (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	root := filepath.Join(cfgDir, appName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create storage dir %q: %w", root, err)
	}
	return root, nil
}
