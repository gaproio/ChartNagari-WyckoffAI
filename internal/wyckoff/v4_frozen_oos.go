package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// ValidateV4FrozenFinalists evaluates the two frozen V4 finalists with a more
// execution-realistic assumption: the rule is decided on a closed 15M candle,
// and the trade enters at the NEXT candle open. Both finalists use the causal
// post-Test structural stop known before that next candle opens.
//
// No detector thresholds or rule parameters are changed here.
func ValidateV4FrozenFinalists(input []models.OHLCV, validation V3ValidationSummary) V4VariantReport {
	bars := v2Chronological(input)
	configs := []struct {
		name  string
		entry v4EntryRule
	}{
		{"A MID + POSTTEST NEXT-OPEN", v4EntryMidpoint},
		{"B MID+PROSPECTIVE_HL + POSTTEST NEXT-OPEN", v4EntryProspectiveHL},
	}

	report := V4VariantReport{Variants: make([]V4VariantResult, 0, len(configs))}
	for _, cfg := range configs {
		trades := make([]v4VariantTrade, 0, len(validation.Events))
		for _, e := range validation.Events {
			if e.BarIndex < 199 || e.SpringATR <= 0 {
				continue
			}
			start := e.BarIndex - 199
			a := AnalyzeV3Foundation(validation.Symbol, "15M", bars[start:e.BarIndex+1])
			if !a.HasSpring || !a.HasTest {
				continue
			}

			testLocal := -1
			for _, ev := range a.Events {
				if ev.Type == V3EventTest {
					testLocal = ev.BarIndex
					break
				}
			}
			if testLocal < 0 {
				continue
			}
			testGlobal := start + testLocal
			if testGlobal < 0 || testGlobal >= len(bars) {
				continue
			}

			midpoint := (a.Range.Support + a.Range.Resistance) / 2
			if midpoint <= 0 {
				continue
			}

			decisionIdx := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, 8, cfg.entry)
			if decisionIdx < 0 {
				continue
			}
			execIdx := decisionIdx + 1
			if execIdx >= len(bars) || execIdx+64 >= len(bars) {
				continue
			}

			// The stop must be fully known at decision-candle close, so do not use
			// any low from the execution candle itself.
			stop := v4VariantStop(bars, testGlobal, decisionIdx, e.SpringLow, e.SpringATR, v4StopPostTest)
			entry := bars[execIdx].Open
			if entry <= 0 || stop <= 0 || stop >= entry {
				continue
			}

			trades = append(trades, v4VariantTrade{
				delay:   execIdx - e.BarIndex,
				riskPct: (entry - stop) / entry * 100,
				ret16:   pctReturn(entry, bars[execIdx+64].Close),
				r1:      simulateRTradeFromOpen(bars, execIdx, 64, entry, stop, 1),
				r2:      simulateRTradeFromOpen(bars, execIdx, 64, entry, stop, 2),
				r3:      simulateRTradeFromOpen(bars, execIdx, 64, entry, stop, 3),
			})
		}
		report.Variants = append(report.Variants, summarizeV4Variant(cfg.name, len(validation.Events), trades))
	}
	return report
}

// simulateRTradeFromOpen is used when the position is entered at a candle's
// open. That same candle can therefore hit the stop or target. If both occur in
// one OHLC candle, the stop wins conservatively because intrabar ordering is
// unknown.
func simulateRTradeFromOpen(bars []models.OHLCV, entryIndex, maxBars int, entry, stop, targetR float64) float64 {
	risk := entry - stop
	if risk <= 0 || entryIndex < 0 || entryIndex >= len(bars) {
		return 0
	}
	target := entry + targetR*risk
	end := entryIndex + maxBars
	if end >= len(bars) {
		end = len(bars)-1
	}
	for j := entryIndex; j <= end; j++ {
		stopHit := bars[j].Low <= stop
		targetHit := bars[j].High >= target
		if stopHit {
			return -1
		}
		if targetHit {
			return targetR
		}
	}
	return (bars[end].Close-entry)/risk
}
