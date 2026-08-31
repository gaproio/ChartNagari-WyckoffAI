package wyckoff

import (
	"fmt"
	"math"

	"github.com/Ju571nK/Chatter/pkg/models"
)

// ValidationEvent is one distinct stage transition inside a unique V2 range.
type ValidationEvent struct {
	BarIndex      int     `json:"bar_index"`
	Time          int64   `json:"time"`
	EntryPrice    float64 `json:"entry_price"`
	RangeID       string  `json:"range_id"`
	Stage         string  `json:"stage"`
	Phase         V2Phase `json:"phase"`
	Confidence    float64 `json:"confidence"`
	HTF1H         string  `json:"htf_1h"`
	HTF4H         string  `json:"htf_4h"`
	Return4H      float64 `json:"return_4h_pct"`
	Return8H      float64 `json:"return_8h_pct"`
	Return16H     float64 `json:"return_16h_pct"`
	MaxFav16H     float64 `json:"max_favorable_16h_pct"`
	MaxAdverse16H float64 `json:"max_adverse_16h_pct"`
}

// ValidationBucket aggregates one subset of events.
type ValidationBucket struct {
	Name          string  `json:"name"`
	Triggers      int     `json:"triggers"`
	WinRate4H     float64 `json:"win_rate_4h"`
	WinRate8H     float64 `json:"win_rate_8h"`
	WinRate16H    float64 `json:"win_rate_16h"`
	AvgReturn4H   float64 `json:"avg_return_4h_pct"`
	AvgReturn8H   float64 `json:"avg_return_8h_pct"`
	AvgReturn16H  float64 `json:"avg_return_16h_pct"`
	AvgMFE16H     float64 `json:"avg_mfe_16h_pct"`
	AvgMAE16H     float64 `json:"avg_mae_16h_pct"`
}

// ValidationSummary aggregates historical V2 trigger quality.
type ValidationSummary struct {
	Symbol       string            `json:"symbol"`
	Timeframe    string            `json:"timeframe"`
	Bars         int               `json:"bars"`
	UniqueRanges int               `json:"unique_ranges"`
	Events       []ValidationEvent `json:"events"`
	Overall      ValidationBucket  `json:"overall"`
	ByStage      []ValidationBucket `json:"by_stage"`
	ByHTF        []ValidationBucket `json:"by_htf"`
}

// ValidateV2 replays AnalyzeV2 over historical 15M candles using a rolling
// 200-bar context. Each trading range is identified by its SC+AR timestamps.
// Within one range, Spring+Test, SOS, and LPS are counted at most once each.
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

	seenStage := map[string]bool{}
	seenRange := map[string]bool{}
	for i := window - 1; i+h16 < len(bars); i++ {
		start := i - window + 1
		a := AnalyzeV2(symbol, "15M", bars[start:i+1])
		if !a.ReadyForLong || a.Confidence < 0.78 {
			continue
		}

		rangeID := validationRangeID(a)
		if rangeID == "" {
			continue
		}
		stage := validationStage(a)
		key := rangeID + ":" + stage
		if seenStage[key] {
			continue
		}
		seenStage[key] = true
		seenRange[rangeID] = true

		entry := bars[i].Close
		if entry <= 0 {
			continue
		}
		e := ValidationEvent{
			BarIndex: i, Time: bars[i].OpenTime.Unix(), EntryPrice: entry,
			RangeID: rangeID, Stage: stage, Phase: a.Phase, Confidence: a.Confidence,
			HTF1H: validationTrend(bars[:i+1], 60*60),
			HTF4H: validationTrend(bars[:i+1], 4*60*60),
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

	out.UniqueRanges = len(seenRange)
	out.Overall = summarizeValidation("ALL", out.Events)
	for _, stage := range []string{"SPRING_TEST", "SOS", "LPS"} {
		out.ByStage = append(out.ByStage, summarizeValidation(stage, filterValidation(out.Events, func(e ValidationEvent) bool { return e.Stage == stage })))
	}
	for _, ctx := range []string{"1H+BULL", "1H+BEAR", "4H+BULL", "4H+BEAR", "1H+4H+BULL"} {
		var filtered []ValidationEvent
		switch ctx {
		case "1H+BULL": filtered = filterValidation(out.Events, func(e ValidationEvent) bool { return e.HTF1H == "BULL" })
		case "1H+BEAR": filtered = filterValidation(out.Events, func(e ValidationEvent) bool { return e.HTF1H == "BEAR" })
		case "4H+BULL": filtered = filterValidation(out.Events, func(e ValidationEvent) bool { return e.HTF4H == "BULL" })
		case "4H+BEAR": filtered = filterValidation(out.Events, func(e ValidationEvent) bool { return e.HTF4H == "BEAR" })
		case "1H+4H+BULL": filtered = filterValidation(out.Events, func(e ValidationEvent) bool { return e.HTF1H == "BULL" && e.HTF4H == "BULL" })
		}
		out.ByHTF = append(out.ByHTF, summarizeValidation(ctx, filtered))
	}
	return out
}

func validationRangeID(a V2Analysis) string {
	var sc, ar int64
	for _, e := range a.Events {
		if e.Type == V2EventSC { sc = e.Time }
		if e.Type == V2EventAR { ar = e.Time }
	}
	if sc == 0 || ar == 0 { return "" }
	return fmt.Sprintf("%d-%d", sc, ar)
}

func validationStage(a V2Analysis) string {
	if a.HasLPS { return "LPS" }
	if a.HasSOS { return "SOS" }
	return "SPRING_TEST"
}

// validationTrend aggregates aligned 15M closes into the requested timeframe
// and compares the latest close with EMA50. It is context only, not a trade rule.
func validationTrend(bars []models.OHLCV, bucketSeconds int64) string {
	if len(bars) == 0 { return "NA" }
	closes := make([]float64, 0, len(bars)/4)
	var lastBucket int64 = -1
	for _, b := range bars {
		bucket := b.OpenTime.Unix() / bucketSeconds
		if bucket != lastBucket {
			closes = append(closes, b.Close)
			lastBucket = bucket
		} else {
			closes[len(closes)-1] = b.Close
		}
	}
	if len(closes) < 50 { return "NA" }
	alpha := 2.0 / 51.0
	ema := closes[0]
	for _, c := range closes[1:] { ema = alpha*c + (1-alpha)*ema }
	if closes[len(closes)-1] >= ema { return "BULL" }
	return "BEAR"
}

func summarizeValidation(name string, events []ValidationEvent) ValidationBucket {
	b := ValidationBucket{Name: name, Triggers: len(events)}
	if len(events) == 0 { return b }
	var w4, w8, w16 int
	for _, e := range events {
		b.AvgReturn4H += e.Return4H
		b.AvgReturn8H += e.Return8H
		b.AvgReturn16H += e.Return16H
		b.AvgMFE16H += e.MaxFav16H
		b.AvgMAE16H += e.MaxAdverse16H
		if e.Return4H > 0 { w4++ }
		if e.Return8H > 0 { w8++ }
		if e.Return16H > 0 { w16++ }
	}
	n := float64(len(events))
	b.AvgReturn4H /= n; b.AvgReturn8H /= n; b.AvgReturn16H /= n
	b.AvgMFE16H /= n; b.AvgMAE16H /= n
	b.WinRate4H = float64(w4)/n*100; b.WinRate8H = float64(w8)/n*100; b.WinRate16H = float64(w16)/n*100
	return b
}

func filterValidation(events []ValidationEvent, keep func(ValidationEvent) bool) []ValidationEvent {
	out := make([]ValidationEvent, 0)
	for _, e := range events { if keep(e) { out = append(out, e) } }
	return out
}

func pctReturn(entry, exit float64) float64 {
	if math.Abs(entry) < 1e-12 { return 0 }
	return (exit-entry)/entry*100
}
