package wyckoff

import (
	"math"

	"github.com/Ju571nK/Chatter/pkg/models"
)

// V3EventType identifies the first structural events used by the V3 detector.
type V3EventType string

const (
	V3EventPS V3EventType = "PS"
	V3EventSC V3EventType = "SC"
	V3EventAR V3EventType = "AR"
	V3EventST V3EventType = "ST"
)

type V3Event struct {
	Type      V3EventType `json:"type"`
	BarIndex  int         `json:"bar_index"`
	Time      int64       `json:"time"`
	Price     float64     `json:"price"`
	VolumeRel float64     `json:"volume_rel"`
	ATR       float64     `json:"atr"`
}

// V3Analysis deliberately separates structural quality from any future trade
// score. This first V3 milestone stops at Phase B: prior downtrend -> PS -> SC
// -> AR -> ST. Spring/Test/SOS/LPS are added only after this foundation is
// validated independently.
type V3Analysis struct {
	Symbol              string       `json:"symbol"`
	Timeframe           string       `json:"timeframe"`
	Phase               V2Phase      `json:"phase"`
	Range               TradingRange `json:"range"`
	Events              []V3Event    `json:"events"`
	StructureConfidence float64      `json:"structure_confidence"`
	PriorDowntrend      bool         `json:"prior_downtrend"`
	HasPS               bool         `json:"has_ps"`
	HasSC               bool         `json:"has_sc"`
	HasAR               bool         `json:"has_ar"`
	HasST               bool         `json:"has_st"`
}

// AnalyzeV3Foundation detects an accumulation foundation using only information
// available at each historical candle. In particular, every volatility test
// uses rolling ATR[i], not the ATR of the latest candle in the window.
func AnalyzeV3Foundation(symbol, timeframe string, input []models.OHLCV) V3Analysis {
	bars := v2Chronological(input)
	out := V3Analysis{Symbol: symbol, Timeframe: timeframe, Phase: V2PhaseUnknown}
	if len(bars) < 50 {
		return out
	}

	atr := v3ATRSeries(bars, 14)
	volMA := v2VolumeMA(bars, 20)

	// Keep enough future bars for AR and ST. SC must also have enough prior bars
	// to prove that a real decline preceded the stopping action.
	searchEnd := len(bars) - 12
	sc := -1
	for i := 30; i < searchEnd; i++ {
		if atr[i] <= 0 || volMA[i] <= 0 || !v3PriorDowntrend(bars, i) {
			continue
		}
		spread := bars[i].High - bars[i].Low
		closePos := v2ClosePosition(bars[i])
		if bars[i].Volume < 1.6*volMA[i] || spread < 1.25*atr[i] || closePos < 0.35 {
			continue
		}
		// Prefer the most climactic low when several candidates exist.
		if sc == -1 || bars[i].Low < bars[sc].Low {
			sc = i
		}
	}
	if sc < 0 {
		return out
	}
	out.PriorDowntrend = true
	out.HasSC = true

	// Preliminary Support should appear shortly before the SC: downside spread
	// and volume expand, but the event is less extreme than the eventual SC.
	ps := -1
	psStart := sc - 16
	if psStart < 20 { psStart = 20 }
	for i := psStart; i < sc; i++ {
		if atr[i] <= 0 || volMA[i] <= 0 { continue }
		spread := bars[i].High - bars[i].Low
		if bars[i].Volume >= 1.2*volMA[i] && spread >= 0.9*atr[i] && bars[i].Low > bars[sc].Low {
			ps = i
		}
	}
	if ps < 0 {
		return out
	}
	out.HasPS = true

	// AR must demonstrate a meaningful response from the SC rather than merely
	// being the highest candle in an arbitrary fixed window.
	ar := -1
	arEnd := minInt(len(bars)-1, sc+16)
	minRally := math.Max(2.0*atr[sc], math.Abs(bars[sc].Close-bars[sc].Low))
	for i := sc + 1; i <= arEnd; i++ {
		if bars[i].High-bars[sc].Low < minRally { continue }
		if ar == -1 || bars[i].High > bars[ar].High { ar = i }
	}
	if ar < 0 || bars[ar].High <= bars[sc].High {
		return out
	}
	out.HasAR = true

	support := bars[sc].Low
	resistance := bars[ar].High
	mid := (support + resistance) / 2
	if mid <= 0 || resistance <= support { return out }
	widthPct := (resistance-support)/mid*100
	if widthPct < 2 || widthPct > 30 { return out }
	out.Range = TradingRange{Support: support, Resistance: resistance, StartIndex: ps, EndIndex: len(bars)-1, WidthPct: widthPct}
	out.Phase = V2PhaseA
	out.StructureConfidence = 0.45

	out.Events = append(out.Events,
		v3Event(V3EventPS, ps, bars, volMA, atr),
		v3Event(V3EventSC, sc, bars, volMA, atr),
		v3Event(V3EventAR, ar, bars, volMA, atr),
	)

	// ST revisits the lower part of the range after AR, holds the SC low within
	// local volatility tolerance, and should show less effort than the SC.
	st := -1
	for i := ar + 1; i < len(bars); i++ {
		if atr[i] <= 0 { continue }
		tolerance := math.Max(0.35*atr[i], (resistance-support)*0.06)
		nearSupport := bars[i].Low <= support+tolerance
		holdsSC := bars[i].Low >= support-0.35*atr[i]
		lessEffort := bars[i].Volume < bars[sc].Volume
		if nearSupport && holdsSC && lessEffort && bars[i].Close >= support-tolerance {
			st = i
			break
		}
	}
	if st < 0 { return out }

	out.HasST = true
	out.Phase = V2PhaseB
	out.StructureConfidence = 0.65
	out.Events = append(out.Events, v3Event(V3EventST, st, bars, volMA, atr))
	return out
}

// v3PriorDowntrend requires both net decline and a falling linear-regression
// slope over the 24 bars preceding the candidate SC.
func v3PriorDowntrend(bars []models.OHLCV, end int) bool {
	const lookback = 24
	if end < lookback { return false }
	start := end - lookback
	first := bars[start].Close
	last := bars[end-1].Close
	if first <= 0 || last >= first*0.97 { return false }

	var sumX, sumY, sumXY, sumXX float64
	n := float64(lookback)
	for j := 0; j < lookback; j++ {
		x := float64(j)
		y := bars[start+j].Close
		sumX += x; sumY += y; sumXY += x*y; sumXX += x*x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 { return false }
	slope := (n*sumXY - sumX*sumY) / denom
	return slope < 0
}

// v3ATRSeries is a simple rolling ATR series. Index i contains only true-range
// observations up through i, preventing future volatility from changing the
// classification of an older event.
func v3ATRSeries(bars []models.OHLCV, period int) []float64 {
	out := make([]float64, len(bars))
	if period <= 0 || len(bars) < 2 { return out }
	tr := make([]float64, len(bars))
	for i := 1; i < len(bars); i++ {
		prev := bars[i-1].Close
		tr[i] = math.Max(bars[i].High-bars[i].Low, math.Max(math.Abs(bars[i].High-prev), math.Abs(bars[i].Low-prev)))
	}
	var sum float64
	for i := 1; i < len(bars); i++ {
		sum += tr[i]
		if i > period { sum -= tr[i-period] }
		if i >= period { out[i] = sum/float64(period) }
	}
	return out
}

func v3Event(t V3EventType, i int, bars []models.OHLCV, volMA, atr []float64) V3Event {
	rel := 0.0
	if volMA[i] > 0 { rel = bars[i].Volume/volMA[i] }
	return V3Event{Type:t, BarIndex:i, Time:bars[i].OpenTime.Unix(), Price:bars[i].Close, VolumeRel:rel, ATR:atr[i]}
}
