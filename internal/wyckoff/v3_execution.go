package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// V3ExecutionSummary compares alternative execution rules without changing the
// V3 structure detector itself.
type V3ExecutionSummary struct {
	Name       string  `json:"name"`
	Trades     int     `json:"trades"`
	AvgRiskPct float64 `json:"avg_risk_pct"`
	R1WinRate  float64 `json:"r1_win_rate"`
	R2WinRate  float64 `json:"r2_win_rate"`
	R3WinRate  float64 `json:"r3_win_rate"`
	AvgR1      float64 `json:"avg_r1"`
	AvgR2      float64 `json:"avg_r2"`
	AvgR3      float64 `json:"avg_r3"`
}

type V3ExecutionReport struct {
	Overall []V3ExecutionSummary `json:"overall"`
	Score65 []V3ExecutionSummary `json:"score_65"`
}

type v3ExecTrade struct {
	riskPct float64
	r1      float64
	r2      float64
	r3      float64
}

// ValidateV3Execution reuses the exact qualifying ranges from ValidateV3 and
// tests execution alternatives. The signal detector and TradeScore remain
// untouched, so changes in expectancy come only from entry/stop placement.
func ValidateV3Execution(input []models.OHLCV, validation V3ValidationSummary) V3ExecutionReport {
	bars := v2Chronological(input)
	return V3ExecutionReport{
		Overall: v3ExecutionSet(bars, validation.Events),
		Score65: v3ExecutionSet(bars, filterV3Validation(validation.Events, func(e V3ValidationEvent) bool { return e.TradeScore >= 0.65 })),
	}
}

func v3ExecutionSet(bars []models.OHLCV, events []V3ValidationEvent) []V3ExecutionSummary {
	variants := []struct {
		name      string
		atrBuffer float64
		confirm   bool
	}{
		{"TEST + 0.25ATR", 0.25, false},
		{"TEST + 0.75ATR", 0.75, false},
		{"TEST + 1.00ATR", 1.00, false},
		{"CONFIRM + 0.75ATR", 0.75, true},
	}
	out := make([]V3ExecutionSummary, 0, len(variants))
	for _, v := range variants {
		trades := make([]v3ExecTrade, 0, len(events))
		for _, e := range events {
			entryIndex := e.BarIndex
			entry := e.EntryPrice
			if entryIndex < 0 || entryIndex >= len(bars) || entry <= 0 || e.SpringATR <= 0 { continue }
			if v.confirm {
				idx, px := v3ConfirmationEntry(bars, entryIndex, 8)
				if idx < 0 { continue }
				entryIndex, entry = idx, px
			}
			stop := e.SpringLow - v.atrBuffer*e.SpringATR
			if stop <= 0 || stop >= entry { continue }
			riskPct := (entry-stop)/entry*100
			trades = append(trades, v3ExecTrade{
				riskPct:riskPct,
				r1:simulateRTrade(bars, entryIndex, 64, entry, stop, 1),
				r2:simulateRTrade(bars, entryIndex, 64, entry, stop, 2),
				r3:simulateRTrade(bars, entryIndex, 64, entry, stop, 3),
			})
		}
		out = append(out, summarizeV3Execution(v.name, trades))
	}
	return out
}

// Confirmation waits up to eight 15M candles for a close above the signal
// candle high. Entry occurs at that confirmation close, not retroactively.
func v3ConfirmationEntry(bars []models.OHLCV, signalIndex, maxWait int) (int, float64) {
	if signalIndex < 0 || signalIndex >= len(bars) { return -1, 0 }
	trigger := bars[signalIndex].High
	end := signalIndex + maxWait
	if end >= len(bars) { end = len(bars)-1 }
	for i := signalIndex+1; i <= end; i++ {
		if bars[i].Close > trigger { return i, bars[i].Close }
	}
	return -1, 0
}

func summarizeV3Execution(name string, trades []v3ExecTrade) V3ExecutionSummary {
	s := V3ExecutionSummary{Name:name, Trades:len(trades)}
	if len(trades)==0 { return s }
	var w1,w2,w3 int
	for _,t := range trades {
		s.AvgRiskPct += t.riskPct; s.AvgR1 += t.r1; s.AvgR2 += t.r2; s.AvgR3 += t.r3
		if t.r1>0 { w1++ }; if t.r2>0 { w2++ }; if t.r3>0 { w3++ }
	}
	n:=float64(len(trades))
	s.AvgRiskPct/=n; s.AvgR1/=n; s.AvgR2/=n; s.AvgR3/=n
	s.R1WinRate=float64(w1)/n*100; s.R2WinRate=float64(w2)/n*100; s.R3WinRate=float64(w3)/n*100
	return s
}
