package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// V4VariantResult measures a causal V4 entry/stop combination without changing
// V3 detection. All entry rules use only information available at the entry
// candle and all studies use the same eight-candle post-signal window.
type V4VariantResult struct {
	Name        string  `json:"name"`
	V3Signals   int     `json:"v3_signals"`
	Entries     int     `json:"entries"`
	EntryRate   float64 `json:"entry_rate"`
	AvgDelay    float64 `json:"avg_delay_bars"`
	AvgRiskPct  float64 `json:"avg_risk_pct"`
	WinRate16H  float64 `json:"win_rate_16h"`
	AvgReturn16H float64 `json:"avg_return_16h_pct"`
	R1WinRate   float64 `json:"r1_win_rate"`
	R2WinRate   float64 `json:"r2_win_rate"`
	R3WinRate   float64 `json:"r3_win_rate"`
	AvgR1       float64 `json:"avg_r1"`
	AvgR2       float64 `json:"avg_r2"`
	AvgR3       float64 `json:"avg_r3"`
}

type V4VariantReport struct {
	Variants []V4VariantResult `json:"variants"`
}

type v4VariantTrade struct {
	delay    int
	riskPct  float64
	ret16    float64
	r1       float64
	r2       float64
	r3       float64
}

type v4EntryRule int

const (
	v4EntryMidpoint v4EntryRule = iota
	v4EntryProspectiveHL
	v4EntryAboveSignalHigh
)

type v4StopRule int

const (
	v4StopSpring v4StopRule = iota
	v4StopPostTest
)

// ValidateV4EntryVariants compares three causal entry styles with two structural
// stop styles. The detector is unchanged and no parameter is optimized per coin.
//
// Entry A: first close above the range midpoint.
// Entry B: midpoint reclaim while price has remained above the Test low and the
//          entry candle turns upward (bullish close and close > previous close).
//          This is a prospective higher-low condition; it does not wait for a
//          fully confirmed pivot.
// Entry C: first close above both the midpoint and the V3 signal-candle high.
//
// Stop SPRING: Spring low - 0.75 ATR (existing structural benchmark).
// Stop POSTTEST: lowest low from the Test through entry - 0.25 ATR.
func ValidateV4EntryVariants(input []models.OHLCV, validation V3ValidationSummary) V4VariantReport {
	bars := v2Chronological(input)
	configs := []struct {
		name  string
		entry v4EntryRule
		stop  v4StopRule
	}{
		{"A MID + SPRING", v4EntryMidpoint, v4StopSpring},
		{"A MID + POSTTEST", v4EntryMidpoint, v4StopPostTest},
		{"B MID+PROSPECTIVE_HL + SPRING", v4EntryProspectiveHL, v4StopSpring},
		{"B MID+PROSPECTIVE_HL + POSTTEST", v4EntryProspectiveHL, v4StopPostTest},
		{"C MID+ABOVE + SPRING", v4EntryAboveSignalHigh, v4StopSpring},
		{"C MID+ABOVE + POSTTEST", v4EntryAboveSignalHigh, v4StopPostTest},
	}

	report := V4VariantReport{Variants: make([]V4VariantResult, 0, len(configs))}
	for _, cfg := range configs {
		trades := make([]v4VariantTrade, 0, len(validation.Events))
		for _, e := range validation.Events {
			if e.BarIndex < 199 || e.BarIndex+72 >= len(bars) || e.SpringATR <= 0 {
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
			entryIdx := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, 8, cfg.entry)
			if entryIdx < 0 || entryIdx+64 >= len(bars) {
				continue
			}
			entry := bars[entryIdx].Close
			if entry <= 0 {
				continue
			}

			stop := v4VariantStop(bars, testGlobal, entryIdx, e.SpringLow, e.SpringATR, cfg.stop)
			if stop <= 0 || stop >= entry {
				continue
			}
			trades = append(trades, v4VariantTrade{
				delay:   entryIdx - e.BarIndex,
				riskPct: (entry - stop) / entry * 100,
				ret16:   pctReturn(entry, bars[entryIdx+64].Close),
				r1:      simulateRTrade(bars, entryIdx, 64, entry, stop, 1),
				r2:      simulateRTrade(bars, entryIdx, 64, entry, stop, 2),
				r3:      simulateRTrade(bars, entryIdx, 64, entry, stop, 3),
			})
		}
		report.Variants = append(report.Variants, summarizeV4Variant(cfg.name, len(validation.Events), trades))
	}
	return report
}

func v4VariantEntry(bars []models.OHLCV, signalIndex, testIndex int, midpoint float64, maxWait int, rule v4EntryRule) int {
	if signalIndex < 0 || signalIndex >= len(bars) || testIndex < 0 || testIndex >= len(bars) || midpoint <= 0 {
		return -1
	}
	end := signalIndex + maxWait
	if end >= len(bars) {
		end = len(bars) - 1
	}
	testLow := bars[testIndex].Low
	signalHigh := bars[signalIndex].High
	heldTestLow := true

	for i := signalIndex + 1; i <= end; i++ {
		b := bars[i]
		if b.Low <= testLow {
			heldTestLow = false
		}
		switch rule {
		case v4EntryMidpoint:
			if b.Close > midpoint {
				return i
			}
		case v4EntryProspectiveHL:
			turningUp := i > signalIndex && b.Close > b.Open && b.Close > bars[i-1].Close
			if heldTestLow && turningUp && b.Close > midpoint {
				return i
			}
		case v4EntryAboveSignalHigh:
			if b.Close > midpoint && b.Close > signalHigh {
				return i
			}
		}
	}
	return -1
}

func v4VariantStop(bars []models.OHLCV, testIndex, entryIndex int, springLow, springATR float64, rule v4StopRule) float64 {
	if springATR <= 0 {
		return 0
	}
	if rule == v4StopSpring {
		return springLow - 0.75*springATR
	}
	if testIndex < 0 || entryIndex < testIndex || entryIndex >= len(bars) {
		return 0
	}
	low := bars[testIndex].Low
	for i := testIndex + 1; i <= entryIndex; i++ {
		if bars[i].Low < low {
			low = bars[i].Low
		}
	}
	return low - 0.25*springATR
}

func summarizeV4Variant(name string, v3Signals int, trades []v4VariantTrade) V4VariantResult {
	r := V4VariantResult{Name: name, V3Signals: v3Signals, Entries: len(trades)}
	if v3Signals > 0 {
		r.EntryRate = float64(r.Entries) / float64(v3Signals) * 100
	}
	if len(trades) == 0 {
		return r
	}
	var w16, w1, w2, w3 int
	for _, t := range trades {
		r.AvgDelay += float64(t.delay)
		r.AvgRiskPct += t.riskPct
		r.AvgReturn16H += t.ret16
		r.AvgR1 += t.r1
		r.AvgR2 += t.r2
		r.AvgR3 += t.r3
		if t.ret16 > 0 { w16++ }
		if t.r1 > 0 { w1++ }
		if t.r2 > 0 { w2++ }
		if t.r3 > 0 { w3++ }
	}
	n := float64(len(trades))
	r.AvgDelay /= n
	r.AvgRiskPct /= n
	r.AvgReturn16H /= n
	r.AvgR1 /= n
	r.AvgR2 /= n
	r.AvgR3 /= n
	r.WinRate16H = float64(w16) / n * 100
	r.R1WinRate = float64(w1) / n * 100
	r.R2WinRate = float64(w2) / n * 100
	r.R3WinRate = float64(w3) / n * 100
	return r
}
