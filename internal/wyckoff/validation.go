package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// ValidationEvent is one distinct transition into a V2 long-ready state and
// its subsequent forward returns. A transition is counted once, rather than on
// every candle while the same setup remains active.
type ValidationEvent struct {
	BarIndex    int     `json:"bar_index"`
	Time        int64   `json:"time"`
	EntryPrice  float64 `json:"entry_price"`
	Phase       V2Phase `json:"phase"`
	Confidence  float64 `json:"confidence"`
	HasSpring   bool    `json:"has_spring"`
	HasTest     bool    `json:"has_test"`
	HasSOS      bool    `json:"has_sos"`
	Return4H    float64 `json:"return_4h_pct"`
	Return8H    float64 `json:"return_8h_pct"`
	Return16H   float64 `json:"return_16h_pct"`
	MaxFav16H   float64 `json:"max_favorable_16h_pct"`
	MaxAdverse16H float64 `json:"max_adverse_16h_pct"`
}

// ValidationSummary aggregates historical V2 trigger quality.
type ValidationSummary struct {
	Symbol       string            `json:"symbol"`
	Timeframe    string            `json:"timeframe"`
	Bars         int               `json:"bars"`
	Events       []ValidationEvent `json:"events"`
	Triggers     int               `json:"triggers"`
	WinRate4H    float64           `json:"win_rate_4h"`
	WinRate8H    float64           `json:"win_rate_8h"`
	WinRate16H   float64           `json:"win_rate_16h"`
	AvgReturn4H  float64           `json:"avg_return_4h_pct"`
	AvgReturn8H  float64           `json:"avg_return_8h_pct"`
	AvgReturn16H float64           `json:"avg_return_16h_pct"`
	AvgMFE16H    float64           `json:"avg_mfe_16h_pct"`
	AvgMAE16H    float64           `json:"avg_mae_16h_pct"`
}

// ValidateV2 replays AnalyzeV2 over historical 15M candles. It uses a rolling
// 200-bar context and records only false->true ReadyForLong transitions to
// avoid counting one persistent Wyckoff structure as many separate signals.
func ValidateV2(symbol string, input []models.OHLCV) ValidationSummary {
	bars := v2Chronological(input)
	out := ValidationSummary{Symbol: symbol, Timeframe: "15M", Bars: len(bars)}
	const window = 200
	const h4 = 16
	const h8 = 32
	const h16 = 64
	if len(bars) < window+h16+1 {
		return out
	}

	prevReady := false
	for i := window - 1; i+h16 < len(bars); i++ {
		start := i - window + 1
		a := AnalyzeV2(symbol, "15M", bars[start:i+1])
		ready := a.ReadyForLong && a.Confidence >= 0.78
		if ready && !prevReady {
			entry := bars[i].Close
			if entry > 0 {
				e := ValidationEvent{
					BarIndex: i, Time: bars[i].OpenTime.Unix(), EntryPrice: entry,
					Phase: a.Phase, Confidence: a.Confidence,
					HasSpring: a.HasSpring, HasTest: a.HasTest, HasSOS: a.HasSOS,
					Return4H: pctReturn(entry, bars[i+h4].Close),
					Return8H: pctReturn(entry, bars[i+h8].Close),
					Return16H: pctReturn(entry, bars[i+h16].Close),
				}
				maxHigh, minLow := entry, entry
				for j := i + 1; j <= i+h16; j++ {
					if bars[j].High > maxHigh { maxHigh = bars[j].High }
					if bars[j].Low < minLow { minLow = bars[j].Low }
				}
				e.MaxFav16H = pctReturn(entry, maxHigh)
				e.MaxAdverse16H = pctReturn(entry, minLow)
				out.Events = append(out.Events, e)
			}
		}
		prevReady = ready
	}

	out.Triggers = len(out.Events)
	if out.Triggers == 0 { return out }
	var wins4, wins8, wins16 int
	for _, e := range out.Events {
		out.AvgReturn4H += e.Return4H
		out.AvgReturn8H += e.Return8H
		out.AvgReturn16H += e.Return16H
		out.AvgMFE16H += e.MaxFav16H
		out.AvgMAE16H += e.MaxAdverse16H
		if e.Return4H > 0 { wins4++ }
		if e.Return8H > 0 { wins8++ }
		if e.Return16H > 0 { wins16++ }
	}
	n := float64(out.Triggers)
	out.AvgReturn4H /= n
	out.AvgReturn8H /= n
	out.AvgReturn16H /= n
	out.AvgMFE16H /= n
	out.AvgMAE16H /= n
	out.WinRate4H = float64(wins4) / n * 100
	out.WinRate8H = float64(wins8) / n * 100
	out.WinRate16H = float64(wins16) / n * 100
	return out
}

func pctReturn(entry, exit float64) float64 {
	if entry == 0 { return 0 }
	return (exit-entry)/entry*100
}
