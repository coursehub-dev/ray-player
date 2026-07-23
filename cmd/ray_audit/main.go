package main

import (
	"flag"
	"fmt"
	"os"

	"ray-player1/internal/db"
	"ray-player1/internal/events"
	"ray-player1/internal/library"
	"ray-player1/internal/logx"
	"ray-player1/internal/recommend"
)

var auditLog = logx.New("ray_audit")

func main() {
	seedID := flag.String("seed", "", "track id")
	mode := flag.String("mode", "", "continue_mood|warm_up|cool_down|explore|deepen")
	limit := flag.Int("limit", 20, "number of rows")
	flag.Parse()
	if *seedID == "" {
		auditLog.E("--seed is required")
		os.Exit(1)
	}
	store, err := db.Open("ray-player1")
	if err != nil {
		auditLog.E("%v", err)
		os.Exit(1)
	}
	defer store.Close()
	lib := library.NewService(store, nil, nil, nil)
	if err := lib.Load(); err != nil {
		auditLog.E("%v", err)
		os.Exit(1)
	}
	track, ok := lib.TrackByID(*seedID)
	if !ok {
		fmt.Fprintf(os.Stderr, "track not found: %s\n", *seedID)
		os.Exit(1)
	}
	evt := events.NewService(store, lib)
	rec := recommend.NewService(evt, nil)
	audit := rec.AuditRay(track, lib.AllTracks(), *mode, *limit)
	fmt.Printf("seed=%s mode=%s rows=%d\n", audit.SeedTrackID, audit.Mode, len(audit.Rows))
	for i, row := range audit.Rows {
		transition := ""
		if i > 0 {
			prevLabel := audit.Rows[i-1].EmotionLabel
			prevFamily := audit.Rows[i-1].EmotionFamily
			transition = fmt.Sprintf("%s/%s -> %s/%s", prevLabel, prevFamily, row.EmotionLabel, row.EmotionFamily)
		} else {
			transition = fmt.Sprintf("SEED %s/%s", row.EmotionLabel, row.EmotionFamily)
		}
		fmt.Printf("%02d %-28s sim=%.2f dist=%.2f jump=%.2f bridge=%.2f familyPen=%.2f\n", row.Position, row.Title, row.Insight.Similarity, row.EmotionDistance, row.HardJumpRisk, row.BridgeScore, row.FamilyPenalty)
		fmt.Printf("    %s\n", transition)
		if row.Reason != "" {
			fmt.Printf("    reason=%s\n", row.Reason)
		}
	}
}
