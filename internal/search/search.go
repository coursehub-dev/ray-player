package search

import (
	"fmt"
	"strings"

	"ray-player1/internal/db"
	"ray-player1/internal/library"
	"ray-player1/internal/logx"
)

var searchLog = logx.New("search")

type Result struct {
	Track       library.Track `json:"track"`
	Score       float64       `json:"score"`
	Explanation string        `json:"explanation"`
}

type Service struct{ store *db.Store }

func NewService(store *db.Store) *Service { return &Service{store: store} }
func (s *Service) Rebuild()               {}

func (s *Service) Query(query string, limit int) []Result {
	query = strings.TrimSpace(query)
	hits, err := s.store.SearchTracks(query, grams(query), limit)
	if err != nil {
		return nil
	}
	if query == "" {
		searchLog.D("empty query limit=%d results=%d", limit, len(hits))
	} else {
		searchLog.D("query=%q results=%d", query, len(hits))
	}
	out := make([]Result, 0, len(hits))
	for _, hit := range hits {
		if query != "" {
			searchLog.T("hit id=%s title=%q final=%.4f bm25=%.4f ngram=%.4f analyzed=%d", hit.ID, hit.Title, hit.Final, hit.BM25, hit.Ngram, hit.AnalyzedLevel)
		}
		track := library.TrackFromRow(hit.TrackRow)
		out = append(out, Result{Track: track, Score: hit.Final, Explanation: explainHit(hit)})
	}
	return out
}

func explainHit(hit db.SearchHit) string {
	parts := []string{}
	if hit.AnalyzedLevel >= 2 {
		parts = append(parts, "audio analyzed")
	} else {
		parts = append(parts, "fast import")
	}
	if hit.BM25 > 0.4 {
		parts = append(parts, "strong title/tag match")
	}
	if hit.Ngram > 0.2 {
		parts = append(parts, "fuzzy filename match")
	}
	if hit.PlayCount > 0 {
		parts = append(parts, fmt.Sprintf("played %d", hit.PlayCount))
	}
	if strings.TrimSpace(hit.GenrePrimary) != "" {
		parts = append(parts, hit.GenrePrimary)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("BM25 %.2f · ngram %.2f", hit.BM25, hit.Ngram)
	}
	return strings.Join(parts, " · ")
}

func grams(s string) []string {
	s = normalize(s)
	if len(s) < 3 {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	out := []string{}
	seen := map[string]struct{}{}
	for i := 0; i <= len(s)-3; i++ {
		g := s[i : i+3]
		if _, ok := seen[g]; !ok {
			seen[g] = struct{}{}
			out = append(out, g)
		}
	}
	return out
}
func normalize(s string) string {
	b := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 32
		}
		switch r {
		case 'Ё':
			r = 'е'
		case 'ё':
			r = 'е'
		case '_', '-', '.', '/', '\\':
			r = ' '
		}
		b = append(b, r)
	}
	cleaned := strings.Join(strings.Fields(string(b)), " ")
	for _, noise := range []string{"320kbps", "official", "track", "audio"} {
		cleaned = strings.ReplaceAll(cleaned, noise, " ")
	}
	return strings.Join(strings.Fields(cleaned), " ")
}
