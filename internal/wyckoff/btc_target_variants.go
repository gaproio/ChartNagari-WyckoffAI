package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// BTCTargetVariantResult compares fixed reward targets for the frozen BTC 15M
// execution model. Detector, <=8-bar B confirmation, next-open entry, post-Test
// stop, 16-hour max hold and transaction costs remain unchanged.
type BTCTargetVariantResult struct {
	Name       string  `json:"name"`
	TargetR    float64 `json:"target_r"`
	Entries    int     `json:"entries"`
	TargetHits int     `json:"target_hits"`
	StopHits   int     `json:"stop_hits"`
	TimeExits  int     `json:"time_exits"`
	NetWinRate float64 `json:"net_win_rate"`
	AvgGrossR  float64 `json:"avg_gross_r"`
	AvgNetR    float64 `json:"avg_net_r"`
}

type btcTargetTrade struct {
	grossR  float64
	netR    float64
	outcome string
}

// ValidateBTCTargetVariants measures a small predeclared target set. This is a
// research comparison, not an optimizer; the frozen 3R baseline is unchanged.
func ValidateBTCTargetVariants(input []models.OHLCV, validation V3ValidationSummary, cfg BTCMasterConfig) []BTCTargetVariantResult {
	bars := v2Chronological(input)
	targets := []struct {
		name string
		r    float64
	}{
		{"1R", 1},
		{"2R", 2},
		{"3R FROZEN", 3},
		{"4R", 4},
	}

	out := make([]BTCTargetVariantResult, 0, len(targets))
	for _, target := range targets {
		trades := btcTargetTrades(bars, validation, cfg, target.r, 64)
		out = append(out, summarizeBTCTarget(target.name, target.r, trades))
	}
	return out
}

func btcTargetTrades(bars []models.OHLCV, validation V3ValidationSummary, cfg BTCMasterConfig, targetR float64, maxHold int) []btcTargetTrade {
	trades := make([]btcTargetTrade, 0, len(validation.Events))
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

		grossR, outcome, _ := simulateBTCMasterOutcome(bars, execIdx, maxHold, entry, stop, targetR)
		costFraction := 2 * (cfg.FeeBpsPerSide + cfg.SlippageBpsPerSide) / 10000.0
		riskFraction := risk / entry
		costR := 0.0
		if riskFraction > 0 { costR = costFraction / riskFraction }
		trades = append(trades, btcTargetTrade{grossR:grossR, netR:grossR-costR, outcome:outcome})
	}
	return trades
}

func summarizeBTCTarget(name string, targetR float64, trades []btcTargetTrade) BTCTargetVariantResult {
	r := BTCTargetVariantResult{Name:name, TargetR:targetR, Entries:len(trades)}
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
