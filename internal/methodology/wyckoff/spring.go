package wyckoff

import (
	"fmt"
	"time"

	structural "github.com/Ju571nK/Chatter/internal/wyckoff"
	"github.com/Ju571nK/Chatter/pkg/models"
)

// WyckoffSpringRule detects a structurally confirmed V2 accumulation trigger on
// 15M first, then falls back to the legacy swing-low Spring heuristic on higher
// timeframes. The V2 path emits a distinct rule name so downstream consumers can
// distinguish it from the legacy detector.
type WyckoffSpringRule struct{}

func (r *WyckoffSpringRule) Name() string                 { return "wyckoff_spring" }
func (r *WyckoffSpringRule) RequiredIndicators() []string { return nil }

func (r *WyckoffSpringRule) Analyze(ctx models.AnalysisContext) (*models.Signal, error) {
	// Prefer the structural V2 analyzer on 15M. A confirmed Spring+Test starts at
	// confidence 0.78; SOS/LPS raises it further. Score is deliberately scaled so
	// structurally confirmed V2 setups can clear the current pipeline score gate
	// without weakening thresholds for unrelated rules.
	if bars15, ok := ctx.Timeframes["15M"]; ok && len(bars15) >= 30 {
		analysis := structural.AnalyzeV2(ctx.Symbol, "15M", bars15)
		if analysis.ReadyForLong && analysis.Confidence >= 0.78 {
			stage := "Spring + Test"
			if analysis.HasSOS {
				stage = "SOS confirmed"
			}

			return &models.Signal{
				Symbol:    ctx.Symbol,
				Timeframe: "15M",
				Rule:      "wyckoff_v2_long",
				Direction: "LONG",
				Score:     analysis.Confidence * 3.0,
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
	}

	const lookback = 5
	const volMultiplier = 1.5

	// Legacy detector remains as a fallback/context signal.
	tfs := []string{"1W", "1D", "4H", "1H"}
	tfW := map[string]float64{"1W": 2.0, "1D": 1.5, "4H": 1.2, "1H": 1.0}

	bestScore := 0.0
	bestTF := ""
	bestSwingLow := 0.0

	for _, tf := range tfs {
		bars, ok := ctx.Timeframes[tf]
		if !ok || len(bars) < lookback {
			continue
		}

		swingLowKey := tf + ":SWING_LOW"
		volMAKey := tf + ":VOLUME_MA_20"

		swingLow, hasSwingLow := ctx.Indicators[swingLowKey]
		volMA, hasVolMA := ctx.Indicators[volMAKey]
		if !hasSwingLow || !hasVolMA {
			continue
		}

		curr := bars[len(bars)-1]
		prev := bars[len(bars)-lookback : len(bars)-1]

		dipped := false
		for _, b := range prev {
			if b.Low < swingLow {
				dipped = true
				break
			}
		}
		if !dipped {
			continue
		}

		if curr.Close <= swingLow {
			continue
		}

		if curr.Volume < volMultiplier*volMA {
			continue
		}

		weighted := tfW[tf]
		if weighted > bestScore {
			bestScore = weighted
			bestTF = tf
			bestSwingLow = swingLow
		}
	}

	if bestTF == "" {
		return nil, nil
	}

	return &models.Signal{
		Symbol:    ctx.Symbol,
		Timeframe: bestTF,
		Rule:      r.Name(),
		Direction: "LONG",
		Score:     1.0,
		Message:   fmt.Sprintf("[%s] Wyckoff Spring pattern -> LONG (swing low: %.4f)", bestTF, bestSwingLow),
		CreatedAt: time.Now(),
	}, nil
}
