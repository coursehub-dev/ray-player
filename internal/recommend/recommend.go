package recommend

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"ray-player1/internal/emotion"
	"ray-player1/internal/events"
	"ray-player1/internal/library"
	"ray-player1/internal/logx"
	"ray-player1/internal/modelcontract"
	"ray-player1/internal/rays"
)

var recommendLog = logx.New("recommend")

var ErrRayBuildCanceled = errors.New("ray build canceled")

type emotionCache struct {
	n FeatureNormalizer
	m map[string]emotion.Result
}

func newEmotionCache(n FeatureNormalizer) *emotionCache {
	return &emotionCache{n: n, m: map[string]emotion.Result{}}
}

func (c *emotionCache) Get(t library.Track) emotion.Result {
	if c == nil {
		return emotion.Compute(t, FeatureNormalizer{})
	}
	key := t.ID
	if key == "" {
		key = t.Path
	}
	if key == "" {
		return emotion.Compute(t, c.n)
	}
	if res, ok := c.m[key]; ok {
		return res
	}
	res := emotion.Compute(t, c.n)
	c.m[key] = res
	return res
}

func emotionFromContext(ctx rankingContext, t library.Track) emotion.Result {
	if ctx.EmotionCache != nil {
		return ctx.EmotionCache.Get(t)
	}
	return emotion.Compute(t, ctx.Normalizer)
}

type Service struct {
	events *events.Service
	rays   *rays.Service
}

type scored struct {
	track        library.Track
	score        float64
	reason       string
	strategy     string
	bucket       string
	trackInsight rays.QueueInsight
}

type RawMoodFeatures struct {
	Danceability     float64
	Happy            float64
	Sad              float64
	Relaxed          float64
	Party            float64
	Aggressive       float64
	Acousticness     float64
	Electronicness   float64
	Instrumentalness float64
	Vocalness        float64
	Brightness       float64
	Tonality         float64
	Melodicness      float64
	Softness         float64
	Heaviness        float64
	Dreaminess       float64
	Emotionality     float64
	Valence          float64
	TempoPulse       float64
	ZeroCrossingRate float64
	SpectralCentroid float64
}

type InterpretedMood struct {
	Movement   float64
	Drive      float64
	Energy     float64
	Intensity  float64
	Calmness   float64
	Tension    float64
	Atmosphere float64
	Club       float64
	Darkness   float64
	Valence    float64
	Texture    float64
	Vocalness  float64
	Confidence float64
}

type MoodBasis struct {
	Motion   float64
	Pressure float64
	Calmness float64
	Coolness float64
	Warmth   float64
	Texture  float64
	Edge     float64
	Valence  float64
	Vocality float64
}

type calibratedMood struct {
	SoftEnergy       float64
	Brightness       float64
	Darkness         float64
	Calmness         float64
	Aggression       float64
	Pressure         float64
	Movement         float64
	Edge             float64
	Electronicness   float64
	Organicness      float64
	VocalPresence    float64
	Melodicness      float64
	Softness         float64
	Heaviness        float64
	Dreaminess       float64
	Emotionality     float64
	TimbreBrightness float64
	Tonality         float64
	Approachability  float64
	Engagement       float64
}

type feedbackItem struct {
	lastPlayedAt   int64
	lastSkippedAt  int64
	avgCompletion  float64
	affinity       float64
	playEvents     int
	skipEvents     int
	completeEvents int
	lastEventType  string
}

type trustProfile struct {
	Mood      float64
	Tempo     float64
	Embedding float64
	Analysis  float64
}

func trackTrust(t library.Track) trustProfile {
	mood := moodConfidence(t)
	tempo := tempoTrust(t)
	embedding := embeddingTrust(t)
	analysis := clamp01(mood*0.45 + tempo*0.25 + embedding*0.20 + analysisLevelTrust(t)*0.10)
	return trustProfile{Mood: mood, Tempo: tempo, Embedding: embedding, Analysis: analysis}
}

func analysisLevelTrust(t library.Track) float64 {
	switch {
	case t.AnalyzedLevel >= 2:
		return 1
	case t.AnalyzedLevel == 1:
		return 0.55
	default:
		return 0.20
	}
}

func tempoTrust(t library.Track) float64 {
	if t.BPMPerceived <= 0 && t.Tempo <= 0 {
		return 0
	}
	if strings.EqualFold(strings.TrimSpace(t.TempoSource), "error") || strings.TrimSpace(t.TempoError) != "" {
		return 0
	}
	if t.TempoConfidence <= 0 {
		return 0.10
	}
	return clamp01(t.TempoConfidence*0.70 + t.TempoStability*0.30)
}

func embeddingTrust(t library.Track) float64 {
	if len(t.Embedding) == 0 {
		return 0
	}
	return 1
}

func moodConfidence(t library.Track) float64 {
	values := []float64{
		t.Danceability, t.Valence, t.Happy, t.Sad, t.Relaxed,
		t.Party, t.Aggressive, t.Acousticness, t.Electronicness,
		t.Melodicness, t.Softness, t.Heaviness, t.Dreaminess,
		t.Emotionality,
	}
	nonZero := 0
	for _, v := range values {
		if v > 0.001 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			nonZero++
		}
	}
	base := float64(nonZero) / float64(len(values))
	if t.AnalyzedLevel < 2 {
		base *= 0.55
	}
	if t.TempoConfidence > 0 {
		base = clamp01(base*0.75 + clamp01(t.TempoConfidence)*0.25)
	}
	return clamp01(base)
}

const (
	bucketCore      = "core"
	bucketAdjacent  = "adjacent"
	bucketBridge    = "bridge"
	bucketDiscovery = "discovery"
	rayQueueMax     = 20
)

type RayTrajectoryMode string

const (
	TrajectoryContinueMood RayTrajectoryMode = "continue_mood"
	TrajectoryWarmUp       RayTrajectoryMode = "warm_up"
	TrajectoryCoolDown     RayTrajectoryMode = "cool_down"
	TrajectoryExplore      RayTrajectoryMode = "explore"
	TrajectoryDeepen       RayTrajectoryMode = "deepen"
)

type rankingContext struct {
	Seed         library.Track
	Mode         RayTrajectoryMode
	Position     int
	QueueLength  int
	TargetEnergy float64
	SessionMood  calibratedMood
	TargetMood   calibratedMood
	Exploration  float64
	Temperature  float64
	Normalizer   FeatureNormalizer
	EmotionCache *emotionCache

	TargetEnergyDelta float64
	TargetCalmDelta   float64
	TargetBrightDelta float64
	ContentMode       rays.ContentMode
}

type modeWeights struct {
	Base       float64
	Flow       float64
	Quota      float64
	Transition float64
	Novelty    float64
	Repeat     float64
	Jump       float64
	Skip       float64
	Volatility float64
}

func applyContentMode(
	ctx rankingContext,
	mode rays.ContentMode,
) rankingContext {
	ctx.ContentMode = mode

	switch mode {
	case rays.ContentWarmUp:
		ctx.TargetEnergyDelta = 0.22
		ctx.TargetCalmDelta = -0.12
		ctx.TargetBrightDelta = 0.08

	case rays.ContentCoolDown:
		ctx.TargetEnergyDelta = -0.24
		ctx.TargetCalmDelta = 0.20
		ctx.TargetBrightDelta = -0.04

	case rays.ContentIntensify:
		ctx.TargetEnergyDelta = 0.32
		ctx.TargetCalmDelta = -0.20

	case rays.ContentDeepen:
		ctx.TargetEnergyDelta = -0.06
		ctx.TargetCalmDelta = 0.12
		ctx.TargetBrightDelta = -0.12

	case rays.ContentExplore:
		ctx.TargetEnergyDelta = 0
		ctx.TargetCalmDelta = 0
		ctx.TargetBrightDelta = 0

	default:
		ctx.ContentMode = rays.ContentStable
	}

	return ctx
}

func weightsForMode(mode RayTrajectoryMode) modeWeights {
	switch mode {
	case TrajectoryWarmUp:
		return modeWeights{Base: 0.36, Flow: 0.36, Quota: 0.08, Transition: 0.06, Novelty: 0.04, Repeat: 0.10, Jump: 0.16, Skip: 0.14, Volatility: 0.10}
	case TrajectoryCoolDown:
		return modeWeights{Base: 0.34, Flow: 0.38, Quota: 0.08, Transition: 0.06, Novelty: 0.03, Repeat: 0.10, Jump: 0.20, Skip: 0.15, Volatility: 0.14}
	case TrajectoryExplore:
		return modeWeights{Base: 0.32, Flow: 0.30, Quota: 0.08, Transition: 0.06, Novelty: 0.12, Repeat: 0.10, Jump: 0.14, Skip: 0.16, Volatility: 0.06}
	case TrajectoryDeepen:
		return modeWeights{Base: 0.36, Flow: 0.38, Quota: 0.08, Transition: 0.06, Novelty: 0.03, Repeat: 0.10, Jump: 0.17, Skip: 0.14, Volatility: 0.12}
	default:
		return modeWeights{Base: 0.38, Flow: 0.34, Quota: 0.08, Transition: 0.06, Novelty: 0.05, Repeat: 0.10, Jump: 0.16, Skip: 0.14, Volatility: 0.10}
	}
}

func trajectoryModeScore(
	ctx rankingContext,
	seed library.Track,
	candidate library.Track,
) float64 {
	if ctx.ContentMode == rays.ContentStable || ctx.ContentMode == "" {
		return 0
	}

	seedMood := emotionFromContext(ctx, seed)
	candidateMood := emotionFromContext(ctx, candidate)

	energyDelta :=
		candidateMood.Basis.Joy -
			seedMood.Basis.Joy
	calmDelta :=
		candidateMood.Basis.Serenity -
			seedMood.Basis.Serenity
	brightDelta :=
		candidateMood.Basis.Brightness -
			seedMood.Basis.Brightness

	targetDistance :=
		math.Abs(
			energyDelta-
				ctx.TargetEnergyDelta,
		) +
			0.75*math.Abs(
				calmDelta-
					ctx.TargetCalmDelta,
			) +
			0.45*math.Abs(
				brightDelta-
					ctx.TargetBrightDelta,
			)

	modeFit := 1 - math.Min(1, targetDistance)

	if ctx.ContentMode == rays.ContentExplore {
		similarity := 1 - emotion.Distance(
			seedMood.Basis,
			candidateMood.Basis,
		)
		switch {
		case similarity >= 0.55 && similarity <= 0.78:
			modeFit = 1
		case similarity > 0.90:
			modeFit = 0.25
		default:
			modeFit = 0.55
		}
	}

	return 0.24 * modeFit
}

type quotaState struct {
	Target map[string]int
	Used   map[string]int
	Total  int
}

func newQuotaState(total int) quotaState {
	return quotaState{
		Target: map[string]int{
			bucketCore:      max(8, int(float64(total)*0.52)),
			bucketBridge:    max(3, int(float64(total)*0.20)),
			bucketAdjacent:  max(3, int(float64(total)*0.18)),
			bucketDiscovery: max(1, int(float64(total)*0.10)),
		},
		Used:  map[string]int{},
		Total: total,
	}
}

func quotaFromQueue(queue []rays.QueueItem, total int) quotaState {
	q := newQuotaState(total)
	for _, item := range queue {
		if item.Bucket != "" {
			q.Used[item.Bucket]++
		}
	}
	return q
}

func (q quotaState) Score(bucket string, position int) float64 {
	target := q.Target[bucket]
	used := q.Used[bucket]
	if target <= 0 {
		if used > 0 {
			return -0.4
		}
		return 0
	}
	ratio := float64(used) / float64(target)
	if ratio < 0.75 {
		return 0.18
	}
	if ratio <= 1.05 {
		return 0.04
	}
	return -0.20
}

func scoreNextCandidate(
	seed library.Track,
	prev library.Track,
	history []library.Track,
	c scored,
	ctx rankingContext,
	quota quotaState,
	feedback map[string]feedbackItem,
	strategyStats map[string]strategyStat,
	recentTrackSet map[string]bool,
) float64 {
	w := weightsForMode(ctx.Mode)
	flow := flowScore(ctx, seed, prev, history, c.track)
	quotaFit := quota.Score(c.bucket, ctx.Position)
	transitionFb := transitionFeedbackScore(strategyStats, prev, c.track)
	novelty := controlledDiscoveryScore(c, history, ctx)
	repeatPenalty := repetitionPenalty(c.track, history, recentTrackSet[c.track.ID])
	jump := blendedJumpPenalty(ctx, prev, c.track, ctx.Mode)
	skip := skipRiskScore(c.track, feedback[c.track.ID])
	volatility := clamp01(sessionVolatility(history, c.track)*0.40 + perceptualSessionVolatility(history, c.track, ctx.Normalizer)*0.60)

	total := c.score*w.Base +
		flow*w.Flow +
		quotaFit*w.Quota +
		transitionFb*w.Transition +
		novelty*w.Novelty -
		repeatPenalty*w.Repeat -
		jump*w.Jump -
		skip*w.Skip -
		volatility*w.Volatility

	pRisk := perceptualHardJumpRisk(ctx, prev, c.track, ctx.Mode)
	if c.bucket == bucketBridge {
		pRisk *= 0.72
	}
	if c.bucket == bucketDiscovery {
		pRisk *= 1.10
	}
	total -= pRisk * 0.22
	if pRisk > 0.62 && c.bucket != bucketBridge {
		total -= 0.30
	}
	return total
}

func controlledDiscoveryScore(c scored, history []library.Track, ctx rankingContext) float64 {
	n := noveltyScore(c.track)
	if c.bucket != bucketDiscovery {
		return n * 0.25
	}
	prev := ctx.Seed
	if len(history) > 0 {
		prev = history[len(history)-1]
	}
	prevEmotion := emotionFromContext(ctx, prev).Basis
	candEmotion := emotionFromContext(ctx, c.track).Basis
	moodSafe := 1 - emotion.Distance(prevEmotion, candEmotion)
	jumpSafe := 1 - emotion.HardJumpRisk(prevEmotion, candEmotion)
	tempoSafe := tempoCompatibility(prev, c.track)
	textureSafe := textureContinuity(prev, c.track)
	safety := clamp01(moodSafe*0.36 + jumpSafe*0.26 + tempoSafe*0.20 + textureSafe*0.18)
	if ctx.Position < 4 {
		safety *= 0.65
	}
	if ctx.Mode == TrajectoryExplore {
		return clamp01(n*0.48 + safety*0.52)
	}
	return clamp01(n*0.35 + safety*0.65)
}

func repetitionPenalty(t library.Track, history []library.Track, recent bool) float64 {
	penalty := 0.0
	if recent {
		penalty += 0.8
	}
	for _, h := range history {
		if h.ID == t.ID {
			penalty += 1.0
		}
		if h.Artist != "" && h.Artist == t.Artist {
			penalty += 0.20
		}
		if h.Album != "" && h.Album == t.Album {
			penalty += 0.12
		}
	}
	return clamp01(penalty)
}

func sessionVolatility(history []library.Track, next library.Track) float64 {
	if len(history) < 2 {
		return 0
	}
	window := append([]library.Track{}, history...)
	window = append(window, next)

	total := 0.0
	steps := 0
	for i := 1; i < len(window); i++ {
		a := window[i-1]
		b := window[i]
		total += math.Abs(a.Energy-b.Energy) * 0.25
		total += math.Abs(a.Aggressive-b.Aggressive) * 0.25
		total += math.Abs(a.Valence-b.Valence) * 0.18
		total += math.Abs(a.Party-b.Party) * 0.14
		total += math.Abs(a.Relaxed-b.Relaxed) * 0.10
		total += math.Abs(a.TimbreBrightness-b.TimbreBrightness) * 0.08
		steps++
	}
	if steps == 0 {
		return 0
	}
	return clamp01(total / float64(steps))
}

func perceptualSessionVolatility(history []library.Track, next library.Track, n FeatureNormalizer) float64 {
	if len(history) < 2 {
		return 0
	}
	window := append([]library.Track{}, history...)
	window = append(window, next)
	total := 0.0
	steps := 0
	for i := 1; i < len(window); i++ {
		a := emotion.Compute(window[i-1], n).Basis
		b := emotion.Compute(window[i], n).Basis
		total += emotion.HardJumpRisk(a, b)
		steps++
	}
	if steps == 0 {
		return 0
	}
	return clamp01(total / float64(steps))
}

func hardJumpRisk(prev, next library.Track, mode RayTrajectoryMode) bool {
	prevMood := calibrateMood(prev)
	nextMood := calibrateMood(next)
	edgeJump := math.Abs(prevMood.Edge - nextMood.Edge)
	pressureJump := math.Abs(prevMood.Aggression - nextMood.Aggression)
	calmJump := math.Abs(prevMood.Calmness - nextMood.Calmness)
	coolJump := math.Abs(prevMood.Brightness - nextMood.Brightness)
	energyJump := math.Abs(prev.Energy-next.Energy) > 0.42
	tempoJump := math.Abs(effectiveTempo(prev)-effectiveTempo(next)) > 38
	if mode == TrajectoryExplore {
		return (edgeJump > 0.22 && pressureJump > 0.18 && tempoJump) || (edgeJump > 0.30 && calmJump > 0.20)
	}
	return edgeJump > 0.20 && (pressureJump > 0.14 || calmJump > 0.14 || coolJump > 0.12 || energyJump || tempoJump)
}

func perceptualHardJumpRisk(ctx rankingContext, prev, next library.Track, mode RayTrajectoryMode) float64 {
	a := emotionFromContext(ctx, prev).Basis
	b := emotionFromContext(ctx, next).Basis
	risk := emotion.HardJumpRisk(a, b)
	rawDist := rawSensoryDistance(prev, next)
	if rawDist > 0.28 {
		risk = math.Max(risk, rawDist*0.72)
	}
	familyPen := familyTransitionPenalty(a.Label, b.Label)
	if familyPen > 0 {
		risk = math.Max(risk, familyPen)
	}
	if !sameEmotionFamily(a.Label, b.Label) && a.Label != "neutral" && b.Label != "neutral" {
		risk = math.Max(risk, 0.32)
	}
	if mode == TrajectoryExplore {
		risk *= 0.86
		if math.Abs(a.Combat-b.Combat) > 0.34 || math.Abs(a.Pressure-b.Pressure) > 0.34 {
			risk = math.Max(risk, 0.48)
		}
	}
	return clamp01(risk)
}

func blendedJumpPenalty(ctx rankingContext, prev, cand library.Track, mode RayTrajectoryMode) float64 {
	legacy := jumpPenalty(prev, cand)
	pRisk := perceptualHardJumpRisk(ctx, prev, cand, mode)
	return clamp01(legacy*0.45 + pRisk*0.55)
}

func positionPhase(position, total int) string {
	if total <= 0 {
		total = rayQueueMax
	}
	p := float64(position) / float64(total)
	switch {
	case p < 0.25:
		return "trust"
	case p < 0.75:
		return "develop"
	default:
		return "resolve"
	}
}

func canPlaceDiscovery(queue []rays.QueueItem, candidate scored, mode RayTrajectoryMode) bool {
	pos := len(queue) + 1
	if pos <= 3 {
		return false
	}

	discoveryCount := 0
	wildcardCount := 0
	recentDiscovery := 0
	for i, item := range queue {
		bucket := item.Insight.Bucket
		strategy := item.Insight.Strategy
		if bucket == "" {
			bucket = item.Bucket
		}
		if strategy == "" {
			strategy = item.Strategy
		}
		if bucket == bucketDiscovery {
			discoveryCount++
		}
		if strategy == "wildcard" {
			wildcardCount++
		}
		if i >= len(queue)-2 && bucket == bucketDiscovery {
			recentDiscovery++
		}
	}

	maxDiscovery := 2
	maxWildcard := 0
	switch mode {
	case TrajectoryExplore:
		maxDiscovery = 5
		maxWildcard = 1
	case TrajectoryWarmUp:
		maxDiscovery = 2
		maxWildcard = 0
	case TrajectoryCoolDown:
		maxDiscovery = 1
		maxWildcard = 0
	case TrajectoryDeepen:
		maxDiscovery = 2
		maxWildcard = 0
	default:
		maxDiscovery = 2
		maxWildcard = 0
	}

	if discoveryCount >= maxDiscovery {
		return false
	}
	if candidate.strategy == "wildcard" && wildcardCount >= maxWildcard {
		return false
	}
	if recentDiscovery >= 1 && mode != TrajectoryExplore {
		return false
	}
	if recentDiscovery >= 2 {
		return false
	}
	return true
}

func effectiveTempo(t library.Track) float64 {
	if t.BPMPerceived > 0 {
		return t.BPMPerceived
	}
	return t.Tempo
}

func transitionLabel(prev, next library.Track) string {
	energy := next.Energy - prev.Energy
	aggr := next.Aggressive - prev.Aggressive
	valence := next.Valence - prev.Valence
	tempo := effectiveTempo(next) - effectiveTempo(prev)

	switch {
	case math.Abs(energy) < 0.10 && math.Abs(aggr) < 0.10 && math.Abs(valence) < 0.10 && math.Abs(tempo) < 10:
		return "stable"
	case energy > 0.16 && aggr < 0.16:
		return "warm_up"
	case energy < -0.16 || aggr < -0.16:
		return "cool_down"
	case next.Dreaminess > prev.Dreaminess+0.12 || next.Emotionality > prev.Emotionality+0.12:
		return "deepen"
	case valence > 0.14:
		return "brighten"
	case valence < -0.14:
		return "darken"
	case aggr > 0.16 || next.Heaviness > prev.Heaviness+0.16:
		return "intensify"
	case next.Softness > prev.Softness+0.14 || next.Relaxed > prev.Relaxed+0.14:
		return "soften"
	default:
		return "shift"
	}
}

func energyDirectionLabel(prev, next library.Track) string {
	d := next.Energy - prev.Energy
	if d > 0.12 {
		return "up"
	}
	if d < -0.12 {
		return "down"
	}
	return "stable"
}

func insightConfidenceWithNormalizer(t library.Track, n FeatureNormalizer) float64 {
	base := insightConfidence(t)
	if len(n.Stats) == 0 {
		return base
	}
	moodNames := []string{"Danceability", "Valence", "Relaxed", "Party", "Aggressive", "Sad", "Heaviness", "Softness", "TimbreBrightness"}
	sum := 0.0
	for _, name := range moodNames {
		sum += n.Reliability(name)
	}
	moodRel := sum / float64(len(moodNames))
	return clamp01(0.35*moodRel + 0.65*base)
}

func insightConfidence(t library.Track) float64 {
	moodConf := moodTrust(t)
	tempoConf := tempoTrust(t)
	genreConf := genreTrust(t)
	analysisConf := 0.25
	if t.AnalyzedLevel >= 2 && strings.EqualFold(strings.TrimSpace(t.AnalysisStatus), "done") {
		analysisConf = 0.85
	}
	if strings.TrimSpace(t.AnalysisError) != "" {
		analysisConf *= 0.35
	}
	embeddingConf := 0.0
	if len(t.Embedding) > 0 {
		embeddingConf = 1.0
	}
	score := clamp01(moodConf*0.30 + tempoConf*0.25 + genreConf*0.15 + analysisConf*0.15 + embeddingConf*0.15)
	if tempoConf <= 0 {
		score = math.Min(score, 0.72)
	}
	if genreConf < 0.25 {
		score = math.Min(score, 0.78)
	}
	if hasSaturatedMoodHeads(t) {
		score = math.Min(score, 0.70)
	}
	return clamp01(score)
}

func insightFallbackLabel(t library.Track) string {
	parts := []string{}
	if effectiveTempo(t) <= 0 || t.TempoConfidence < 0.35 {
		parts = append(parts, "tempo")
	}
	if t.AnalyzedLevel < 2 {
		parts = append(parts, "analysis")
	}
	if len(t.Embedding) == 0 {
		parts = append(parts, "embedding")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "+")
}

func moodTrust(t library.Track) float64 {
	fields := []float64{
		t.Energy, t.Danceability, t.Valence, t.Happy, t.Sad, t.Relaxed,
		t.Party, t.Aggressive, t.Acousticness, t.Electronicness, t.Instrumentalness,
		t.Vocalness, t.Melodicness, t.Softness, t.Heaviness, t.Dreaminess,
		t.Emotionality, t.TimbreBrightness, t.Tonality, t.Approachability, t.Engagement,
	}
	valid := 0
	nonZero := 0
	saturated := 0
	for _, v := range fields {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			continue
		}
		valid++
		if v > 0.001 {
			nonZero++
		}
		if v >= 0.995 || v <= 0.005 {
			saturated++
		}
	}
	if valid == 0 {
		return 0
	}
	coverage := float64(nonZero) / float64(valid)
	saturationPenalty := float64(saturated) / float64(valid)
	return clamp01(coverage * (1 - saturationPenalty*0.55))
}

func hasSaturatedMoodHeads(t library.Track) bool {
	count := 0
	if t.Sad >= 0.95 {
		count++
	}
	if t.Tonality >= 0.995 {
		count++
	}
	if t.Approachability >= 0.995 {
		count++
	}
	if t.Engagement >= 0.995 {
		count++
	}
	return count >= 3
}

func insightWarning(ins rays.QueueInsight, t library.Track) string {
	warnings := []string{}
	if ins.TempoUnknown || tempoTrust(t) <= 0 {
		warnings = append(warnings, "tempo_unknown")
	}
	if genreTrust(t) < 0.35 {
		warnings = append(warnings, "genre_weak")
	}
	if ins.TargetMoodFit > 0 && ins.TargetMoodFit < 0.55 {
		warnings = append(warnings, "target")
	}
	if ins.Novelty >= 0.99 {
		warnings = append(warnings, "nov_saturated")
	}
	if ins.JumpPenalty <= 0.001 && ins.MoodDistance > 0.25 {
		warnings = append(warnings, "jump_zero_suspect")
	}
	if ins.JumpPenalty <= 0.001 && ins.MoodDistance > 0.25 {
		warnings = append(warnings, "jump_zero_suspect")
	}
	if t.Sad >= 0.95 {
		warnings = append(warnings, "sad_saturated")
	}
	if t.Tonality >= 0.995 {
		warnings = append(warnings, "tonal_saturated")
	}
	if t.Approachability >= 0.995 {
		warnings = append(warnings, "approach_saturated")
	}
	if t.Engagement >= 0.995 {
		warnings = append(warnings, "engage_saturated")
	}
	if ins.Confidence > 0.90 && (ins.TempoUnknown || genreTrust(t) < 0.35 || hasSaturatedMoodHeads(t)) {
		warnings = append(warnings, "conf_suspect")
	}
	return strings.Join(warnings, ",")
}

type RayAuditRow struct {
	Position        int               `json:"position"`
	TrackID         string            `json:"trackId"`
	Title           string            `json:"title"`
	Reason          string            `json:"reason"`
	Bucket          string            `json:"bucket"`
	Strategy        string            `json:"strategy"`
	Score           float64           `json:"score"`
	Insight         rays.QueueInsight `json:"insight"`
	EmotionLabel    string            `json:"emotionLabel,omitempty"`
	EmotionFamily   string            `json:"emotionFamily,omitempty"`
	EmotionDistance float64           `json:"emotionDistance,omitempty"`
	HardJumpRisk    float64           `json:"hardJumpRisk,omitempty"`
	BridgeScore     float64           `json:"bridgeScore,omitempty"`
	FamilyPenalty   float64           `json:"familyPenalty,omitempty"`
}

type RayAuditResult struct {
	SeedTrackID string          `json:"seedTrackId"`
	Mode        string          `json:"mode"`
	Rows        []RayAuditRow   `json:"rows"`
	Summary     RayAuditSummary `json:"summary"`
	Warnings    []string        `json:"warnings"`
}

type RayAuditSummary struct {
	TotalTracks    int            `json:"totalTracks"`
	CoreCount      int            `json:"coreCount"`
	BridgeCount    int            `json:"bridgeCount"`
	AdjacentCount  int            `json:"adjacentCount"`
	DiscoveryCount int            `json:"discoveryCount"`
	AvgConfidence  float64        `json:"avgConfidence"`
	AvgNovelty     float64        `json:"avgNovelty"`
	TopStrategies  map[string]int `json:"topStrategies"`
	TopTransitions map[string]int `json:"topTransitions"`
}

type ExtendRayRequest struct {
	Seed             library.Track
	LastTracks       []library.Track
	ExistingTrackIDs []string
	Mode             string
	Count            int
	Library          []library.Track
}

func (s *Service) ExtendRay(req ExtendRayRequest) []rays.QueueItem {
	if req.Count <= 0 {
		req.Count = 6
	}
	existing := map[string]bool{}
	for _, id := range req.ExistingTrackIDs {
		existing[id] = true
	}
	queue := s.BuildRayWithMode(req.Seed, req.Library, "", req.Mode)
	out := make([]rays.QueueItem, 0, req.Count)
	for _, item := range queue {
		if existing[item.TrackID] {
			continue
		}
		out = append(out, item)
		existing[item.TrackID] = true
		if len(out) >= req.Count {
			break
		}
	}
	return out
}

func NewService(events *events.Service, rays *rays.Service) *Service {
	return &Service{events: events, rays: rays}
}

func (s *Service) Recluster(tracks []library.Track) {
	start := time.Now()
	if len(tracks) == 0 {
		recommendLog.D("recluster skipped empty library")
		return
	}
	k := max(3, int(math.Sqrt(float64(len(tracks)))))
	before := make(map[string]int, len(tracks))
	for _, track := range tracks {
		before[track.ID] = track.ClusterID
	}

	normalizer := BuildFeatureNormalizer(tracks)
	points := make([][]float64, len(tracks))
	for i := range tracks {
		if len(tracks[i].Embedding) != modelcontract.DiscogsEmbeddingSize {
			continue
		}
		points[i] = emotionClusterVector(tracks[i], normalizer)
	}

	recommendLog.I("recluster start tracks=%d k=%d mode=emotion", len(tracks), k)
	centroids := initialEmotionClusterCentroids(tracks, k, normalizer)
	if len(centroids) == 0 {
		for i := range tracks {
			tracks[i].ClusterID = 0
			recommendLog.T("recluster fallback track=%s cluster=0 reason=no-semantic-analysis", tracks[i].ID)
		}
		recommendLog.I("recluster done tracks=%d clusters=1 changed=%d mode=fallback-no-semantic-analysis ms=%d", len(tracks), changedClusterCount(tracks, before), time.Since(start).Milliseconds())
		return
	}

	for iter := 0; iter < 6; iter++ {
		recommendLog.D("recluster iter=%d centroids=%d mode=emotion", iter+1, len(centroids))
		sums := make([][]float64, len(centroids))
		counts := make([]int, len(centroids))
		for i := range sums {
			sums[i] = make([]float64, len(centroids[i]))
		}
		for i := range tracks {
			if len(points[i]) == 0 {
				tracks[i].ClusterID = 0
				continue
			}
			best := 0
			bestDistance := math.Inf(1)
			for c := range centroids {
				distance := emotionVectorDistance(points[i], centroids[c])
				if distance < bestDistance {
					best, bestDistance = c, distance
				}
			}
			tracks[i].ClusterID = best
			counts[best]++
			for d := range centroids[best] {
				sums[best][d] += points[i][d]
			}
		}
		for c := range centroids {
			if counts[c] == 0 {
				continue
			}
			for d := range centroids[c] {
				centroids[c][d] = sums[c][d] / float64(counts[c])
			}
			recommendLog.T("centroid updated iter=%d cluster=%d count=%d mode=emotion", iter+1, c, counts[c])
		}
	}
	for _, track := range tracks {
		basis := emotion.Compute(track, normalizer).Basis
		recommendLog.D(
			"recluster assignment track=%s title=%q cluster=%d emotion=%q joy=%.3f melancholy=%.3f serenity=%.3f combat=%.3f",
			track.ID,
			track.Title,
			track.ClusterID,
			basis.Label,
			basis.Joy,
			basis.Melancholy,
			basis.Serenity,
			basis.Combat,
		)
	}
	health := measureClusterHealth(tracks, points, centroids)
	recommendLog.I(
		"recluster health valid=%d clusters=%d sizes=%v meanDistance=%.4f maxDistance=%.4f",
		health.ValidPoints,
		len(centroids),
		health.Sizes,
		health.MeanDistance,
		health.MaxDistance,
	)
	recommendLog.I("recluster done tracks=%d clusters=%d changed=%d mode=emotion ms=%d", len(tracks), len(centroids), changedClusterCount(tracks, before), time.Since(start).Milliseconds())
}

type clusterHealth struct {
	ValidPoints  int
	Sizes        []int
	MeanDistance float64
	MaxDistance  float64
}

func measureClusterHealth(tracks []library.Track, points, centroids [][]float64) clusterHealth {
	health := clusterHealth{Sizes: make([]int, len(centroids))}
	if len(centroids) == 0 {
		return health
	}
	totalDistance := 0.0
	limit := len(tracks)
	if len(points) < limit {
		limit = len(points)
	}
	for i := 0; i < limit; i++ {
		if len(points[i]) == 0 {
			continue
		}
		clusterID := tracks[i].ClusterID
		if clusterID < 0 || clusterID >= len(centroids) {
			continue
		}
		distance := emotionVectorDistance(points[i], centroids[clusterID])
		if math.IsNaN(distance) || math.IsInf(distance, 0) {
			continue
		}
		health.ValidPoints++
		health.Sizes[clusterID]++
		totalDistance += distance
		if distance > health.MaxDistance {
			health.MaxDistance = distance
		}
	}
	if health.ValidPoints > 0 {
		health.MeanDistance = totalDistance / float64(health.ValidPoints)
	}
	return health
}

func changedClusterCount(tracks []library.Track, before map[string]int) int {
	changed := 0
	for _, track := range tracks {
		if before[track.ID] != track.ClusterID {
			changed++
		}
	}
	return changed
}

func initialEmotionClusterCentroids(tracks []library.Track, k int, normalizer FeatureNormalizer) [][]float64 {
	if k <= 0 {
		return nil
	}
	type candidate struct {
		id     string
		vector []float64
	}
	candidates := make([]candidate, 0, len(tracks))
	for _, track := range tracks {
		if len(track.Embedding) != modelcontract.DiscogsEmbeddingSize {
			continue
		}
		id := strings.TrimSpace(track.ID)
		if id == "" {
			id = strings.TrimSpace(track.Path)
		}
		candidates = append(candidates, candidate{
			id:     id,
			vector: emotionClusterVector(track, normalizer),
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].id != candidates[j].id {
			return candidates[i].id < candidates[j].id
		}
		return emotionVectorLexLess(candidates[i].vector, candidates[j].vector)
	})
	if k > len(candidates) {
		k = len(candidates)
	}

	centroids := make([][]float64, 0, k)
	selected := make([]bool, len(candidates))
	selected[0] = true
	centroids = append(centroids, append([]float64{}, candidates[0].vector...))

	// Deterministic farthest-first seeding keeps cluster IDs reproducible for
	// the same library while spreading seeds over subjective emotion space.
	for len(centroids) < k {
		bestIndex := -1
		bestDistance := -1.0
		for i := range candidates {
			if selected[i] {
				continue
			}
			nearestDistance := math.Inf(1)
			for _, centroid := range centroids {
				distance := emotionVectorDistance(candidates[i].vector, centroid)
				if distance < nearestDistance {
					nearestDistance = distance
				}
			}
			if nearestDistance > bestDistance+1e-12 {
				bestDistance = nearestDistance
				bestIndex = i
			}
		}
		if bestIndex < 0 {
			break
		}
		selected[bestIndex] = true
		centroids = append(centroids, append([]float64{}, candidates[bestIndex].vector...))
	}
	return centroids
}

func emotionClusterVector(track library.Track, normalizer FeatureNormalizer) []float64 {
	basis := emotion.Compute(track, normalizer).Basis
	return []float64{
		basis.Motion,
		basis.Roughness,
		basis.Smoothness,
		basis.Pressure,
		basis.Joy,
		basis.Melancholy,
		basis.Serenity,
		basis.Swagger,
		basis.Combat,
		basis.Dreaminess,
	}
}

func emotionVectorDistance(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return math.Inf(1)
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		delta := a[i] - b[i]
		sum += delta * delta
	}
	return math.Sqrt(sum / float64(n))
}

func emotionVectorLexLess(a, b []float64) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] == b[i] {
			continue
		}
		return a[i] < b[i]
	}
	return len(a) < len(b)
}

func (s *Service) BuildRay(seed library.Track, tracks []library.Track, currentRayID string) []rays.QueueItem {
	return s.BuildRayWithMode(
		seed,
		tracks,
		currentRayID,
		"",
	)
}

func (s *Service) BuildRayWithMode(
	seed library.Track,
	tracks []library.Track,
	currentRayID string,
	requestedMode string,
) []rays.QueueItem {
	queue, err := s.BuildRayWithModeContext(
		context.Background(),
		seed,
		tracks,
		currentRayID,
		requestedMode,
	)
	if err != nil {
		return nil
	}
	return queue
}

func (s *Service) BuildRayWithModeContext(
	ctx context.Context,
	seed library.Track,
	tracks []library.Track,
	currentRayID string,
	requestedMode string,
) ([]rays.QueueItem, error) {
	startedAt := time.Now()

	if err := checkRayBuildContext(ctx); err != nil {
		return nil, err
	}

	_ = currentRayID
	normalizer := BuildFeatureNormalizer(tracks)

	if err := checkRayBuildContext(ctx); err != nil {
		return nil, err
	}

	emoCache := newEmotionCache(normalizer)
	seedRaw := seed
	seed = normalizeTrackFeatures(seed, normalizer)
	seedTrust := trackTrust(seedRaw)
	logFeatureHealth(normalizer)
	strategyStats := s.loadStrategyStats()
	feedback := s.loadFeedback()
	recentTrackSet := map[string]bool{}
	for _, id := range s.loadRecentTrackIDs(12) {
		recentTrackSet[id] = true
	}
	delete(recentTrackSet, seed.ID)
	mode := normalizeRayMode(requestedMode, inferRayMode(seedRaw, tracks, feedback, normalizer))
	contentMode := rays.NormalizeContentMode(requestedMode)
	core := make([]scored, 0, len(tracks))
	adjacent := make([]scored, 0, len(tracks))
	bridge := make([]scored, 0, len(tracks)/2)
	favorites := make([]scored, 0, len(tracks)/3)
	discovery := make([]scored, 0, len(tracks))
	wildcard := make([]scored, 0, len(tracks)/4)

	for trackIndex, t := range tracks {
		if trackIndex&7 == 0 {
			if err := checkRayBuildContext(ctx); err != nil {
				return nil, err
			}
		}

		if t.ID == seedRaw.ID {
			continue
		}
		raw := t
		t = normalizeTrackFeatures(t, normalizer)
		trust := trackTrust(raw)

		seedEmotionResult := emoCache.Get(seedRaw)
		candEmotionResult := emoCache.Get(raw)
		emoflowAffinity := emoflowCandidateAffinityFromResults(
			seedRaw,
			raw,
			seedEmotionResult,
			candEmotionResult,
		)

		audioSim := vectorSim(seedRaw.Embedding, raw.Embedding)
		if audioSim == 0 {
			audioSim = 1 - math.Min(1, math.Abs(seedRaw.Tempo-raw.Tempo)/60) - math.Abs(seedRaw.Energy-raw.Energy)*0.35
		}
		if seedTrust.Embedding < 0.5 || trust.Embedding < 0.5 {
			audioSim = clamp01(
				audioSim*0.45 +
					genreAffinity(seedRaw, raw)*0.25 +
					emoflowAffinity*0.30,
			)
		}
		tempoAffinity := tempoCompatibility(seedRaw, raw)
		energyAffinity := 1 - math.Min(1, math.Abs(seedRaw.Energy-raw.Energy)/0.22)
		legacyMoodAffinity := 1 - math.Min(1, math.Abs(seedRaw.Danceability-raw.Danceability)*0.45+math.Abs(seedRaw.Valence-raw.Valence)*0.35)
		seedEmotion := seedEmotionResult.Basis
		candEmotion := candEmotionResult.Basis
		perceptualMoodAffinity := 1 -
			emotion.Distance(seedEmotion, candEmotion)
		moodAffinity := clamp01(legacyMoodAffinity*0.35 + perceptualMoodAffinity*0.65)
		transitionAffinity := smoothTransition(seedRaw, raw)
		genreAffinity := genreAffinity(seedRaw, raw)
		clusterNeighbor := clusterNeighborScore(seedRaw.ClusterID, raw.ClusterID)
		itemFeedback := feedback[t.ID]
		novelty := noveltyScore(raw)
		notRecentlyPlayed := notRecentlyPlayedScore(raw, itemFeedback, recentTrackSet[t.ID])
		skipRisk := skipRiskScore(raw, itemFeedback)
		userAffinity := userAffinityScore(raw, itemFeedback)
		banditBoost := ucbBoost(strategyStats, classifyStrategy(seedRaw, raw))
		recentPenalty := 0.0
		if recentTrackSet[t.ID] {
			recentPenalty = 0.22
		}
		healthPenalty := playbackHealthPenalty(raw)

		if audioSim >= 0.82 && clusterNeighbor >= 0.85 {
			strategy := classifyStrategy(seedRaw, raw)
			reason := "Похожий трек в том же луче"
			score := audioSim*0.56 + genreAffinity*0.14 + moodAffinity*0.14 + energyAffinity*0.06 + tempoAffinity*0.06 + notRecentlyPlayed*0.04 + userAffinity*0.04 - skipRisk*0.20 - recentPenalty - healthPenalty
			core = append(core, scored{track: raw, score: score, reason: reason, strategy: strategy, bucket: bucketCore})
			continue
		}
		if audioSim >= 0.62 || (clusterNeighbor >= 0.65 && moodAffinity >= 0.55) {
			strategy := classifyStrategy(seedRaw, raw)
			reason := "Соседний трек с совместимым настроением"
			score := audioSim*0.38 + moodAffinity*0.20 + transitionAffinity*0.15 + novelty*0.12 + clusterNeighbor*0.10 + banditBoost*0.05 + notRecentlyPlayed*0.04 - skipRisk*0.18 - recentPenalty - healthPenalty
			adjacent = append(adjacent, scored{track: raw, score: score, reason: reason, strategy: strategy, bucket: bucketAdjacent})
		}
		seedCtx := rankingContext{Seed: seedRaw, Mode: mode, QueueLength: rayQueueMax, Normalizer: normalizer, EmotionCache: emoCache}
		bridgeScore := clamp01(bridgeAffinity(seedRaw, raw)*0.28 + perceptualBridgeAffinityWithContext(seedCtx, seedRaw, raw)*0.52 + (1-rawSensoryDistance(seedRaw, raw))*0.20)
		bridgeJump := blendedJumpPenalty(seedCtx, seedRaw, raw, mode)
		if bridgeScore >= 0.66 && transitionAffinity >= 0.58 && bridgeJump <= 0.34 && skipRisk < 0.90 {
			strategy := "mood_bridge"
			reason := "Совместимое настроение для мягкого перехода"
			score := bridgeScore*0.34 +
				emoflowAffinity*0.22 +
				transitionAffinity*0.18 +
				audioSim*0.08 +
				userAffinity*0.08 +
				notRecentlyPlayed*0.06 +
				banditBoost*0.04 -
				skipRisk*0.16 -
				bridgeJump*0.18 -
				recentPenalty*0.4 -
				healthPenalty
			bridge = append(bridge, scored{track: raw, score: score, reason: reason, strategy: strategy, bucket: bucketBridge})
		}
		if userAffinity >= 0.52 && notRecentlyPlayed >= 0.35 && skipRisk < 0.70 {
			strategy := "forgotten_favorite"
			reason := "Знакомый трек, который давно не звучал"
			score := userAffinity*0.34 + notRecentlyPlayed*0.24 + audioSim*0.18 + transitionAffinity*0.12 + moodAffinity*0.08 - recentPenalty*0.5 - healthPenalty
			favorites = append(favorites, scored{track: raw, score: score, reason: reason, strategy: strategy, bucket: bucketAdjacent})
		}
		if audioSim >= 0.45 && novelty >= 0.45 && skipRisk < 0.75 {
			strategy := "discovery_safe"
			reason := "Осторожное открытие рядом по вайбу"
			score := audioSim*0.28 + novelty*0.22 + moodAffinity*0.18 + (1-genreAffinity)*0.12 + banditBoost*0.10 + notRecentlyPlayed*0.10 - skipRisk*0.25 - recentPenalty*0.5 - healthPenalty
			discovery = append(discovery, scored{track: raw, score: score, reason: reason, strategy: strategy, bucket: bucketDiscovery})
		}
		if novelty >= 0.82 && skipRisk < 0.50 && audioSim >= 0.30 {
			strategy := "wildcard"
			reason := "Неожиданный, но потенциально уместный поворот"
			score := novelty*0.34 + banditBoost*0.16 + moodAffinity*0.15 + transitionAffinity*0.14 + audioSim*0.12 + notRecentlyPlayed*0.09 - skipRisk*0.20 - healthPenalty
			wildcard = append(wildcard, scored{track: raw, score: score, reason: reason, strategy: strategy, bucket: bucketDiscovery})
		}
		if mode == TrajectoryDeepen && genreAffinity >= 0.9 && moodAffinity >= 0.70 {
			strategy := "deepen_same_scene"
			reason := "Глубже в том же музыкальном направлении"
			score := audioSim*0.40 + genreAffinity*0.20 + moodAffinity*0.16 + transitionAffinity*0.14 + userAffinity*0.10 - skipRisk*0.16 - healthPenalty
			core = append(core, scored{track: raw, score: score + 0.05, reason: reason, strategy: strategy, bucket: bucketCore})
		}
		if mode == TrajectoryExplore && novelty >= 0.70 && moodAffinity >= 0.60 {
			strategy := "adjacent_discovery"
			reason := "Соседняя сцена без поломки настроения"
			score := novelty*0.24 + moodAffinity*0.22 + transitionAffinity*0.18 + audioSim*0.16 + (1-genreAffinity)*0.10 + banditBoost*0.10 - skipRisk*0.15 - healthPenalty
			discovery = append(discovery, scored{track: raw, score: score, reason: reason, strategy: strategy, bucket: bucketDiscovery})
		}
		if mode == TrajectoryDeepen && novelty < 0.70 && clusterNeighbor >= 0.75 && audioSim >= 0.60 {
			strategy := "comfort_mix"
			reason := "Плотнее внутри знакомого вайба"
			score := audioSim*0.36 + clusterNeighbor*0.18 + moodAffinity*0.16 + userAffinity*0.14 + transitionAffinity*0.10 + genreAffinity*0.06 - skipRisk*0.16 - healthPenalty
			adjacent = append(adjacent, scored{track: raw, score: score, reason: reason, strategy: strategy, bucket: bucketAdjacent})
		}
	}
	sortBucket(core)
	sortBucket(adjacent)
	sortBucket(bridge)
	sortBucket(favorites)
	sortBucket(discovery)
	sortBucket(wildcard)
	if len(discovery) > 5 {
		discovery = discovery[:5]
	}
	if len(wildcard) > 1 {
		wildcard = wildcard[:1]
	}
	seedInsight := rays.QueueInsight{
		Mode:            string(mode),
		Bucket:          "seed",
		Strategy:        "seed",
		Score:           1,
		Transition:      "seed",
		EnergyDirection: "stable",
		Confidence:      insightConfidence(seed),
		Fallback:        insightFallbackLabel(seed),
	}
	seedInsight.Warning = insightWarning(seedInsight, seedRaw)
	queue := []rays.QueueItem{{TrackID: seedRaw.ID, Title: seedRaw.Title, Subtitle: "текущий трек", DurationLabel: seedRaw.DurationLabel, IsCurrent: true, Reason: "старт луча", Bucket: "seed", Strategy: "seed", Score: 1, OriginalPosition: 0, Insight: seedInsight, Track: seedRaw}}
	used := map[string]bool{seed.ID: true}
	lookup := trackLookup(tracks)
	curve := buildEnergyCurve(seed, mode, rayQueueMax)

	allPools := mergeCandidatePools(
		rayQueueMax,
		core,
		adjacent,
		bridge,
		favorites,
		discovery,
		wildcard,
	)

	if err := checkRayBuildContext(ctx); err != nil {
		return nil, err
	}

	queue, err := buildQueueArcContext(
		ctx,
		queue,
		used,
		seed,
		lookup,
		recentTrackSet,
		allPools,
		rayQueueMax,
		strategyStats,
		feedback,
		mode,
		contentMode,
		curve,
		normalizer,
		emoCache,
	)
	if err != nil {
		return nil, err
	}

	logTop(seed, "core", core, 5)
	logTop(seed, "adjacent", adjacent, 4)
	logTop(seed, "bridge", bridge, 4)
	logTop(seed, "discovery", discovery, 3)
	recommendLog.I(
		"ray built seed=%s queue=%d candidates=%d tracks=%d core=%d adjacent=%d bridge=%d discovery=%d currentRay=%s ms=%d",
		seedRaw.ID,
		len(queue),
		len(allPools),
		len(tracks),
		len(core),
		len(adjacent),
		len(bridge),
		len(discovery),
		currentRayID,
		time.Since(startedAt).Milliseconds(),
	)
	logRayHealth(queue)
	return queue, nil
}

func (s *Service) AuditRay(seed library.Track, tracks []library.Track, requestedMode string, n int) RayAuditResult {
	if n <= 0 {
		n = 10
	}
	queue := s.BuildRayWithMode(seed, tracks, "", requestedMode)
	if len(queue) > n {
		queue = queue[:n]
	}
	mode := ""
	if len(queue) > 1 {
		mode = queue[1].Insight.Mode
	}
	rows := make([]RayAuditRow, 0, len(queue))
	coreCount := 0
	bridgeCount := 0
	adjacentCount := 0
	discoveryCount := 0
	totalConfidence := 0.0
	totalNovelty := 0.0
	topStrategies := map[string]int{}
	topTransitions := map[string]int{}
	for i, item := range queue {
		ef := emotionFamily(item.Insight.Emotion.Label)
		var prevFamily string
		if i > 0 {
			prevFamily = emotionFamily(queue[i-1].Insight.Emotion.Label)
		}
		fp := 0.0
		if prevFamily != "" && prevFamily != ef {
			fp = familyTransitionPenalty(queue[i-1].Insight.Emotion.Label, item.Insight.Emotion.Label)
		}
		rows = append(rows, RayAuditRow{Position: i + 1, TrackID: item.TrackID, Title: item.Title, Reason: item.Reason, Bucket: item.Bucket, Strategy: item.Strategy, Score: item.Score, Insight: item.Insight, EmotionLabel: item.Insight.Emotion.Label, EmotionFamily: ef, EmotionDistance: item.Insight.Emotion.Distance, HardJumpRisk: item.Insight.Emotion.HardJump, BridgeScore: item.Insight.Emotion.BridgeScore, FamilyPenalty: fp})
		switch item.Bucket {
		case bucketCore:
			coreCount++
		case bucketBridge:
			bridgeCount++
		case bucketAdjacent:
			adjacentCount++
		case bucketDiscovery:
			discoveryCount++
		}
		totalConfidence += item.Insight.Confidence
		totalNovelty += item.Insight.Novelty
		if item.Strategy != "" {
			topStrategies[item.Strategy]++
		}
		if item.Insight.Transition != "" {
			topTransitions[item.Insight.Transition]++
		}
	}
	summary := RayAuditSummary{
		TotalTracks:    len(rows),
		CoreCount:      coreCount,
		BridgeCount:    bridgeCount,
		AdjacentCount:  adjacentCount,
		DiscoveryCount: discoveryCount,
		AvgConfidence:  0,
		AvgNovelty:     0,
		TopStrategies:  topStrategies,
		TopTransitions: topTransitions,
	}
	if len(rows) > 0 {
		summary.AvgConfidence = totalConfidence / float64(len(rows))
		summary.AvgNovelty = totalNovelty / float64(len(rows))
	}
	warnings := auditWarnings(rows, summary)
	return RayAuditResult{SeedTrackID: seed.ID, Mode: mode, Rows: rows, Summary: summary, Warnings: warnings}
}

func auditWarnings(rows []RayAuditRow, summary RayAuditSummary) []string {
	warnings := []string{}
	if summary.AvgConfidence < 0.5 {
		warnings = append(warnings, "low average confidence")
	}
	if summary.DiscoveryCount > 0 && summary.CoreCount == 0 {
		warnings = append(warnings, "discovery without core")
	}
	if summary.BridgeCount == 0 && len(rows) > 5 {
		warnings = append(warnings, "no bridge tracks")
	}
	for _, row := range rows {
		if row.HardJumpRisk > 0.55 {
			warnings = append(warnings, fmt.Sprintf("position %d: perceptual hard jump %.2f", row.Position, row.HardJumpRisk))
		}
		if row.Bucket == bucketDiscovery && row.HardJumpRisk > 0.42 {
			warnings = append(warnings, fmt.Sprintf("position %d: risky discovery %.2f", row.Position, row.HardJumpRisk))
		}
		if row.Insight.JumpPenalty > 0.5 {
			warnings = append(warnings, "hard jump detected")
			break
		}
	}
	return warnings
}

func logRayHealth(queue []rays.QueueItem) {
	if len(queue) == 0 {
		return
	}
	tempoUnknown := 0
	novSat := 0
	jumpZero := 0
	discovery := 0
	wildcard := 0
	sadSat := 0
	tonalSat := 0
	for _, item := range queue {
		if item.Insight.TempoUnknown || tempoTrust(item.Track) <= 0 {
			tempoUnknown++
		}
		if item.Insight.Novelty >= 0.99 {
			novSat++
		}
		if item.Insight.JumpPenalty <= 0.001 {
			jumpZero++
		}
		if item.Insight.Bucket == bucketDiscovery {
			discovery++
		}
		if item.Insight.Strategy == "wildcard" {
			wildcard++
		}
		if item.Track.Sad >= 0.95 {
			sadSat++
		}
		if item.Track.Tonality >= 0.99 {
			tonalSat++
		}
	}
	recommendLog.I("ray health queue=%d tempoUnknown=%d novSat=%d jumpZero=%d discovery=%d wildcard=%d sadSat=%d tonalSat=%d", len(queue), tempoUnknown, novSat, jumpZero, discovery, wildcard, sadSat, tonalSat)
}

func summarizeAuditRows(rows []RayAuditRow) RayAuditSummary {
	coreCount := 0
	bridgeCount := 0
	adjacentCount := 0
	discoveryCount := 0
	totalConfidence := 0.0
	totalNovelty := 0.0
	topStrategies := map[string]int{}
	topTransitions := map[string]int{}
	for _, item := range rows {
		switch item.Bucket {
		case bucketCore:
			coreCount++
		case bucketBridge:
			bridgeCount++
		case bucketAdjacent:
			adjacentCount++
		case bucketDiscovery:
			discoveryCount++
		}
		totalConfidence += item.Insight.Confidence
		totalNovelty += item.Insight.Novelty
		if item.Strategy != "" {
			topStrategies[item.Strategy]++
		}
		if item.Insight.Transition != "" {
			topTransitions[item.Insight.Transition]++
		}
	}
	summary := RayAuditSummary{
		TotalTracks:    len(rows),
		CoreCount:      coreCount,
		BridgeCount:    bridgeCount,
		AdjacentCount:  adjacentCount,
		DiscoveryCount: discoveryCount,
		AvgConfidence:  0,
		AvgNovelty:     0,
		TopStrategies:  topStrategies,
		TopTransitions: topTransitions,
	}
	if len(rows) > 0 {
		summary.AvgConfidence = totalConfidence / float64(len(rows))
		summary.AvgNovelty = totalNovelty / float64(len(rows))
	}
	return summary
}

func (s *Service) loadFeedback() map[string]feedbackItem {
	if s == nil || s.events == nil {
		return nil
	}
	raw := s.events.Feedback()
	out := map[string]feedbackItem{}
	for key, value := range raw {
		out[key] = feedbackItem{lastPlayedAt: value.LastPlayedAt, lastSkippedAt: value.LastSkippedAt, avgCompletion: value.AvgCompletion, affinity: value.Affinity, playEvents: value.PlayEvents, skipEvents: value.SkipEvents, completeEvents: value.CompleteEvents, lastEventType: value.LastEventType}
	}
	return out
}

func (s *Service) loadRecentTrackIDs(limit int) []string {
	if s == nil || s.events == nil {
		return nil
	}
	return s.events.RecentTrackIDs(limit)
}

func (s *Service) loadStrategyStats() map[string]strategyStat {
	if s == nil || s.events == nil {
		return nil
	}
	raw := s.events.StrategyStats()
	out := map[string]strategyStat{}
	for key, value := range raw {
		out[key] = strategyStat{plays: value.Plays, reward: value.Reward}
	}
	return out
}

type strategyStat struct {
	plays  int
	reward float64
}

func ucbBoost(stats map[string]strategyStat, strategy string) float64 {
	if len(stats) == 0 {
		return 0.03
	}
	total := 1
	st := stats[strategy]
	for _, v := range stats {
		total += v.plays
	}
	if st.plays == 0 {
		return 0.12
	}
	mean := st.reward / float64(st.plays)
	return mean*0.05 + math.Sqrt(2*math.Log(float64(total))/float64(st.plays))*0.04
}

func smoothTransition(seed, candidate library.Track) float64 {
	tempoPenalty := math.Min(1, math.Abs(seed.Tempo-candidate.Tempo)/24)
	energyPenalty := math.Min(1, math.Abs(seed.Energy-candidate.Energy)/0.22)
	loudPenalty := math.Min(1, math.Abs(seed.Loudness-candidate.Loudness)/8)
	centroidPenalty := math.Min(1, math.Abs(seed.SpectralCentroid-candidate.SpectralCentroid)/2400)
	moodPenalty := 1 - moodSmoothness(seed, candidate)
	jumpPenalty := jumpPenalty(seed, candidate)
	return clamp01(1 - (tempoPenalty*0.27 + energyPenalty*0.23 + loudPenalty*0.15 + centroidPenalty*0.10 + moodPenalty*0.15 + jumpPenalty*0.10))
}

func bridgeAffinity(seed, cand library.Track) float64 {
	tempo := tempoCompatibility(seed, cand)
	energyStep := 1 - math.Min(1, math.Abs(seed.Energy-cand.Energy)/0.28)
	aggrStep := 1 - math.Min(1, math.Abs(seed.Aggressive-cand.Aggressive)/0.35)
	texture := 1 - math.Min(1,
		math.Abs(seed.Acousticness-cand.Acousticness)*0.5+
			math.Abs(seed.Electronicness-cand.Electronicness)*0.5,
	)
	vocal := 1 - math.Min(1, math.Abs(seed.Vocalness-cand.Vocalness)/0.45)
	approach := 1 - math.Min(1, math.Abs(seed.Approachability-cand.Approachability)/0.45)
	engagement := 1 - math.Min(1, math.Abs(seed.Engagement-cand.Engagement)/0.45)

	return clamp01(
		tempo*0.20 +
			energyStep*0.20 +
			aggrStep*0.18 +
			texture*0.14 +
			vocal*0.10 +
			approach*0.09 +
			engagement*0.09,
	)
}

func perceptualBridgeAffinity(prev, cand library.Track, n FeatureNormalizer) float64 {
	a := emotion.Compute(prev, n).Basis
	b := emotion.Compute(cand, n).Basis
	tempo := tempoCompatibility(prev, cand)
	texture := textureContinuity(prev, cand)
	vocal := vocalContinuity(prev, cand)
	return clamp01(emotion.BridgeScore(a, b)*0.48 + tempo*0.18 + texture*0.14 + vocal*0.08 + (1-emotion.HardJumpRisk(a, b))*0.12)
}

func perceptualBridgeAffinityWithContext(ctx rankingContext, prev, cand library.Track) float64 {
	a := emotionFromContext(ctx, prev).Basis
	b := emotionFromContext(ctx, cand).Basis
	tempo := tempoCompatibility(prev, cand)
	texture := textureContinuity(prev, cand)
	vocal := vocalContinuity(prev, cand)
	rawSafe := 1 - rawSensoryDistance(prev, cand)
	return clamp01(emotion.BridgeScore(a, b)*0.38 + rawSafe*0.24 + tempo*0.14 + texture*0.12 + vocal*0.06 + (1-emotion.HardJumpRisk(a, b))*0.06)
}

func vectorSim(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		av, bv := float64(a[i]), float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func sortBucket(items []scored) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})
}

func mergeCandidatePools(
	queueSize int,
	pools ...[]scored,
) []scored {
	byTrackID := make(map[string]scored)

	for _, pool := range pools {
		for _, item := range pool {
			trackID := strings.TrimSpace(item.track.ID)
			if trackID == "" {
				continue
			}

			current, exists := byTrackID[trackID]
			if !exists ||
				candidateMergeScore(item) >
					candidateMergeScore(current) {
				byTrackID[trackID] = item
			}
		}
	}

	merged := make([]scored, 0, len(byTrackID))
	for _, item := range byTrackID {
		merged = append(merged, item)
	}

	sort.SliceStable(merged, func(i, j int) bool {
		left := candidateMergeScore(merged[i])
		right := candidateMergeScore(merged[j])
		if left == right {
			return merged[i].track.ID < merged[j].track.ID
		}
		return left > right
	})

	limit := candidatePoolLimit(queueSize)
	if len(merged) > limit {
		merged = merged[:limit]
	}

	return merged
}

func candidateMergeScore(item scored) float64 {
	bonus := 0.0

	switch item.bucket {
	case bucketBridge:
		bonus = 0.035
	case bucketCore:
		bonus = 0.025
	case bucketAdjacent:
		bonus = 0.015
	case bucketDiscovery:
		bonus = 0.005
	}

	return item.score + bonus
}

func candidatePoolLimit(queueSize int) int {
	if queueSize <= 0 {
		queueSize = 24
	}

	limit := queueSize * 6
	if limit < 96 {
		limit = 96
	}
	if limit > 192 {
		limit = 192
	}
	return limit
}

func genreAffinity(seed, candidate library.Track) float64 {
	seedTrust := genreTrust(seed)
	candidateTrust := genreTrust(candidate)
	if seedTrust < 0.35 || candidateTrust < 0.35 {
		return metadataGenreAffinity(seed, candidate)
	}
	seedGenre := lowerTrim(seed.GenrePrimary)
	candidateGenre := lowerTrim(candidate.GenrePrimary)
	if seedGenre != "" && seedGenre == candidateGenre {
		return 0.75 + 0.25*math.Min(seedTrust, candidateTrust)
	}
	if seedGenre == "" || candidateGenre == "" {
		return 0.45
	}
	if strings.Contains(strings.ToLower(seed.GenreLabel), candidateGenre) || strings.Contains(strings.ToLower(candidate.GenreLabel), seedGenre) {
		return 0.62
	}
	return 0.25
}

func genreTrust(t library.Track) float64 {
	label := strings.ToLower(strings.TrimSpace(t.GenrePrimary + " " + t.GenreDetail + " " + t.GenreLabel))
	if label == "" || strings.Contains(label, "unknown") {
		return 0
	}
	if len(t.GenreTags) == 0 {
		if strings.TrimSpace(t.Genre) != "" {
			return 0.35
		}
		return 0
	}
	best := 0.0
	second := 0.0
	for _, tag := range t.GenreTags {
		score := tag.Score
		if score > best {
			second = best
			best = score
		} else if score > second {
			second = score
		}
	}
	if best <= 0 {
		return 0.20
	}
	margin := best - second
	return clamp01(best*1.20 + margin*1.80)
}

func metadataGenreAffinity(a, b library.Track) float64 {
	ga := strings.ToLower(strings.TrimSpace(a.Genre))
	gb := strings.ToLower(strings.TrimSpace(b.Genre))
	if ga == "" || gb == "" || ga == "unknown" || gb == "unknown" {
		return 0.45
	}
	if ga == gb {
		return 0.75
	}
	for _, part := range strings.Split(ga, ",") {
		part = strings.TrimSpace(part)
		if part != "" && strings.Contains(gb, part) {
			return 0.65
		}
	}
	for _, part := range strings.Split(gb, ",") {
		part = strings.TrimSpace(part)
		if part != "" && strings.Contains(ga, part) {
			return 0.65
		}
	}
	return 0.35
}

func clusterNeighborScore(seedCluster, candidateCluster int) float64 {
	diff := math.Abs(float64(seedCluster - candidateCluster))
	if diff == 0 {
		return 1
	}
	if diff == 1 {
		return 0.78
	}
	if diff == 2 {
		return 0.52
	}
	return 0.2
}

func noveltyScore(t library.Track) float64 {
	plays := float64(t.PlayCount)
	skips := float64(t.SkipCount)
	completes := float64(t.CompleteCount)
	base := 0.72
	if plays > 0 {
		base = 1.0 / math.Sqrt(plays+1)
	}
	skipPenalty := math.Min(0.35, skips*0.10)
	completePenalty := math.Min(0.20, completes*0.04)
	return clamp01(base - skipPenalty - completePenalty)
}

func controlledNoveltyScore(seed, prev, cand library.Track) float64 {
	raw := noveltyScore(cand)
	genreNew := 1 - genreAffinity(seed, cand)
	transitionSafe := 1 - transitionJumpPenalty(prev, cand)
	tempoSafe := tempoCompatibility(prev, cand)
	textureSafe := textureContinuity(prev, cand)
	sameArtistPenalty := 0.0
	if strings.TrimSpace(seed.Artist) != "" && seed.Artist == cand.Artist {
		sameArtistPenalty = 0.25
	}
	unknownPenalty := 0.0
	if tempoTrust(cand) <= 0 {
		unknownPenalty += 0.08
	}
	if genreTrust(cand) < 0.35 {
		unknownPenalty += 0.08
	}
	return clamp01(raw*0.36 + genreNew*0.16 + transitionSafe*0.24 + tempoSafe*0.12 + textureSafe*0.12 - sameArtistPenalty - unknownPenalty)
}

func familiarityScore(t library.Track) float64 {
	plays := float64(t.PlayCount)
	completes := float64(t.CompleteCount)
	return math.Min(1, (plays*0.25+completes*0.4)/5)
}

func notRecentlyPlayedScore(t library.Track, fb feedbackItem, recent bool) float64 {
	base := 1 - familiarityScore(t)
	if recent {
		base *= 0.25
	}
	if fb.avgCompletion > 0 {
		base = math.Max(0, math.Min(1, base+(0.5-fb.avgCompletion)*0.2))
	}
	return math.Max(0, math.Min(1, base))
}

func skipRiskScore(t library.Track, fb feedbackItem) float64 {
	plays := math.Max(1, float64(t.PlayCount)+float64(fb.playEvents)*0.5)
	penalty := float64(t.SkipCount) + float64(fb.skipEvents)*0.8 + math.Max(0, float64(t.SkipCount-t.CompleteCount))*0.5
	if fb.lastSkippedAt > fb.lastPlayedAt && fb.lastSkippedAt > 0 {
		penalty += 0.8
	}
	if fb.lastEventType == "early_skip" {
		penalty += 0.5
	}
	return math.Min(1.2, penalty/plays)
}

func userAffinityScore(t library.Track, fb feedbackItem) float64 {
	plays := float64(t.PlayCount)
	completes := float64(t.CompleteCount)
	skips := float64(t.SkipCount)
	base := (completes*1.2 + plays*0.2 - skips*0.6) / 5
	base += fb.affinity*0.7 + fb.avgCompletion*0.25
	if fb.completeEvents > fb.skipEvents {
		base += 0.08
	}
	return math.Max(0, math.Min(1, base))
}

func classifyStrategy(seed, t library.Track) string {
	if t.Artist == seed.Artist && t.Artist != "" && t.Artist != "Unknown Artist" {
		return "same_artist"
	}
	if t.ClusterID == seed.ClusterID {
		return "same_cluster"
	}
	if math.Abs(seed.Tempo-t.Tempo) < 8 && math.Abs(seed.Energy-t.Energy) < 0.12 {
		return "smooth_transition"
	}
	return "neighbor_cluster"
}

func bucketTargets(max int) (core, adjacent, discovery int) {
	usable := max - 1
	if usable <= 0 {
		return 0, 0, 0
	}
	core = int(math.Round(float64(usable) * 0.60))
	adjacent = int(math.Round(float64(usable) * 0.25))
	discovery = usable - core - adjacent
	if discovery < 1 && usable >= 3 {
		discovery = 1
		if core >= adjacent {
			core--
		} else {
			adjacent--
		}
	}
	if core < 0 {
		core = 0
	}
	if adjacent < 0 {
		adjacent = 0
	}
	if discovery < 0 {
		discovery = 0
	}
	return core, adjacent, discovery
}

func emoflowCandidateAffinity(seed, candidate library.Track) float64 {
	return emoflowCandidateAffinityWithNormalizer(
		seed,
		candidate,
		FeatureNormalizer{},
	)
}

func emoflowCandidateAffinityFromResults(
	seed library.Track,
	candidate library.Track,
	seedEmotion emotion.Result,
	candidateEmotion emotion.Result,
) float64 {
	a := seedEmotion.Basis
	b := candidateEmotion.Basis

	return clamp01(
		(1-emotion.Distance(a, b))*0.46 +
			(1-emotion.HardJumpRisk(a, b))*0.22 +
			emotion.BridgeScore(a, b)*0.18 +
			tempoCompatibility(seed, candidate)*0.08 +
			textureContinuity(seed, candidate)*0.06,
	)
}

func emoflowCandidateAffinityWithNormalizer(
	seed,
	candidate library.Track,
	n FeatureNormalizer,
) float64 {
	return emoflowCandidateAffinityFromResults(
		seed,
		candidate,
		emotion.Compute(seed, n),
		emotion.Compute(candidate, n),
	)
}

func buildQueueArc(
	queue []rays.QueueItem,
	used map[string]bool,
	seed library.Track,
	lookup map[string]library.Track,
	recentTrackSet map[string]bool,
	items []scored,
	maxSize int,
	strategyStats map[string]strategyStat,
	feedback map[string]feedbackItem,
	mode RayTrajectoryMode,
	contentMode rays.ContentMode,
	curve []float64,
	normalizer FeatureNormalizer,
	emoCache *emotionCache,
) []rays.QueueItem {
	result, _ := buildQueueArcContext(
		context.Background(),
		queue,
		used,
		seed,
		lookup,
		recentTrackSet,
		items,
		maxSize,
		strategyStats,
		feedback,
		mode,
		contentMode,
		curve,
		normalizer,
		emoCache,
	)
	return result
}

func buildQueueArcContext(
	ctx context.Context,
	queue []rays.QueueItem,
	used map[string]bool,
	seed library.Track,
	lookup map[string]library.Track,
	recentTrackSet map[string]bool,
	items []scored,
	maxSize int,
	strategyStats map[string]strategyStat,
	feedback map[string]feedbackItem,
	mode RayTrajectoryMode,
	contentMode rays.ContentMode,
	curve []float64,
	normalizer FeatureNormalizer,
	emoCache *emotionCache,
) ([]rays.QueueItem, error) {
	for len(queue) < maxSize {
		if err := checkRayBuildContext(ctx); err != nil {
			return nil, err
		}

		idx, chosen, ok, err := chooseNextCandidateContext(
			ctx,
			items,
			queue,
			used,
			seed,
			lookup,
			recentTrackSet,
			strategyStats,
			feedback,
			mode,
			contentMode,
			curve,
			normalizer,
			emoCache,
		)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		subtitle := subtitleForBucket(chosen.bucket)
		chosen.trackInsight.Bucket = chosen.bucket
		chosen.trackInsight.Strategy = chosen.strategy
		chosen.trackInsight.Score = chosen.score
		if chosen.trackInsight.Warning == "" {
			chosen.trackInsight.Warning = insightWarning(chosen.trackInsight, chosen.track)
		}
		queue = append(queue, rays.QueueItem{TrackID: chosen.track.ID, Title: chosen.track.Title, Subtitle: subtitle, DurationLabel: chosen.track.DurationLabel, Reason: chosen.reason, Insight: chosen.trackInsight, Track: chosen.track, Bucket: chosen.bucket, Strategy: chosen.strategy, Score: chosen.score, OriginalPosition: len(queue)})
		used[chosen.track.ID] = true
		items[idx].score = -9999
	}
	return queue, nil
}

func subtitleForBucket(bucket string) string {
	switch bucket {
	case bucketBridge:
		return "мост"
	case bucketAdjacent:
		return "рядом"
	case bucketDiscovery:
		return "открытие"
	default:
		return "далее"
	}
}

func appendBucket(queue []rays.QueueItem, items []scored, used map[string]bool, target int, seed library.Track, lookup map[string]library.Track, recentTrackSet map[string]bool, strategyStats map[string]strategyStat, feedback map[string]feedbackItem, mode RayTrajectoryMode, contentMode rays.ContentMode, curve []float64, normalizer FeatureNormalizer, emoCache *emotionCache, subtitle string) []rays.QueueItem {
	for target > 0 {
		idx, chosen, ok := chooseNextCandidate(items, queue, used, seed, lookup, recentTrackSet, strategyStats, feedback, mode, contentMode, curve, normalizer, emoCache)
		if !ok {
			break
		}
		chosen.trackInsight.Bucket = chosen.bucket
		chosen.trackInsight.Strategy = chosen.strategy
		chosen.trackInsight.Score = chosen.score
		if chosen.trackInsight.Warning == "" {
			chosen.trackInsight.Warning = insightWarning(chosen.trackInsight, chosen.track)
		}
		queue = append(queue, rays.QueueItem{TrackID: chosen.track.ID, Title: chosen.track.Title, Subtitle: subtitle, DurationLabel: chosen.track.DurationLabel, Reason: chosen.reason, Insight: chosen.trackInsight, Track: chosen.track, Bucket: chosen.bucket, Strategy: chosen.strategy, Score: chosen.score, OriginalPosition: len(queue)})
		used[chosen.track.ID] = true
		items[idx].score = -9999
		target--
	}
	return queue
}

func fillRemaining(queue []rays.QueueItem, used map[string]bool, seed library.Track, lookup map[string]library.Track, recentTrackSet map[string]bool, items []scored, maxSize int, strategyStats map[string]strategyStat, feedback map[string]feedbackItem, mode RayTrajectoryMode, contentMode rays.ContentMode, curve []float64, normalizer FeatureNormalizer, emoCache *emotionCache) []rays.QueueItem {
	for len(queue) < maxSize {
		idx, chosen, ok := chooseNextCandidate(items, queue, used, seed, lookup, recentTrackSet, strategyStats, feedback, mode, contentMode, curve, normalizer, emoCache)
		if !ok {
			break
		}
		subtitle := "далее"
		if chosen.bucket == bucketAdjacent {
			subtitle = "рядом"
		}
		if chosen.bucket == bucketDiscovery {
			subtitle = "открытие"
		}
		chosen.trackInsight.Bucket = chosen.bucket
		chosen.trackInsight.Strategy = chosen.strategy
		chosen.trackInsight.Score = chosen.score
		if chosen.trackInsight.Warning == "" {
			chosen.trackInsight.Warning = insightWarning(chosen.trackInsight, chosen.track)
		}
		queue = append(queue, rays.QueueItem{TrackID: chosen.track.ID, Title: chosen.track.Title, Subtitle: subtitle, DurationLabel: chosen.track.DurationLabel, Reason: chosen.reason, Insight: chosen.trackInsight, Track: chosen.track, Bucket: chosen.bucket, Strategy: chosen.strategy, Score: chosen.score, OriginalPosition: len(queue)})
		used[chosen.track.ID] = true
		items[idx].score = -9999
	}
	return queue
}

func playbackHealthPenalty(t library.Track) float64 {
	if t.PlaybackErrorCount >= 3 {
		return 2.0
	}
	if t.PlaybackErrorCount <= 0 {
		return 0
	}
	penalty := 0.10 * float64(t.PlaybackErrorCount)
	if t.LastPlaybackErrorAt > 0 && time.Now().Unix()-t.LastPlaybackErrorAt < 86400 {
		penalty += 0.20 * float64(t.PlaybackErrorCount)
	}
	return penalty
}

func canUseCandidate(item scored, queue []rays.QueueItem, used map[string]bool, seed library.Track, recentTrackSet map[string]bool) bool {
	if used[item.track.ID] {
		return false
	}
	if item.track.SourceType == "yt_dlp" {
		if item.track.DownloadStatus != "ready" ||
			strings.TrimSpace(item.track.Path) == "" ||
			item.track.AnalysisStatus != "done" {
			return false
		}
	}
	if item.track.PlaybackErrorCount >= 3 {
		return false
	}
	if len(queue) == 0 {
		return true
	}
	if recentTrackSet[item.track.ID] {
		return false
	}
	if item.track.Artist == seed.Artist && len(queue) < 4 {
		return false
	}
	recentArtist := 0
	for i := max(0, len(queue)-2); i < len(queue); i++ {
		if queue[i].Title == item.track.Title {
			return false
		}
		if queue[i].Strategy == "same_artist" && item.strategy == "same_artist" {
			recentArtist++
		}
	}
	if recentArtist >= 1 {
		return false
	}
	if len(queue) > 1 {
		last := queue[len(queue)-1]
		if last.Bucket == item.bucket && item.bucket == bucketDiscovery {
			return false
		}
		if last.Bucket != bucketDiscovery && item.bucket != bucketDiscovery {
			if math.Abs(seed.Tempo-item.track.Tempo) > 26 || math.Abs(seed.Energy-item.track.Energy) > 0.28 {
				return false
			}
		}
	}
	return true
}

func chooseNextCandidate(
	items []scored,
	queue []rays.QueueItem,
	used map[string]bool,
	seed library.Track,
	lookup map[string]library.Track,
	recentTrackSet map[string]bool,
	strategyStats map[string]strategyStat,
	feedback map[string]feedbackItem,
	mode RayTrajectoryMode,
	contentMode rays.ContentMode,
	curve []float64,
	normalizer FeatureNormalizer,
	emoCache *emotionCache,
) (int, scored, bool) {
	index, item, ok, _ := chooseNextCandidateContext(
		context.Background(),
		items,
		queue,
		used,
		seed,
		lookup,
		recentTrackSet,
		strategyStats,
		feedback,
		mode,
		contentMode,
		curve,
		normalizer,
		emoCache,
	)
	return index, item, ok
}

func chooseNextCandidateContext(
	ctx context.Context,
	items []scored,
	queue []rays.QueueItem,
	used map[string]bool,
	seed library.Track,
	lookup map[string]library.Track,
	recentTrackSet map[string]bool,
	strategyStats map[string]strategyStat,
	feedback map[string]feedbackItem,
	mode RayTrajectoryMode,
	contentMode rays.ContentMode,
	curve []float64,
	normalizer FeatureNormalizer,
	emoCache *emotionCache,
) (int, scored, bool, error) {
	prev := seed
	if len(queue) > 0 {
		if track, ok := lookup[queue[len(queue)-1].TrackID]; ok {
			prev = track
		}
	}
	history := queueTracks(queue, lookup)
	rankCtx := buildRankingContext(seed, history, mode, curve, len(queue), rayQueueMax)
	rankCtx.Normalizer = normalizer
	rankCtx.EmotionCache = emoCache
	rankCtx = applyContentMode(rankCtx, contentMode)
	quota := quotaFromQueue(queue, rayQueueMax)
	bestIdx := -1
	bestScore := -9999.0
	best := scored{}
	phase := positionPhase(len(queue)+1, rayQueueMax)
	for i, item := range items {
		if i&7 == 0 {
			if err := checkRayBuildContext(ctx); err != nil {
				return -1, scored{}, false, err
			}
		}

		if item.score <= -9998 {
			continue
		}
		if !canUseCandidate(item, queue, used, seed, recentTrackSet) {
			continue
		}
		pRisk := perceptualHardJumpRisk(rankCtx, prev, item.track, mode)
		if item.bucket == bucketBridge {
			pRisk *= 0.72
		}
		if item.bucket == bucketDiscovery {
			if pRisk > 0.46 && mode != TrajectoryExplore {
				continue
			}
			if pRisk > 0.56 {
				continue
			}
		}
		if item.bucket == bucketDiscovery && !canPlaceDiscovery(queue, item, mode) {
			continue
		}
		total := scoreNextCandidate(seed, prev, history, item, rankCtx, quota, feedback, strategyStats, recentTrackSet)
		modePenalty := 0.0
		if mode == TrajectoryWarmUp {
			energyDelta := item.track.Energy - prev.Energy
			if energyDelta < -0.08 {
				modePenalty += 0.18
			}
			if item.track.Relaxed > prev.Relaxed+0.18 && item.track.Party < prev.Party {
				modePenalty += 0.10
			}
		}
		if mode == TrajectoryCoolDown {
			energyDelta := item.track.Energy - prev.Energy
			if energyDelta > 0.10 {
				modePenalty += 0.18
			}
			if item.track.Aggressive > prev.Aggressive+0.12 {
				modePenalty += 0.15
			}
		}
		phasePenalty := 0.0
		phaseBoost := 0.0
		switch phase {
		case "trust":
			if item.bucket == bucketDiscovery {
				phasePenalty += 0.30
			}
			if item.strategy == "wildcard" {
				phasePenalty += 0.50
			}
			if blendedJumpPenalty(rankCtx, prev, item.track, mode) > 0.16 {
				phasePenalty += 0.20
			}
		case "develop":
			if item.bucket == bucketBridge {
				phaseBoost += 0.08
			}
			if item.bucket == bucketDiscovery && mode == TrajectoryExplore {
				phaseBoost += 0.05
			}
		case "resolve":
			if item.strategy == "wildcard" {
				phasePenalty += 0.35
			}
			if blendedJumpPenalty(rankCtx, prev, item.track, mode) > 0.18 {
				phasePenalty += 0.20
			}
			phaseBoost += clamp01(item.track.Melodicness*0.35+item.track.Softness*0.25+item.track.Relaxed*0.25) * 0.10
		}
		total += phaseBoost
		total -= phasePenalty
		total -= modePenalty
		if total > bestScore {
			bestIdx = i
			bestScore = total
			best = item
			best.score = total
			best.reason = explainEmotionTransition(rankCtx, prev, item.track)
			if best.reason == "" {
				best.reason = explainTransition(prev, item.track, item.bucket)
			}
			best.trackInsight = queueInsight(rankCtx, seed, prev, history, item.track)
			best.trackInsight.Bucket = item.bucket
			best.trackInsight.Strategy = item.strategy
			best.trackInsight.Transition = explainEmotionTransition(rankCtx, prev, item.track)
			if best.trackInsight.Transition == "" {
				best.trackInsight.Transition = transitionLabel(prev, item.track)
			}
			best.trackInsight.EnergyDirection = emotionDirectionLabel(rankCtx, prev, item.track)
			best.trackInsight.Bridge = item.bucket == bucketBridge
			best.trackInsight.Discovery = item.bucket == bucketDiscovery
			best.trackInsight.Confidence = clamp01(1 - skipRiskScore(item.track, feedback[item.track.ID]))
		}
	}
	return bestIdx, best, bestIdx >= 0, nil
}

func checkRayBuildContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf(
			"%w: %v",
			ErrRayBuildCanceled,
			ctx.Err(),
		)
	default:
		return nil
	}
}

func bucketQuotaPressure(queue []rays.QueueItem, bucket string, maxSize int) float64 {
	if len(queue) <= 1 {
		return 1
	}
	coreTarget, adjacentTarget, discoveryTarget := bucketTargets(maxSize)
	bridgeTarget := max(2, maxSize/5)
	counts := map[string]int{}
	for _, item := range queue[1:] {
		counts[item.Bucket]++
	}
	target := coreTarget
	switch bucket {
	case bucketAdjacent:
		target = adjacentTarget
	case bucketBridge:
		target = bridgeTarget
	case bucketDiscovery:
		target = discoveryTarget
	}
	if target <= 0 {
		return 0.7
	}
	ratio := float64(counts[bucket]+1) / float64(target)
	if ratio <= 1 {
		return 1 - (ratio-0.5)*0.2
	}
	return clamp01(1.1 - (ratio-1)*0.45)
}

func queueInsight(ctx rankingContext, seed, prev library.Track, history []library.Track, cand library.Track) rays.QueueInsight {
	prevEmotion := emotionFromContext(ctx, prev)
	candEmotion := emotionFromContext(ctx, cand)
	pm := calibrateMood(prev)
	cm := calibrateMood(cand)
	texture := textureContinuity(prev, cand)
	vocal := vocalContinuity(prev, cand)
	tempo := tempoCompatibility(prev, cand)
	session := sessionMoodFit(ctx, history, cand)
	targetMood := clamp01(1 - moodDistance(ctx.TargetMood, cm)/3.5)
	dist := emotion.Distance(prevEmotion.Basis, candEmotion.Basis)
	hard := emotion.HardJumpRisk(prevEmotion.Basis, candEmotion.Basis)
	bridge := emotion.BridgeScore(prevEmotion.Basis, candEmotion.Basis)
	ins := rays.QueueInsight{
		Similarity:         vectorSim(seed.Embedding, cand.Embedding),
		MoodSmoothness:     moodSmoothness(prev, cand),
		MoodDistance:       moodDistance(pm, cm),
		EnergyDelta:        cm.SoftEnergy - pm.SoftEnergy,
		JumpPenalty:        transitionJumpPenalty(prev, cand),
		Novelty:            noveltyScore(cand),
		TempoCompatibility: tempo,
		TempoUnknown:       tempoTrust(prev) <= 0 || tempoTrust(cand) <= 0,
		TextureContinuity:  texture,
		VocalContinuity:    vocal,
		SessionFit:         session,
		TargetMoodFit:      targetMood,
		Mode:               string(ctx.Mode),
		Transition:         explainEmotionTransition(ctx, prev, cand),
		EnergyDirection:    emotionDirectionLabel(ctx, prev, cand),
		Confidence:         insightConfidenceWithNormalizer(cand, ctx.Normalizer),
		LowTrustFeatures:   lowTrustFeatureNames(ctx.Normalizer),
		Fallback:           insightFallbackLabel(cand),
		Emotion: rays.EmotionBasisInsight{
			Label:             candEmotion.Basis.Label,
			PrevLabel:         prevEmotion.Basis.Label,
			Distance:          dist,
			Smoothness:        1 - dist,
			HardJump:          hard,
			BridgeScore:       bridge,
			RawDistance:       rawSensoryDistance(prev, cand),
			TextureConfidence: candEmotion.Debug.TextureConfidence,
			EdgeDrive:         candEmotion.Debug.EdgeDrive,
			DirtyElectro:      candEmotion.Debug.DirtyElectro,
			Joy:               candEmotion.Basis.Joy,
			Melancholy:        candEmotion.Basis.Melancholy,
			Serenity:          candEmotion.Basis.Serenity,
			Combat:            candEmotion.Basis.Combat,
			Pressure:          candEmotion.Basis.Pressure,
			Roughness:         candEmotion.Basis.Roughness,
			Swagger:           candEmotion.Basis.Swagger,
			SprintClean:       candEmotion.Basis.SprintClean,
		},
	}
	ins.Warning = insightWarning(ins, cand)
	ins.Confidence = capConfidenceByWarning(ins.Confidence, ins.Warning)
	return ins
}

func inferRayMode(seed library.Track, tracks []library.Track, feedback map[string]feedbackItem, normalizer FeatureNormalizer) RayTrajectoryMode {
	seedEmotion := emotion.Compute(seed, normalizer).Basis

	if seedEmotion.Combat >= 0.48 || seedEmotion.Pressure >= 0.54 || seedEmotion.Roughness >= 0.50 {
		return TrajectoryContinueMood
	}
	if seedEmotion.Melancholy >= 0.58 && seedEmotion.Serenity >= 0.50 && seedEmotion.Joy < 0.42 {
		return TrajectoryDeepen
	}
	if seedEmotion.Serenity >= 0.66 && seedEmotion.Pressure < 0.34 && seedEmotion.Combat < 0.32 {
		return TrajectoryWarmUp
	}

	seedMood := calibrateMood(seed)
	if seedMood.Calmness >= 0.72 || seedMood.SoftEnergy <= 0.34 {
		return TrajectoryWarmUp
	}
	if seedMood.SoftEnergy >= 0.78 && seedEmotion.Combat < 0.35 && seedEmotion.Pressure < 0.42 {
		return TrajectoryCoolDown
	}
	if seedMood.Brightness < 0.35 && seedMood.Darkness > 0.45 && seedMood.Movement < 0.60 {
		return TrajectoryDeepen
	}
	novelCount := 0
	for _, t := range tracks {
		if t.ID == seed.ID {
			continue
		}
		if noveltyScore(t) >= 0.75 && feedback[t.ID].skipEvents == 0 {
			novelCount++
		}
	}
	if novelCount >= 6 {
		return TrajectoryExplore
	}
	return TrajectoryContinueMood
}

func normalizeRayMode(requested string, fallback RayTrajectoryMode) RayTrajectoryMode {
	switch RayTrajectoryMode(strings.TrimSpace(requested)) {
	case TrajectoryContinueMood, TrajectoryWarmUp, TrajectoryCoolDown, TrajectoryExplore, TrajectoryDeepen:
		return RayTrajectoryMode(strings.TrimSpace(requested))
	default:
		return fallback
	}
}

func buildEnergyCurve(seed library.Track, mode RayTrajectoryMode, n int) []float64 {
	if n < 1 {
		n = 1
	}
	curve := make([]float64, n)
	start := calibrateMood(seed).SoftEnergy
	curve[0] = start
	for i := 1; i < n; i++ {
		frac := float64(i) / float64(max(1, n-1))
		target := start
		switch mode {
		case TrajectoryWarmUp:
			target = clamp01(start + 0.22*frac)
		case TrajectoryCoolDown:
			target = clamp01(start - 0.22*frac)
		case TrajectoryExplore:
			drift := 0.10
			if i%2 == 0 {
				target = clamp01(start + drift*frac)
			} else {
				target = clamp01(start - drift*frac*0.6)
			}
		case TrajectoryDeepen:
			target = clamp01(start + 0.08*frac)
		default:
			target = clamp01(start + 0.04*(0.5-frac))
		}
		curve[i] = target
	}
	return curve
}

func buildRankingContext(seed library.Track, history []library.Track, mode RayTrajectoryMode, curve []float64, position int, queueLength int) rankingContext {
	seedMood := calibrateMood(seed)
	sessionMood := averageMood(lastN(history, 5))
	targetMood := seedMood
	if sessionMood != (calibratedMood{}) {
		targetMood = blendMood(seedMood, sessionMood, 0.5)
	}
	ctx := rankingContext{Seed: seed, Mode: mode, Position: position, QueueLength: queueLength, TargetEnergy: seedMood.SoftEnergy, SessionMood: sessionMood, TargetMood: targetMood, Exploration: 0.08, Temperature: 0.18}
	if position >= 0 && position < len(curve) {
		ctx.TargetEnergy = curve[position]
	} else if len(curve) > 0 {
		ctx.TargetEnergy = curve[len(curve)-1]
	}
	if mode == TrajectoryExplore {
		ctx.Exploration = 0.20
		ctx.Temperature = 0.28
	}
	if mode == TrajectoryDeepen {
		ctx.Exploration = 0.04
		ctx.Temperature = 0.14
	}
	return ctx
}

func lowTrustFeatureNames(n FeatureNormalizer) []string {
	if len(n.Stats) == 0 {
		return nil
	}
	out := make([]string, 0)
	for name, st := range n.Stats {
		if st.Invalid || st.Reliability < 0.25 {
			out = append(out, strings.ToLower(strings.ReplaceAll(name, "TimbreBrightness", "timbre_brightness")))
		}
	}
	sort.Strings(out)
	return out
}

func logFeatureHealth(n FeatureNormalizer) {
	for name, st := range n.Stats {
		if st.Invalid || st.Reliability < 0.25 {
			recommendLog.I("feature low trust name=%s reliability=%.2f spread=%.3f p05=%.3f p50=%.3f p95=%.3f zero=%.2f one=%.2f binary=%.2f reason=%s", strings.ToLower(name), st.Reliability, st.Spread, st.P05, st.P50, st.P95, st.NearZeroRatio, st.NearOneRatio, st.BinaryRatio, st.Reason)
		}
	}
}

func modeLabel(mode RayTrajectoryMode) string {
	switch mode {
	case TrajectoryWarmUp:
		return "разгон"
	case TrajectoryCoolDown:
		return "мягкая посадка"
	case TrajectoryExplore:
		return "осторожное исследование"
	case TrajectoryDeepen:
		return "углубление"
	default:
		return "удержание вайба"
	}
}

func transitionFeedbackScore(stats map[string]strategyStat, prev, cand library.Track) float64 {
	if len(stats) == 0 {
		return 0
	}
	key := transitionKey(prev, cand)
	st, ok := stats[key]
	if !ok || st.plays <= 0 {
		return 0
	}
	mean := st.reward / float64(st.plays)
	return clamp01((mean+1)/2) - 0.5
}

func TransitionRewardKey(prev, next library.Track) string { return transitionKey(prev, next) }

func transitionKey(prev, next library.Track) string {
	pm := calibrateMood(prev)
	nm := calibrateMood(next)
	energy := "steady"
	delta := nm.SoftEnergy - pm.SoftEnergy
	switch {
	case delta > 0.18:
		energy = "up"
	case delta < -0.18:
		energy = "down"
	}
	color := "neutral"
	if nm.Brightness-pm.Brightness > 0.18 {
		color = "brighter"
	} else if nm.Darkness-pm.Darkness > 0.18 {
		color = "darker"
	}
	texture := "steady"
	if nm.Electronicness-pm.Electronicness > 0.20 {
		texture = "more_electronic"
	} else if nm.Organicness-pm.Organicness > 0.20 {
		texture = "more_organic"
	}
	if jumpPenalty(prev, next) > 0.24 {
		return "transition:jump:" + energy + ":" + color + ":" + texture
	}
	return "transition:smooth:" + energy + ":" + color + ":" + texture
}

func trackLookup(tracks []library.Track) map[string]library.Track {
	out := make(map[string]library.Track, len(tracks))
	for _, track := range tracks {
		out[track.ID] = track
	}
	return out
}

func queueTracks(queue []rays.QueueItem, lookup map[string]library.Track) []library.Track {
	out := make([]library.Track, 0, len(queue))
	for _, item := range queue {
		if track, ok := lookup[item.TrackID]; ok {
			out = append(out, track)
		}
	}
	return out
}

func flowScore(ctx rankingContext, seed, prev library.Track, history []library.Track, cand library.Track) float64 {
	prevEmotion := emotionFromContext(ctx, prev)
	candEmotion := emotionFromContext(ctx, cand)
	seedEmotion := emotionFromContext(ctx, seed)
	targetEmotion := emotionTargetFromMode(seedEmotion.Basis, ctx.Mode, ctx.Position, ctx.QueueLength)
	emotionDistance := emotion.Distance(prevEmotion.Basis, candEmotion.Basis)
	emotionSmooth := 1 - emotionDistance
	emotionJump := emotion.HardJumpRisk(prevEmotion.Basis, candEmotion.Basis)
	targetEmotionFit := 1 - emotion.Distance(targetEmotion, candEmotion.Basis)
	emotionBridge := emotion.BridgeScore(prevEmotion.Basis, candEmotion.Basis)

	mood := moodSmoothness(prev, cand)
	energy := energyStepFit(ctx, seed, prev, history, cand)
	session := sessionMoodFit(ctx, history, cand)
	targetMood := clamp01(1 - moodDistance(ctx.TargetMood, calibrateMood(cand))/5.8)
	driftPenalty := sessionMoodDriftPenalty(history, cand)
	aggressionPenalty := aggressionTransitionPenalty(prev, cand, ctx.Mode)
	jump := jumpPenalty(prev, cand)
	modeBonus := modeFit(ctx.Mode, prev, cand)
	texture := textureContinuity(prev, cand)
	vocal := vocalContinuity(prev, cand)
	tempo := tempoCompatibility(prev, cand)
	melodic := melodicness(cand)
	tempoDir := tempoDirectionFit(ctx.Mode, prev, cand)
	moodFit := clamp01(1 - moodDistance(calibrateMood(prev), calibrateMood(cand))/5.8)
	jumpRisk := jumpPenaltyFromMood(calibrateMood(prev), calibrateMood(cand))
	if hardJumpRisk(prev, cand, ctx.Mode) {
		jumpRisk = math.Max(jumpRisk, 0.32)
	}
	if ctx.Mode == TrajectoryExplore && hardJumpRisk(prev, cand, ctx.Mode) {
		jumpRisk = math.Max(jumpRisk, 0.28)
	}
	legacyFlow := clamp01(0.42*moodFit + 0.15*tempo + 0.10*texture + 0.08*vocal + 0.12*session + 0.13*targetMood - 0.26*jumpRisk)
	perceptualFlow := clamp01(
		emotionSmooth*0.34 +
			(1-emotionJump)*0.20 +
			targetEmotionFit*0.16 +
			emotionBridge*0.10 +
			tempoCompatibility(prev, cand)*0.08 +
			textureContinuity(prev, cand)*0.05 +
			vocalContinuity(prev, cand)*0.03 +
			sessionMoodFit(ctx, history, cand)*0.04,
	)
	return clamp01(legacyFlow*0.45 + perceptualFlow*0.55 - emotionJump*0.10 + mood*0.10 + energy*0.06 + modeBonus*0.03 + texture*0.02 + vocal*0.02 + tempo*0.03 + tempoDir*0.02 + melodic*0.01 - driftPenalty*0.06 - aggressionPenalty*0.06 - jump*0.05 + ctx.Exploration*noveltyScore(cand)*0.08)
}

func emotionTargetFromMode(seed emotion.Basis, mode RayTrajectoryMode, position, queueLength int) emotion.Basis {
	target := seed
	p := 0.0
	if queueLength > 0 {
		p = clamp01(float64(position) / float64(queueLength))
	}
	switch mode {
	case TrajectoryWarmUp:
		target.Motion = clamp01(seed.Motion + 0.18*p)
		target.Pressure = clamp01(seed.Pressure + 0.10*p)
		target.Joy = clamp01(seed.Joy + 0.14*p)
		target.Serenity = clamp01(seed.Serenity - 0.08*p)
	case TrajectoryCoolDown:
		target.Pressure = clamp01(seed.Pressure - 0.18*p)
		target.Combat = clamp01(seed.Combat - 0.20*p)
		target.Roughness = clamp01(seed.Roughness - 0.14*p)
		target.Serenity = clamp01(seed.Serenity + 0.18*p)
	case TrajectoryDeepen:
		target.Melancholy = clamp01(seed.Melancholy + 0.10*p)
		target.Serenity = clamp01(seed.Serenity + 0.08*p)
		target.Joy = clamp01(seed.Joy - 0.08*p)
	case TrajectoryExplore:
		target.Swagger = clamp01(seed.Swagger + 0.10*p)
		target.Brightness = clamp01(seed.Brightness + 0.08*p)
	}
	return target
}

func calibrateMood(t library.Track) calibratedMood {
	raw := rawMoodFeatures(t)
	interp := interpretMood(raw)
	return calibratedMood{
		SoftEnergy:       interp.Energy,
		Brightness:       clamp01(raw.Brightness),
		Darkness:         interp.Darkness,
		Calmness:         interp.Calmness,
		Aggression:       clamp01(interp.Intensity),
		Movement:         interp.Movement,
		Edge:             audioEdge(raw),
		Electronicness:   clamp01(raw.Electronicness),
		Organicness:      clamp01(raw.Acousticness),
		VocalPresence:    interp.Vocalness,
		Melodicness:      clamp01(raw.Melodicness),
		Softness:         clamp01(raw.Softness),
		Heaviness:        clamp01(raw.Heaviness),
		Dreaminess:       clamp01(raw.Dreaminess),
		Emotionality:     clamp01(raw.Emotionality),
		TimbreBrightness: clamp01(raw.Brightness),
		Tonality:         clamp01(raw.Tonality),
		Approachability:  clamp01(t.Approachability),
		Engagement:       clamp01(t.Engagement),
	}
}

func rawMoodFeatures(t library.Track) RawMoodFeatures {
	return RawMoodFeatures{
		Danceability:     clamp01(t.Danceability),
		Happy:            clamp01(t.Happy),
		Sad:              clamp01(t.Sad),
		Relaxed:          clamp01(t.Relaxed),
		Party:            clamp01(t.Party),
		Aggressive:       clamp01(t.Aggressive),
		Acousticness:     clamp01(t.Acousticness),
		Electronicness:   clamp01(t.Electronicness),
		Instrumentalness: clamp01(t.Instrumentalness),
		Vocalness:        clamp01(t.Vocalness),
		Brightness:       clamp01(t.TimbreBrightness),
		Tonality:         clamp01(t.Tonality),
		Melodicness:      clamp01(t.Melodicness),
		Softness:         clamp01(t.Softness),
		Heaviness:        clamp01(t.Heaviness),
		Dreaminess:       clamp01(t.Dreaminess),
		Emotionality:     clamp01(t.Emotionality),
		Valence:          clamp01(t.Valence),
		TempoPulse:       tempoPulseScore(t),
		ZeroCrossingRate: t.ZeroCrossingRate,
		SpectralCentroid: t.SpectralCentroid,
	}
}

func tempoPulseScore(t library.Track) float64 {
	pulse := t.BPMPerceived / 180.0
	if pulse <= 0 {
		pulse = t.Tempo / 180.0
	}
	return clamp01(pulse * (0.60 + 0.40*clamp01(t.TempoConfidence)))
}

func audioEdge(raw RawMoodFeatures) float64 {
	return clamp01(0.28*clamp01(raw.ZeroCrossingRate/0.22) + 0.20*clamp01((raw.SpectralCentroid-900.0)/1800.0) + 0.18*clamp01(raw.Brightness) + 0.16*clamp01(raw.Electronicness) + 0.12*(1-clamp01(raw.Softness)) + 0.06*(1-clamp01(raw.Melodicness)))
}

func interpretMood(raw RawMoodFeatures) InterpretedMood {
	pulse := clamp01(raw.TempoPulse)
	edge := audioEdge(raw)
	movement := clamp01(0.42*raw.Danceability + 0.24*pulse + 0.14*raw.Party + 0.12*raw.Electronicness + 0.08*(1-raw.Softness))
	pressure := clamp01(0.30*movement + 0.22*raw.Brightness + 0.16*edge + 0.14*(1-raw.Softness) + 0.10*raw.Party + 0.08*raw.Electronicness)
	calmness := clamp01(0.34*raw.Relaxed + 0.26*raw.Softness + 0.16*(1-movement) + 0.14*(1-pressure) + 0.10*(1-pulse) - 0.18*edge)
	texture := clamp01(0.58*raw.Electronicness + 0.22*(1-raw.Acousticness) + 0.20*raw.Brightness)
	coolness := clamp01(0.34*(1-raw.Brightness) + 0.24*(1-raw.Valence) + 0.18*texture + 0.14*raw.Relaxed + 0.10*(1-raw.Party))
	warmth := clamp01(0.34*raw.Valence + 0.28*raw.Brightness + 0.18*raw.Happy + 0.12*raw.Party + 0.08*(1-coolness))
	energy := clamp01(0.34*movement + 0.28*pressure + 0.20*pulse + 0.18*raw.Brightness)
	intensity := clamp01(0.32*pressure + 0.24*energy + 0.16*(1-raw.Softness) + 0.14*raw.Party + 0.14*raw.Brightness)
	confidence := clamp01(0.32 + 0.12*raw.Danceability + 0.10*pulse + 0.08*raw.Brightness + 0.08*(1-raw.Sad))
	return InterpretedMood{Movement: movement, Drive: pressure, Energy: energy, Intensity: intensity, Calmness: calmness, Tension: pressure, Atmosphere: clamp01(0.50*calmness + 0.25*texture + 0.25*warmth), Club: clamp01(0.58*raw.Danceability + 0.22*raw.Party + 0.20*raw.Vocalness), Darkness: clamp01(1 - warmth), Valence: clamp01(raw.Valence), Texture: texture, Vocalness: raw.Vocalness, Confidence: confidence}
}

func moodBasisFromTrack(t library.Track) MoodBasis {
	raw := rawMoodFeatures(t)
	interp := interpretMood(raw)
	return MoodBasis{Motion: interp.Movement, Pressure: interp.Drive, Calmness: interp.Calmness, Coolness: clamp01((1-raw.Brightness)*0.34 + (1-raw.Valence)*0.24 + interp.Texture*0.18 + raw.Relaxed*0.14 + (1-raw.Party)*0.10), Warmth: clamp01(raw.Valence*0.34 + raw.Brightness*0.28 + raw.Happy*0.18 + raw.Party*0.12 + (1-interp.Texture)*0.08), Texture: interp.Texture, Edge: audioEdge(raw), Valence: clamp01(raw.Valence), Vocality: clamp01(raw.Vocalness)}
}

func capConfidenceByWarning(conf float64, warning string) float64 {
	if warning == "" {
		return clamp01(conf)
	}
	w := strings.ToLower(warning)
	if strings.Contains(w, "tempo_unknown") {
		conf = math.Min(conf, 0.72)
	}
	if strings.Contains(w, "sad_saturated") || strings.Contains(w, "tonal_saturated") || strings.Contains(w, "approach_saturated") || strings.Contains(w, "engage_saturated") {
		conf = math.Min(conf, 0.70)
	}
	if strings.Contains(w, "conf_suspect") {
		conf = math.Min(conf, 0.64)
	}
	return clamp01(conf)
}

func moodDistance(a, b calibratedMood) float64 {
	return weightedAbs(a.SoftEnergy, b.SoftEnergy, 0.10) +
		weightedAbs(a.Brightness, b.Brightness, 0.10) +
		weightedAbs(a.Darkness, b.Darkness, 0.08) +
		weightedAbs(a.Calmness, b.Calmness, 0.14) +
		weightedAbs(a.Aggression, b.Aggression, 0.10) +
		weightedAbs(a.Movement, b.Movement, 0.08) +
		weightedAbs(a.Edge, b.Edge, 0.46) +
		weightedAbs(a.Electronicness, b.Electronicness, 0.06) +
		weightedAbs(a.Organicness, b.Organicness, 0.05) +
		weightedAbs(a.VocalPresence, b.VocalPresence, 0.03) +
		weightedAbs(a.Melodicness, b.Melodicness, 0.05) +
		weightedAbs(a.Softness, b.Softness, 0.05) +
		weightedAbs(a.Heaviness, b.Heaviness, 0.03) +
		weightedAbs(a.Dreaminess, b.Dreaminess, 0.02) +
		weightedAbs(a.TimbreBrightness, b.TimbreBrightness, 0.06) +
		weightedAbs(a.Tonality, b.Tonality, 0.03)
}

func moodSmoothness(prev, next library.Track) float64 {
	dist := moodDistance(calibrateMood(prev), calibrateMood(next))
	return clamp01(1 - dist/5.8)
}

func transitionJumpPenalty(prev, next library.Track) float64 {
	risk := 0.0
	risk += math.Abs(prev.Energy-next.Energy) * 0.18
	risk += math.Abs(prev.Valence-next.Valence) * 0.14
	risk += math.Abs(prev.Happy-next.Happy) * 0.08
	risk += math.Abs(prev.Sad-next.Sad) * 0.08
	risk += math.Abs(prev.Relaxed-next.Relaxed) * 0.10
	risk += math.Abs(prev.Party-next.Party) * 0.10
	risk += math.Abs(prev.Aggressive-next.Aggressive) * 0.12
	risk += math.Abs(prev.Acousticness-next.Acousticness) * 0.07
	risk += math.Abs(prev.Electronicness-next.Electronicness) * 0.07
	risk += math.Abs(prev.Vocalness-next.Vocalness) * 0.04
	risk += math.Abs(prev.TimbreBrightness-next.TimbreBrightness) * 0.04
	if tempoTrust(prev) > 0 && tempoTrust(next) > 0 {
		risk += (1 - tempoCompatibility(prev, next)) * 0.18
	}
	return clamp01(risk)
}

func jumpPenalty(prev, next library.Track) float64 {
	return transitionJumpPenalty(prev, next)
}

func jumpPenaltyFromMood(a, b calibratedMood) float64 {
	p := 0.0
	p += math.Abs(a.Edge-b.Edge) * 0.42
	p += math.Abs(a.Pressure-b.Aggression) * 0.22
	p += math.Abs(a.Calmness-b.Calmness) * 0.18
	p += math.Abs(a.SoftEnergy-b.SoftEnergy) * 0.10
	p += math.Abs(a.Brightness-b.Brightness) * 0.08
	if math.Abs(a.Electronicness-b.Electronicness)+math.Abs(a.Organicness-b.Organicness) > 0.90 {
		p += 0.08
	}
	if math.Abs(a.TimbreBrightness-b.TimbreBrightness) > 0.55 {
		p += 0.06
	}
	if math.Abs(a.VocalPresence-b.VocalPresence) > 0.70 {
		p += 0.04
	}
	return clamp01(p)
}

func blendMood(a, b calibratedMood, ratio float64) calibratedMood {
	inv := 1 - ratio
	return calibratedMood{SoftEnergy: a.SoftEnergy*inv + b.SoftEnergy*ratio, Brightness: a.Brightness*inv + b.Brightness*ratio, Darkness: a.Darkness*inv + b.Darkness*ratio, Calmness: a.Calmness*inv + b.Calmness*ratio, Aggression: a.Aggression*inv + b.Aggression*ratio, Movement: a.Movement*inv + b.Movement*ratio, Electronicness: a.Electronicness*inv + b.Electronicness*ratio, Organicness: a.Organicness*inv + b.Organicness*ratio, VocalPresence: a.VocalPresence*inv + b.VocalPresence*ratio, Melodicness: a.Melodicness*inv + b.Melodicness*ratio, Softness: a.Softness*inv + b.Softness*ratio, Heaviness: a.Heaviness*inv + b.Heaviness*ratio, Dreaminess: a.Dreaminess*inv + b.Dreaminess*ratio, Emotionality: a.Emotionality*inv + b.Emotionality*ratio, TimbreBrightness: a.TimbreBrightness*inv + b.TimbreBrightness*ratio, Tonality: a.Tonality*inv + b.Tonality*ratio, Approachability: a.Approachability*inv + b.Approachability*ratio, Engagement: a.Engagement*inv + b.Engagement*ratio}
}

func energyStepFit(ctx rankingContext, seed, prev library.Track, history []library.Track, cand library.Track) float64 {
	target := ctx.TargetEnergy
	if target <= 0 {
		target = calibrateMood(seed).SoftEnergy
	}
	if len(history) >= 2 {
		target = clamp01(target*0.65 + averageMood(history).SoftEnergy*0.35)
	}
	step := 1 - math.Abs(calibrateMood(cand).SoftEnergy-target)
	jump := math.Abs(calibrateMood(cand).SoftEnergy - calibrateMood(prev).SoftEnergy)
	if jump > 0.30 {
		step -= (jump - 0.30) * 0.8
	}
	return clamp01(step)
}

func sessionMoodDriftPenalty(history []library.Track, cand library.Track) float64 {
	if len(history) == 0 {
		return 0
	}
	recentMood := averageMood(lastN(history, 5))
	dist := moodDistance(recentMood, calibrateMood(cand))
	if dist < 0.8 {
		return 0
	}
	return clamp01((dist - 0.8) / 2.0)
}

func sessionMoodFit(ctx rankingContext, history []library.Track, cand library.Track) float64 {
	if len(history) == 0 {
		return 1
	}
	base := averageMood(lastN(history, 5))
	if ctx.SessionMood != (calibratedMood{}) {
		base = ctx.SessionMood
	}
	dist := moodDistance(base, calibrateMood(cand))
	fit := clamp01(1 - dist/5.8)
	if jumpPenaltyFromMood(base, calibrateMood(cand)) > 0.24 {
		fit *= 0.75
	}
	return fit
}

func averageMood(tracks []library.Track) calibratedMood {
	if len(tracks) == 0 {
		return calibratedMood{}
	}
	var out calibratedMood
	for _, track := range tracks {
		m := calibrateMood(track)
		out.SoftEnergy += m.SoftEnergy
		out.Brightness += m.Brightness
		out.Darkness += m.Darkness
		out.Calmness += m.Calmness
		out.Aggression += m.Aggression
		out.Movement += m.Movement
		out.Electronicness += m.Electronicness
		out.Organicness += m.Organicness
		out.VocalPresence += m.VocalPresence
		out.Melodicness += m.Melodicness
		out.Softness += m.Softness
		out.Heaviness += m.Heaviness
		out.Dreaminess += m.Dreaminess
		out.Emotionality += m.Emotionality
		out.TimbreBrightness += m.TimbreBrightness
		out.Tonality += m.Tonality
		out.Approachability += m.Approachability
		out.Engagement += m.Engagement
	}
	inv := 1 / float64(len(tracks))
	out.SoftEnergy *= inv
	out.Brightness *= inv
	out.Darkness *= inv
	out.Calmness *= inv
	out.Aggression *= inv
	out.Movement *= inv
	out.Electronicness *= inv
	out.Organicness *= inv
	out.VocalPresence *= inv
	out.Melodicness *= inv
	out.Softness *= inv
	out.Heaviness *= inv
	out.Dreaminess *= inv
	out.Emotionality *= inv
	out.TimbreBrightness *= inv
	out.Tonality *= inv
	out.Approachability *= inv
	out.Engagement *= inv
	return out
}

func aggressionTransitionPenalty(prev, next library.Track, mode RayTrajectoryMode) float64 {
	delta := calibrateMood(next).Aggression - calibrateMood(prev).Aggression
	if delta <= 0.20 {
		return 0
	}
	switch mode {
	case TrajectoryWarmUp:
		return clamp01((delta - 0.25) * 1.5)
	case TrajectoryExplore:
		return clamp01((delta - 0.35) * 1.0)
	default:
		return clamp01((delta - 0.20) * 2.0)
	}
}

func tempoDistance(a, b float64) float64 {
	if a <= 0 || b <= 0 {
		return 1
	}
	candidates := []float64{
		math.Abs(a - b),
		math.Abs(a*2 - b),
		math.Abs(a*0.5 - b),
		math.Abs(a - b*2),
		math.Abs(a - b*0.5),
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate < best {
			best = candidate
		}
	}
	return clamp01(best / 24.0)
}

func tempoCompatibility(prev, next library.Track) float64 {
	if prev.TempoConfidence < 0.35 || next.TempoConfidence < 0.35 {
		return 0.5
	}
	if tempoTrust(prev) <= 0 || tempoTrust(next) <= 0 {
		return 0.5
	}
	a := prev.BPMPerceived
	if a <= 0 {
		a = prev.Tempo
	}
	b := next.BPMPerceived
	if b <= 0 {
		b = next.Tempo
	}
	if a <= 0 || b <= 0 {
		return 0.5
	}
	diff := tempoDistanceNormalized(a, b)
	return clamp01(1 - diff*1.35)
}

func tempoDistanceNormalized(a, b float64) float64 {
	if a <= 0 || b <= 0 {
		return 1
	}
	candidates := []float64{
		math.Abs(a-b) / math.Max(a, b),
		math.Abs(a*2-b) / math.Max(a*2, b),
		math.Abs(a*0.5-b) / math.Max(a*0.5, b),
		math.Abs(a-b*2) / math.Max(a, b*2),
		math.Abs(a-b*0.5) / math.Max(a, b*0.5),
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c < best {
			best = c
		}
	}
	return clamp01(best)
}

func tempoDirectionFit(mode RayTrajectoryMode, current, candidate library.Track) float64 {
	a := current.BPMPerceived
	if a <= 0 {
		a = current.Tempo
	}
	b := candidate.BPMPerceived
	if b <= 0 {
		b = candidate.Tempo
	}
	if a <= 0 || b <= 0 {
		return 0.5
	}
	delta := b - a
	switch mode {
	case TrajectoryWarmUp:
		if delta >= 0 && delta <= 10 {
			return 1
		}
		if delta > 10 && delta <= 20 {
			return 0.65
		}
		if delta >= -5 {
			return 0.55
		}
		return 0.25
	case TrajectoryCoolDown:
		if delta <= 0 && delta >= -10 {
			return 1
		}
		if delta < -10 && delta >= -20 {
			return 0.65
		}
		if delta <= 5 {
			return 0.55
		}
		return 0.25
	case TrajectoryExplore:
		return clamp01(0.35 + tempoCompatibility(current, candidate)*0.65)
	default:
		return tempoCompatibility(current, candidate)
	}
}

func textureContinuity(prev, next library.Track) float64 {
	jump := math.Abs(prev.Electronicness-next.Electronicness) + math.Abs(prev.Acousticness-next.Acousticness)
	return clamp01(1 - jump/2)
}

func vocalContinuity(prev, next library.Track) float64 {
	return clamp01(1 - math.Abs(calibrateMood(prev).VocalPresence-calibrateMood(next).VocalPresence))
}

func melodicness(t library.Track) float64 {
	tonal := clamp01((1-clamp01(t.ZeroCrossingRate*5))*0.35 + clamp01(1-math.Abs(t.SpectralCentroid-1800)/1800)*0.25 + clamp01(t.Vocalness)*0.20 + clamp01(t.Acousticness)*0.20)
	return tonal
}

func modeFit(mode RayTrajectoryMode, prev, cand library.Track) float64 {
	pm := calibrateMood(prev)
	cm := calibrateMood(cand)
	switch mode {
	case TrajectoryWarmUp:
		return clamp01(0.5 + (cm.SoftEnergy-pm.SoftEnergy)*1.4 + (cand.Party-prev.Party)*0.6)
	case TrajectoryCoolDown:
		return clamp01(0.5 + (pm.SoftEnergy-cm.SoftEnergy)*1.4 + (cand.Relaxed-prev.Relaxed)*0.6)
	case TrajectoryExplore:
		genreDrift := 0.0
		if !strings.EqualFold(strings.TrimSpace(prev.GenrePrimary), strings.TrimSpace(cand.GenrePrimary)) {
			genreDrift = 0.2
		}
		return clamp01(0.45 + genreDrift + noveltyScore(cand)*0.35 - jumpPenalty(prev, cand)*0.35)
	case TrajectoryDeepen:
		return clamp01(0.58 + genreAffinity(prev, cand)*0.18 + moodSmoothness(prev, cand)*0.18 + (1-noveltyScore(cand))*0.10 - jumpPenalty(prev, cand)*0.22)
	default:
		return clamp01(0.55 + moodSmoothness(prev, cand)*0.35 - jumpPenalty(prev, cand)*0.30)
	}
}

func explainMoodTransition(a, b MoodBasis) string {
	edgeDelta := b.Edge - a.Edge
	pressureDelta := b.Pressure - a.Pressure
	calmDelta := b.Calmness - a.Calmness
	coolDelta := b.Coolness - a.Coolness
	if edgeDelta > 0.16 && pressureDelta > 0.08 {
		return "становится жёстче и грязнее"
	}
	if edgeDelta < -0.16 && calmDelta > 0.08 {
		return "резко сглаживается и уходит в ночной грув"
	}
	if pressureDelta > 0.18 {
		return "становится тяжелее и мощнее"
	}
	if pressureDelta < -0.18 {
		return "сбрасывает напор"
	}
	if coolDelta > 0.16 {
		return "становится холоднее и задумчивее"
	}
	if math.Abs(edgeDelta) < 0.08 && math.Abs(pressureDelta) < 0.08 && math.Abs(calmDelta) < 0.10 {
		return "близкий грув"
	}
	return "мягкий мост"
}

func explainTransition(prev, next library.Track, bucket string) string {
	prevMood := calibrateMood(prev)
	nextMood := calibrateMood(next)
	energyDelta := nextMood.SoftEnergy - prevMood.SoftEnergy
	if jumpPenalty(prev, next) > 0.24 {
		return explainMoodTransition(moodBasisFromTrack(prev), moodBasisFromTrack(next))
	}
	if energyDelta > 0.10 && energyDelta < 0.25 {
		return "чуть энергичнее"
	}
	if energyDelta < -0.10 && energyDelta > -0.25 {
		return "чуть спокойнее"
	}
	if nextMood.Melodicness > prevMood.Melodicness+0.12 {
		return "чуть мелодичнее"
	}
	if nextMood.Calmness > prevMood.Calmness+0.12 || nextMood.Softness > prevMood.Softness+0.12 {
		return "спокойнее и мягче"
	}
	if nextMood.Darkness > prevMood.Darkness+0.12 {
		return "чуть мрачнее"
	}
	if moodSmoothness(prev, next) > 0.85 {
		return "похожее настроение"
	}
	if bucket == bucketDiscovery && moodSmoothness(prev, next) > 0.65 {
		return "новое, но в том же вайбе"
	}
	if nextMood.Aggression > prevMood.Aggression+0.15 || nextMood.Heaviness > prevMood.Heaviness+0.15 {
		return "плавно тяжелее"
	}
	if nextMood.Electronicness > prevMood.Electronicness+0.18 {
		return "больше электроники"
	}
	if nextMood.Organicness > prevMood.Organicness+0.18 {
		return "более органичное звучание"
	}
	return "подходит к текущему лучу"
}

func explainEmotionTransition(ctx rankingContext, prev, cand library.Track) string {
	a := emotionFromContext(ctx, prev).Basis
	b := emotionFromContext(ctx, cand).Basis
	dist := emotion.Distance(a, b)
	hard := emotion.HardJumpRisk(a, b)
	rawDist := rawSensoryDistance(prev, cand)

	combatDelta := b.Combat - a.Combat
	pressureDelta := b.Pressure - a.Pressure
	roughDelta := b.Roughness - a.Roughness
	joyDelta := b.Joy - a.Joy
	melDelta := b.Melancholy - a.Melancholy
	serenityDelta := b.Serenity - a.Serenity
	swaggerDelta := b.Swagger - a.Swagger
	sprintDelta := b.SprintClean - a.SprintClean

	if combatDelta > 0.10 || pressureDelta > 0.12 || roughDelta > 0.12 {
		if b.Combat >= 0.42 || b.Pressure >= 0.46 || b.Roughness >= 0.44 {
			return "повышает накал и становится жестче"
		}
	}
	if combatDelta < -0.14 && pressureDelta < -0.10 && serenityDelta > 0.08 {
		return "снимает агрессию и смягчает поток"
	}
	if joyDelta > 0.16 && b.Combat < 0.32 && b.Pressure < 0.42 && roughDelta < 0.10 {
		return "становится светлее и радостнее"
	}
	if melDelta > 0.16 && b.Serenity >= a.Serenity && b.Combat < 0.40 {
		return "уходит глубже в спокойную грусть"
	}
	if serenityDelta > 0.16 && pressureDelta < -0.08 && combatDelta <= 0 {
		return "успокаивает и разглаживает настроение"
	}
	if swaggerDelta > 0.14 && b.Combat < 0.42 {
		return "добавляет уверенный грув"
	}
	if sprintDelta > 0.14 && b.Pressure < 0.55 {
		return "ускоряется без резкого утяжеления"
	}
	if hard >= 0.30 || dist >= 0.26 || rawDist >= 0.38 {
		return "контрастный эмоциональный мост"
	}
	if dist < 0.08 && hard < 0.10 && rawDist < 0.18 && sameEmotionFamily(a.Label, b.Label) && a.Label != "neutral" && b.Label != "neutral" {
		return "близкий эмоциональный вайб"
	}
	return "мягкий мост между состояниями"
}

func emotionDirectionLabel(ctx rankingContext, prev, cand library.Track) string {
	a := emotionFromContext(ctx, prev).Basis
	b := emotionFromContext(ctx, cand).Basis
	switch {
	case b.Combat-a.Combat > 0.10 || b.Pressure-a.Pressure > 0.12 || b.Roughness-a.Roughness > 0.12:
		return "intensify"
	case a.Combat-b.Combat > 0.14 && a.Pressure-b.Pressure > 0.10:
		return "soften"
	case b.Joy-a.Joy > 0.16 && b.Combat < 0.32 && b.Pressure < 0.42:
		return "brighten"
	case b.Melancholy-a.Melancholy > 0.14:
		return "deepen"
	case b.Serenity-a.Serenity > 0.16 && b.Pressure < a.Pressure:
		return "cool_down"
	case b.Motion-a.Motion > 0.14:
		return "warm_up"
	default:
		return "stable"
	}
}

func sameEmotionFamily(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	return strings.HasPrefix(a, "serene_") && strings.HasPrefix(b, "serene_") ||
		strings.HasPrefix(a, "melancholy_") && strings.HasPrefix(b, "melancholy_") ||
		strings.HasPrefix(a, "joy_") && strings.HasPrefix(b, "joy_")
}

func emotionFamily(label string) string {
	switch {
	case strings.HasPrefix(label, "serene_"):
		return "calm"
	case label == "night_smooth" || label == "serene_warm_groove":
		return "warm"
	case strings.HasPrefix(label, "melancholy_"):
		return "melancholy"
	case label == "dramatic_arc":
		return "mixed"
	case strings.HasPrefix(label, "joy_") || label == "street_swagger":
		return "positive"
	case label == "uplift_drive":
		return "positive"
	case strings.HasPrefix(label, "combat_") || label == "dirty_electro_combat":
		return "danger"
	case label == "tense_pressure" || label == "melancholy_pressure":
		return "pressure"
	default:
		return "neutral"
	}
}

func familyTransitionPenalty(a, b string) float64 {
	fa := emotionFamily(a)
	fb := emotionFamily(b)
	if fa == "neutral" || fb == "neutral" || fa == fb {
		return 0
	}
	switch {
	case fa == "calm" && fb == "danger":
		return 0.85
	case fa == "danger" && fb == "calm":
		return 0.85
	case fa == "positive" && fb == "danger":
		return 0.75
	case fa == "danger" && fb == "positive":
		return 0.75
	case fa == "calm" && fb == "pressure":
		return 0.55
	case fa == "pressure" && fb == "calm":
		return 0.55
	case fa == "melancholy" && fb == "positive":
		return 0.50
	case fa == "positive" && fb == "melancholy":
		return 0.50
	case fa == "warm" && fb == "danger":
		return 0.70
	case fa == "danger" && fb == "warm":
		return 0.70
	case fa == "mixed" && fb == "calm":
		return 0.25
	case fa == "mixed" && fb == "pressure":
		return 0.25
	case fa == "calm" && fb == "mixed":
		return 0.25
	case fa == "pressure" && fb == "mixed":
		return 0.25
	default:
		return 0.35
	}
}

func rawSensoryDistance(a, b library.Track) float64 {
	tempo := tempoDistanceNormalized(effectiveTempo(a), effectiveTempo(b))
	zcr := scaledAbs(a.ZeroCrossingRate, b.ZeroCrossingRate, 0.16)
	centroid := logScaledAbs(a.SpectralCentroid, b.SpectralCentroid, 6000)
	rms := scaledAbs(a.RMS, b.RMS, 0.30)
	loud := scaledAbs(a.Loudness, b.Loudness, 14.0)
	elec := math.Abs(a.Electronicness - b.Electronicness)
	acoustic := math.Abs(a.Acousticness - b.Acousticness)
	vocal := math.Abs(a.Vocalness - b.Vocalness)
	valence := math.Abs(a.Valence - b.Valence)
	relax := math.Abs(a.Relaxed - b.Relaxed)
	return clamp01(tempo*0.12 + zcr*0.18 + centroid*0.20 + rms*0.10 + loud*0.08 + elec*0.10 + acoustic*0.06 + vocal*0.06 + valence*0.05 + relax*0.05)
}

func scaledAbs(a, b, scale float64) float64 {
	if scale <= 0 {
		return 0
	}
	if a == 0 && b == 0 {
		return 0
	}
	return clamp01(math.Abs(a-b) / scale)
}

func logScaledAbs(a, b, scale float64) float64 {
	if scale <= 0 {
		return 0
	}
	if a <= 0 && b <= 0 {
		return 0
	}
	return clamp01(math.Abs(math.Log1p(math.Max(0, a))-math.Log1p(math.Max(0, b))) / math.Log1p(scale))
}

func weightedAbs(a, b, w float64) float64 { return math.Abs(a-b) * w }

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func lastN(tracks []library.Track, n int) []library.Track {
	if len(tracks) <= n {
		return tracks
	}
	return tracks[len(tracks)-n:]
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func logTop(seed library.Track, name string, items []scored, n int) {
	for i := 0; i < minInt(n, len(items)); i++ {
		item := items[i]
		recommendLog.D("%s rank=%d seed=%q candidate=%q score=%.4f strategy=%s bucket=%s reason=%q cluster=%d plays=%d skips=%d", name, i+1, seed.Title, item.track.Title, item.score, item.strategy, item.bucket, item.reason, item.track.ClusterID, item.track.PlayCount, item.track.SkipCount)
	}
}

func lowerTrim(s string) string { return trimLower(s) }
func trimLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
