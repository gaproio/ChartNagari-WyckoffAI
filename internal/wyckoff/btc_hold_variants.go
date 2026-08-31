package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// BTCHoldVariantResult compares different maximum holding horizons for the
// frozen BTC 15M B entry. This is research only: detector, <=8-bar confirmation,
// next-open entry, post-Test stop, 3R target and cost model stay unchanged.
type BTCHoldVariantResult struct {
	Name       string  `json:"name"`
	HoldBars   int     `json:"hold_bars"`
	Entries    int     `json:"entries"`
	TargetHits int     `json:"target_hits"`
	StopHits   int     `json:"stop_hits"`
	TimeExits  int     `json:"time_exits"`
	NetWinRate float64 `json:"net_win_rate"`
	AvgGrossR  float64 `json:"avg_gross_r"`
	AvgNetR    float64 `json:"avg_net_r"`
}

type btcHoldTrade struct {
	grossR  float64
	netR    float64
	outcome string
}

// ValidateBTCHoldVariants measures fixed, predeclared 15M holding horizons.
// None of these results changes the frozen 16-hour baseline automatically.
func ValidateBTCHoldVariants(input []models.OHLCV, validation V3ValidationSummary, cfg BTCMasterConfig) []BTCHoldVariantResult {
	bars := v2Chronological(input)
	if cfg.TargetR <= 0 { cfg.TargetR = 3 }

	horizons := []struct {
		name string
		bars int
	}{
		{"4H / 16 bars", 16},
		{"8H / 32 bars", 32},
		{"16H / 64 bars FROZEN", 64},
		{"24H / 96 bars", 96},
		{"32H / 128 bars", 128},
	}

	out := make([]BTCHoldVariantResult, 0, len(horizons))
	for _, h := range horizons {
		trades := btcHoldTrades(bars, validation, cfg, h.bars)
		out = append(out, summarizeBTCHold(h.name, h.bars, trades))
	}
	return out
}

func btcHoldTrades(bars []models.OHLCV, validation V3ValidationSummary, cfg BTCMasterConfig, maxHold int) []btcHoldTrade {
	trades := make([]btcHoldTrade, 0, len(validation.Events))
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
		decisionIdx := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, 8, v4EntryProspectiveHL)
		if decisionIdx < 0 { continue }
		execIdx := decisionIdx + 1
		if execIdx >= len(bars) || execIdx+maxHold >= len(bars) { continue }

		stop := v4VariantStop(bars, testGlobal, decisionIdx, e.SpringLow, e.SpringATR, v4StopPostTest)
		entry := bars[execIdx].Open
		if entry <= 0 || stop <= 0 || stop >= entry { continue }
		risk := entry - stop

		grossR, outcome, _ := simulateBTCMasterOutcome(bars, execIdx, maxHold, entry, stop, cfg.TargetR)
		costFraction := 2 * (cfg.FeeBpsPerSide + cfg.SlippageBpsPerSide) / 10000.0
		riskFraction := risk / entry
		costR := 0.0
		if riskFraction > 0 { costR = costFraction / riskFraction }

		trades = append(trades, btcHoldTrade{grossR:grossR, netR:grossR-costR, outcome:outcome})
	}
	return trades
}

func summarizeBTCHold(name string, holdBars int, trades []btcHoldTrade) BTCHoldVariantResult {
	r := BTCHoldVariantResult{Name:name, HoldBars:holdBars, Entries:len(trades)}
	if len(trades)==0 { return r }
	wins := 0
	for _, t := range trades {
		switch t.outcome {
		case "TARGET": r.TargetHits++
		case "STOP": r.StopHits++
		case "TIME": r.TimeExits++
		}
		if t.netR > 0 { wins++ }
		r.AvgGrossR += t.grossR
		r.AvgNetR += t.netR
	}
	n := float64(len(trades))
	r.NetWinRate = float64(wins)/n*100
	r.AvgGrossR /= n
	r.AvgNetR /= n
	return r
}
