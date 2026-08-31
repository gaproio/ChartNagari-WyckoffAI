package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// V4CausalTrade is the first fully causal V4 study entry for a V3 setup.
// After the V3 Test, V4 waits up to eight 15M candles for both:
//   1. a confirmed higher-low pivot above the Test low, and
//   2. a close above the trading-range midpoint.
// Entry is the close of the candle where the second requirement becomes known.
type V4CausalTrade struct {
	SignalBar      int     `json:"signal_bar"`
	EntryBar       int     `json:"entry_bar"`
	EntryPrice     float64 `json:"entry_price"`
	EntryDelayBars int     `json:"entry_delay_bars"`
	Midpoint       float64 `json:"midpoint"`
	HigherLowPrice float64 `json:"higher_low_price"`
	StopPrice      float64 `json:"stop_price"`
	RiskPct        float64 `json:"risk_pct"`
	Return16H      float64 `json:"return_16h_pct"`
	R1             float64 `json:"r1_result"`
	R2             float64 `json:"r2_result"`
	R3             float64 `json:"r3_result"`
}

type V4CausalSummary struct {
	V3Signals        int             `json:"v3_signals"`
	V4Entries        int             `json:"v4_entries"`
	EntryRate        float64         `json:"entry_rate"`
	AvgDelayBars     float64         `json:"avg_delay_bars"`
	AvgRiskPct       float64         `json:"avg_risk_pct"`
	WinRate16H       float64         `json:"win_rate_16h"`
	AvgReturn16H     float64         `json:"avg_return_16h_pct"`
	R1WinRate        float64         `json:"r1_win_rate"`
	R2WinRate        float64         `json:"r2_win_rate"`
	R3WinRate        float64         `json:"r3_win_rate"`
	AvgR1            float64         `json:"avg_r1"`
	AvgR2            float64         `json:"avg_r2"`
	AvgR3            float64         `json:"avg_r3"`
	Trades           []V4CausalTrade `json:"trades"`
}

// ValidateV4Causal evaluates the selected V4 research rule without looking
// beyond the candle that creates the entry. V3 detection remains unchanged.
func ValidateV4Causal(input []models.OHLCV, validation V3ValidationSummary) V4CausalSummary {
	bars := v2Chronological(input)
	out := V4CausalSummary{V3Signals: len(validation.Events)}

	for _, e := range validation.Events {
		if e.BarIndex < 199 || e.BarIndex+72 >= len(bars) || e.SpringATR <= 0 { continue }
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

		mid := (a.Range.Support + a.Range.Resistance) / 2
		if mid <= 0 { continue }
		entryIdx, hlPrice := v4CausalEntry(bars, e.BarIndex, testGlobal, mid, 8)
		if entryIdx < 0 { continue }
		entry := bars[entryIdx].Close
		stop := e.SpringLow - 0.75*e.SpringATR
		if entry <= 0 || stop <= 0 || stop >= entry { continue }

		end := entryIdx + 64
		if end >= len(bars) { continue }
		risk := entry - stop
		trade := V4CausalTrade{
			SignalBar: e.BarIndex, EntryBar: entryIdx, EntryPrice: entry,
			EntryDelayBars: entryIdx-e.BarIndex, Midpoint: mid, HigherLowPrice: hlPrice,
			StopPrice: stop, RiskPct: risk/entry*100,
			Return16H: pctReturn(entry, bars[end].Close),
			R1: simulateRTrade(bars,entryIdx,64,entry,stop,1),
			R2: simulateRTrade(bars,entryIdx,64,entry,stop,2),
			R3: simulateRTrade(bars,entryIdx,64,entry,stop,3),
		}
		out.Trades = append(out.Trades, trade)
	}

	out.V4Entries = len(out.Trades)
	if out.V3Signals > 0 { out.EntryRate = float64(out.V4Entries)/float64(out.V3Signals)*100 }
	if out.V4Entries == 0 { return out }

	var w16,w1,w2,w3 int
	for _, t := range out.Trades {
		out.AvgDelayBars += float64(t.EntryDelayBars)
		out.AvgRiskPct += t.RiskPct
		out.AvgReturn16H += t.Return16H
		out.AvgR1 += t.R1; out.AvgR2 += t.R2; out.AvgR3 += t.R3
		if t.Return16H > 0 { w16++ }
		if t.R1 > 0 { w1++ }; if t.R2 > 0 { w2++ }; if t.R3 > 0 { w3++ }
	}
	n := float64(out.V4Entries)
	out.AvgDelayBars /= n; out.AvgRiskPct /= n; out.AvgReturn16H /= n
	out.AvgR1 /= n; out.AvgR2 /= n; out.AvgR3 /= n
	out.WinRate16H = float64(w16)/n*100
	out.R1WinRate = float64(w1)/n*100; out.R2WinRate = float64(w2)/n*100; out.R3WinRate = float64(w3)/n*100
	return out
}

// v4CausalEntry is intentionally strict about causality. A higher low becomes
// knowable only after the following candle confirms a local pivot. Midpoint
// reclaim can happen before or after that pivot. We enter when both facts are
// known, never retroactively at the pivot candle.
func v4CausalEntry(bars []models.OHLCV, signalIndex, testIndex int, midpoint float64, maxWait int) (int, float64) {
	if signalIndex < 0 || signalIndex >= len(bars) || testIndex < 0 || testIndex >= len(bars) || midpoint <= 0 { return -1,0 }
	end := signalIndex + maxWait
	if end >= len(bars) { end = len(bars)-1 }
	testLow := bars[testIndex].Low
	midReclaimed := false
	higherLow := false
	hlPrice := 0.0

	for i := signalIndex+1; i <= end; i++ {
		if bars[i].Close > midpoint { midReclaimed = true }
		// Confirm a pivot at i-1 using only i and earlier candles. The pivot must
		// remain above the Test low to qualify as a Wyckoff higher low.
		if i >= signalIndex+2 {
			p := i-1
			if bars[p].Low > testLow && bars[p].Low < bars[p-1].Low && bars[p].Low <= bars[i].Low {
				higherLow = true
				hlPrice = bars[p].Low
			}
		}
		if midReclaimed && higherLow { return i,hlPrice }
	}
	return -1,0
}
