package wyckoff

import (
	"fmt"
	"time"

	structural "github.com/Ju571nK/Chatter/internal/wyckoff"
	"github.com/Ju571nK/Chatter/pkg/models"
)

// WyckoffV2LongRule emits a conservative LONG signal from the structural V2
// accumulation analyzer. 15M is the trigger timeframe; higher timeframes remain
// available to the surrounding pipeline for context and risk filters.
type WyckoffV2LongRule struct{}

func (r *WyckoffV2LongRule) Name() string                 { return "wyckoff_v2_long" }
func (r *WyckoffV2LongRule) RequiredIndicators() []string { return nil }

func (r *WyckoffV2LongRule) Analyze(ctx models.AnalysisContext) (*models.Signal, error) {
	bars, ok := ctx.Timeframes["15M"]
	if !ok || len(bars) < 30 {
		return nil, nil
	}

	analysis := structural.AnalyzeV2(ctx.Symbol, "15M", bars)
	if !analysis.ReadyForLong || analysis.Confidence < 0.78 {
		return nil, nil
	}

	stage := "Spring + Test"
	if analysis.HasSOS {
		stage = "SOS confirmed"
	}

	return &models.Signal{
		Symbol:    ctx.Symbol,
		Timeframe: "15M",
		Rule:      r.Name(),
		Direction: "LONG",
		Score:     analysis.Confidence,
		Message: fmt.Sprintf(
			"[15M] Wyckoff V2 %s | Phase %s | confidence %.0f%% | range %.4f–%.4f",
			stage,
			analysis.Phase,
			analysis.Confidence*100,
			analysis.Range.Support,
			analysis.Range.Resistance,
		),
		CreatedAt: time.Now(),
	}, nil
}
