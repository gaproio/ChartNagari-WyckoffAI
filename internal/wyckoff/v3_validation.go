package wyckoff

import (
	"fmt"

	"github.com/Ju571nK/Chatter/pkg/models"
)

// V3ValidationEvent records the first qualifying Spring+Test study signal for a
// unique V3 accumulation range. Entry is the close of the candle on which the
// signal first becomes knowable during the rolling replay.
type V3ValidationEvent struct {
	BarIndex      int     `json:"bar_index"`
	Time          int64   `json:"time"`
	EntryPrice    float64 `json:"entry_price"`
	RangeID       string  `json:"range_id"`
	TradeScore    float64 `json:"trade_score"`
	SpringQuality float64 `json:"spring_quality"`
	TestQuality   float64 `json:"test_quality"`
	StructureConf float64 `json:"structure_confidence"`
	Return4H      float64 `json:"return_4h_pct"`
	Return8H      float64 `json:"return_8h_pct"`
	Return16H     float64 `json:"return_16h_pct"`
	MaxFav16H     float64 `json:"max_favorable_16h_pct"`
	MaxAdverse16H float64 `json:"max_adverse_16h_pct"`
}

type V3ValidationBucket struct {
	Name         string  `json:"name"`
	Triggers     int     `json:"triggers"`
	WinRate4H    float64 `json:"win_rate_4h"`
	WinRate8H    float64 `json:"win_rate_8h"`
	WinRate16H   float64 `json:"win_rate_16h"`
	AvgReturn4H  float64 `json:"avg_return_4h_pct"`
	AvgReturn8H  float64 `json:"avg_return_8h_pct"`
	AvgReturn16H float64 `json:"avg_return_16h_pct"`
	AvgMFE16H    float64 `json:"avg_mfe_16h_pct"`
	AvgMAE16H    float64 `json:"avg_mae_16h_pct"`
	AvgScore     float64 `json:"avg_trade_score"`
}

type V3ValidationSummary struct {
	Symbol       string               `json:"symbol"`
	Timeframe    string               `json:"timeframe"`
	Bars         int                  `json:"bars"`
	UniqueRanges int                  `json:"unique_ranges"`
	Events       []V3ValidationEvent  `json:"events"`
	Overall      V3ValidationBucket   `json:"overall"`
	ByScore      []V3ValidationBucket `json:"by_score"`
}

// ValidateV3 performs a causal rolling replay. No future candles are passed to
// AnalyzeV3Foundation. One range can trigger only once, which prevents repeated
// counting as the same structure remains visible over subsequent candles.
func ValidateV3(symbol string, input []models.OHLCV) V3ValidationSummary {
	bars := v2Chronological(input)
	out := V3ValidationSummary{Symbol:symbol, Timeframe:"15M", Bars:len(bars)}
	const window = 200
	const h4, h8, h16 = 16, 32, 64
	if len(bars) < window+h16+1 { return out }

	seen := map[string]bool{}
	for i := window-1; i+h16 < len(bars); i++ {
		start := i-window+1
		a := AnalyzeV3Foundation(symbol, "15M", bars[start:i+1])
		if !a.ReadyForStudy || !a.HasSpring || !a.HasTest { continue }
		id := v3ValidationRangeID(a)
		if id == "" || seen[id] { continue }
		seen[id] = true

		entry := bars[i].Close
		if entry <= 0 { continue }
		e := V3ValidationEvent{
			BarIndex:i, Time:bars[i].OpenTime.Unix(), EntryPrice:entry, RangeID:id,
			TradeScore:a.TradeScore, SpringQuality:a.SpringQuality, TestQuality:a.TestQuality,
			StructureConf:a.StructureConfidence,
			Return4H:pctReturn(entry,bars[i+h4].Close), Return8H:pctReturn(entry,bars[i+h8].Close),
			Return16H:pctReturn(entry,bars[i+h16].Close),
		}
		maxHigh, minLow := entry, entry
		for j:=i+1; j<=i+h16; j++ {
			if bars[j].High > maxHigh { maxHigh = bars[j].High }
			if bars[j].Low < minLow { minLow = bars[j].Low }
		}
		e.MaxFav16H = pctReturn(entry,maxHigh)
		e.MaxAdverse16H = pctReturn(entry,minLow)
		out.Events = append(out.Events,e)
	}
	out.UniqueRanges = len(seen)
	out.Overall = summarizeV3Validation("ALL",out.Events)
	for _, threshold := range []float64{0.60,0.65,0.70,0.75,0.80} {
		name := fmt.Sprintf("SCORE>=%.2f",threshold)
		filtered := filterV3Validation(out.Events,func(e V3ValidationEvent) bool { return e.TradeScore >= threshold })
		out.ByScore = append(out.ByScore,summarizeV3Validation(name,filtered))
	}
	return out
}

func v3ValidationRangeID(a V3Analysis) string {
	var ps, sc, ar int64
	for _, e := range a.Events {
		switch e.Type {
		case V3EventPS: ps=e.Time
		case V3EventSC: sc=e.Time
		case V3EventAR: ar=e.Time
		}
	}
	if ps==0 || sc==0 || ar==0 { return "" }
	return fmt.Sprintf("%d-%d-%d",ps,sc,ar)
}

func summarizeV3Validation(name string, events []V3ValidationEvent) V3ValidationBucket {
	b := V3ValidationBucket{Name:name,Triggers:len(events)}
	if len(events)==0 { return b }
	var w4,w8,w16 int
	for _,e := range events {
		b.AvgReturn4H += e.Return4H; b.AvgReturn8H += e.Return8H; b.AvgReturn16H += e.Return16H
		b.AvgMFE16H += e.MaxFav16H; b.AvgMAE16H += e.MaxAdverse16H; b.AvgScore += e.TradeScore
		if e.Return4H>0 { w4++ }; if e.Return8H>0 { w8++ }; if e.Return16H>0 { w16++ }
	}
	n:=float64(len(events))
	b.AvgReturn4H/=n; b.AvgReturn8H/=n; b.AvgReturn16H/=n; b.AvgMFE16H/=n; b.AvgMAE16H/=n; b.AvgScore/=n
	b.WinRate4H=float64(w4)/n*100; b.WinRate8H=float64(w8)/n*100; b.WinRate16H=float64(w16)/n*100
	return b
}

func filterV3Validation(events []V3ValidationEvent, keep func(V3ValidationEvent) bool) []V3ValidationEvent {
	out:=make([]V3ValidationEvent,0)
	for _,e:=range events { if keep(e) { out=append(out,e) } }
	return out
}
