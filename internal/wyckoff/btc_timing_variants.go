package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// BTCTimingVariantResult compares causal next-open execution for the frozen B
// condition using different maximum confirmation windows. It is research only;
// the production/frozen BTC rule remains <=8 bars unless later evidence justifies
// a separate frozen change.
type BTCTimingVariantResult struct {
	Name         string  `json:"name"`
	Entries      int     `json:"entries"`
	TargetHits   int     `json:"target_hits"`
	StopHits     int     `json:"stop_hits"`
	TimeExits    int     `json:"time_exits"`
	NetWinRate   float64 `json:"net_win_rate"`
	AvgDelayBars float64 `json:"avg_delay_bars"`
	AvgRiskPct   float64 `json:"avg_risk_pct"`
	AvgGrossR    float64 `json:"avg_gross_r"`
	AvgNetR      float64 `json:"avg_net_r"`
}

type btcTimingTrade struct {
	delay   int
	riskPct float64
	grossR  float64
	netR    float64
	outcome string
}

// ValidateBTCTimingVariants evaluates the existing prospective-HL B condition
// with actual next-bar-open execution. The <=8 bar result is the current frozen
// baseline. <=16 and the incremental 9-16 group are research variants only.
// Detector, stop construction, target, max hold and cost model are unchanged.
func ValidateBTCTimingVariants(input []models.OHLCV, validation V3ValidationSummary, cfg BTCMasterConfig) []BTCTimingVariantResult {
	bars := v2Chronological(input)
	if cfg.TargetR <= 0 { cfg.TargetR = 3 }

	all8 := btcTimingTrades(bars, validation, cfg, 8, 0)
	all16 := btcTimingTrades(bars, validation, cfg, 16, 0)
	lateOnly := btcTimingTrades(bars, validation, cfg, 16, 8)

	return []BTCTimingVariantResult{
		summarizeBTCTiming("B <=8 FROZEN", all8),
		summarizeBTCTiming("B <=16 RESEARCH", all16),
		summarizeBTCTiming("B 9-16 ONLY", lateOnly),
	}
}

// minDecisionBarsExclusive keeps only entries whose decision delay is strictly
// greater than this value. Zero means no lower bound. For B 9-16 ONLY we use 8.
func btcTimingTrades(bars []models.OHLCV, validation V3ValidationSummary, cfg BTCMasterConfig, maxWait, minDecisionBarsExclusive int) []btcTimingTrade {
	trades := make([]btcTimingTrade, 0, len(validation.Events))
	for _, e := range validation.Events {
		if e.BarIndex < 199 || e.SpringATR <= 0 { continue }
		start := e.BarIndex - 199
		a := AnalyzeV3Foundation(validation.Symbol, "15M", bars[start:e.BarIndex+1])
		if !a.HasSpring || !a.HasTest { continue }

		testLocal := -1
		for _, ev := range a.Events {
			if ev.Type == V3EventTest { testLocal = ev.BarIndex; break }
		}
		if testLocal < 0 { continue }
		testGlobal := start + testLocal
		if testGlobal < 0 || testGlobal >= len(bars) { continue }

		midpoint := (a.Range.Support + a.Range.Resistance) / 2
		if midpoint <= 0 { continue }
		decisionIdx := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, maxWait, v4EntryProspectiveHL)
		if decisionIdx < 0 { continue }
		decisionDelay := decisionIdx - e.BarIndex
		if decisionDelay <= minDecisionBarsExclusive { continue }

		execIdx := decisionIdx + 1
		if execIdx >= len(bars) || execIdx+64 >= len(bars) { continue }
		stop := v4VariantStop(bars, testGlobal, decisionIdx, e.SpringLow, e.SpringATR, v4StopPostTest)
		entry := bars[execIdx].Open
		if entry <= 0 || stop <= 0 || stop >= entry { continue }
		risk := entry - stop
		riskPct := risk / entry * 100

		grossR, outcome, _ := simulateBTCMasterOutcome(bars, execIdx, 64, entry, stop, cfg.TargetR)
		costFraction := 2 * (cfg.FeeBpsPerSide + cfg.SlippageBpsPerSide) / 10000.0
		riskFraction := risk / entry
		costR := 0.0
		if riskFraction > 0 { costR = costFraction / riskFraction }

		trades = append(trades, btcTimingTrade{
			delay: execIdx - e.BarIndex,
			riskPct: riskPct,
			grossR: grossR,
			netR: grossR - costR,
			outcome: outcome,
		})
	}
	return trades
}

func summarizeBTCTiming(name string, trades []btcTimingTrade) BTCTimingVariantResult {
	r := BTCTimingVariantResult{Name:name, Entries:len(trades)}
	if len(trades)==0 { return r }
	wins := 0
	for _, t := range trades {
		switch t.outcome {
		case "TARGET": r.TargetHits++
		case "STOP": r.StopHits++
		case "TIME": r.TimeExits++
		}
		if t.netR > 0 { wins++ }
		r.AvgDelayBars += float64(t.delay)
		r.AvgRiskPct += t.riskPct
		r.AvgGrossR += t.grossR
		r.AvgNetR += t.netR
	}
	n := float64(len(trades))
	r.NetWinRate = float64(wins)/n*100
	r.AvgDelayBars /= n
	r.AvgRiskPct /= n
	r.AvgGrossR /= n
	r.AvgNetR /= n
	return r
}
