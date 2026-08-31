package wyckoff

import (
	"math"

	"github.com/Ju571nK/Chatter/pkg/models"
)

// V3EventType identifies structural events used by the V3 detector.
type V3EventType string

const (
	V3EventPS     V3EventType = "PS"
	V3EventSC     V3EventType = "SC"
	V3EventAR     V3EventType = "AR"
	V3EventST     V3EventType = "ST"
	V3EventSpring V3EventType = "SPRING"
	V3EventTest   V3EventType = "TEST"
)

type V3Event struct {
	Type      V3EventType `json:"type"`
	BarIndex  int         `json:"bar_index"`
	Time      int64       `json:"time"`
	Price     float64     `json:"price"`
	VolumeRel float64     `json:"volume_rel"`
	ATR       float64     `json:"atr"`
	Quality   float64     `json:"quality"`
}

// V3Analysis separates structural confidence from trade quality. Structure
// confidence answers "does this look like accumulation?" while TradeScore
// answers "is this Spring/Test sequence attractive enough to study as an entry?"
type V3Analysis struct {
	Symbol              string       `json:"symbol"`
	Timeframe           string       `json:"timeframe"`
	Phase               V2Phase      `json:"phase"`
	Range               TradingRange `json:"range"`
	Events              []V3Event    `json:"events"`
	StructureConfidence float64      `json:"structure_confidence"`
	TradeScore          float64      `json:"trade_score"`
	SpringQuality       float64      `json:"spring_quality"`
	TestQuality         float64      `json:"test_quality"`
	PriorDowntrend      bool         `json:"prior_downtrend"`
	HasPS               bool         `json:"has_ps"`
	HasSC               bool         `json:"has_sc"`
	HasAR               bool         `json:"has_ar"`
	HasST               bool         `json:"has_st"`
	HasSpring           bool         `json:"has_spring"`
	HasTest             bool         `json:"has_test"`
	ReadyForStudy       bool         `json:"ready_for_study"`
}

// AnalyzeV3Foundation detects accumulation using only information available at
// each historical candle. Every volatility test uses rolling ATR[i], not the
// ATR of the latest candle in the window.
func AnalyzeV3Foundation(symbol, timeframe string, input []models.OHLCV) V3Analysis {
	bars := v2Chronological(input)
	out := V3Analysis{Symbol: symbol, Timeframe: timeframe, Phase: V2PhaseUnknown}
	if len(bars) < 50 { return out }

	atr := v3ATRSeries(bars, 14)
	volMA := v2VolumeMA(bars, 20)

	searchEnd := len(bars) - 12
	sc := -1
	for i := 30; i < searchEnd; i++ {
		if atr[i] <= 0 || volMA[i] <= 0 || !v3PriorDowntrend(bars, i) { continue }
		spread := bars[i].High - bars[i].Low
		closePos := v2ClosePosition(bars[i])
		if bars[i].Volume < 1.6*volMA[i] || spread < 1.25*atr[i] || closePos < 0.35 { continue }
		if sc == -1 || bars[i].Low < bars[sc].Low { sc = i }
	}
	if sc < 0 { return out }
	out.PriorDowntrend = true
	out.HasSC = true

	ps := -1
	psStart := sc - 16
	if psStart < 20 { psStart = 20 }
	for i := psStart; i < sc; i++ {
		if atr[i] <= 0 || volMA[i] <= 0 { continue }
		spread := bars[i].High - bars[i].Low
		if bars[i].Volume >= 1.2*volMA[i] && spread >= 0.9*atr[i] && bars[i].Low > bars[sc].Low { ps = i }
	}
	if ps < 0 { return out }
	out.HasPS = true

	ar := -1
	arEnd := minInt(len(bars)-1, sc+16)
	minRally := math.Max(2.0*atr[sc], math.Abs(bars[sc].Close-bars[sc].Low))
	for i := sc + 1; i <= arEnd; i++ {
		if bars[i].High-bars[sc].Low < minRally { continue }
		if ar == -1 || bars[i].High > bars[ar].High { ar = i }
	}
	if ar < 0 || bars[ar].High <= bars[sc].High { return out }
	out.HasAR = true

	support := bars[sc].Low
	resistance := bars[ar].High
	mid := (support + resistance) / 2
	if mid <= 0 || resistance <= support { return out }
	widthPct := (resistance-support)/mid*100
	if widthPct < 2 || widthPct > 30 { return out }
	out.Range = TradingRange{Support:support, Resistance:resistance, StartIndex:ps, EndIndex:len(bars)-1, WidthPct:widthPct}
	out.Phase = V2PhaseA
	out.StructureConfidence = 0.45
	out.Events = append(out.Events,
		v3Event(V3EventPS, ps, bars, volMA, atr, 0),
		v3Event(V3EventSC, sc, bars, volMA, atr, 0),
		v3Event(V3EventAR, ar, bars, volMA, atr, 0),
	)

	st := -1
	for i := ar + 1; i < len(bars); i++ {
		if atr[i] <= 0 { continue }
		tolerance := math.Max(0.35*atr[i], (resistance-support)*0.06)
		nearSupport := bars[i].Low <= support+tolerance
		holdsSC := bars[i].Low >= support-0.35*atr[i]
		lessEffort := bars[i].Volume < bars[sc].Volume
		if nearSupport && holdsSC && lessEffort && bars[i].Close >= support-tolerance { st = i; break }
	}
	if st < 0 { return out }
	out.HasST = true
	out.Phase = V2PhaseB
	out.StructureConfidence = 0.65
	out.Events = append(out.Events, v3Event(V3EventST, st, bars, volMA, atr, 0))

	// Spring quality combines four independent Wyckoff ideas: controlled
	// penetration, strong rejection, favorable close location and non-excessive
	// effort. It is intentionally continuous rather than a binary threshold.
	spring := -1
	bestSpringQuality := 0.0
	for i := st + 1; i < len(bars); i++ {
		if atr[i] <= 0 || bars[i].Low >= support || bars[i].Close <= support { continue }
		penetrationATR := (support-bars[i].Low)/atr[i]
		if penetrationATR <= 0 || penetrationATR > 1.75 { continue }
		q := v3SpringQuality(bars[i], support, atr[i], volMA[i])
		if q > bestSpringQuality { spring, bestSpringQuality = i, q }
	}
	if spring < 0 || bestSpringQuality < 0.50 { return out }
	out.HasSpring = true
	out.SpringQuality = bestSpringQuality
	out.Phase = V2PhaseC
	out.StructureConfidence = 0.76
	out.Events = append(out.Events, v3Event(V3EventSpring, spring, bars, volMA, atr, bestSpringQuality))

	// A Test must hold the Spring low and return to support with lower effort.
	// Quality rewards volume contraction, narrower spread and a close back above
	// support. This directly measures effort-versus-result rather than merely
	// checking "volume < moving average".
	test := -1
	bestTestQuality := 0.0
	for i := spring + 1; i < len(bars); i++ {
		if atr[i] <= 0 { continue }
		tolerance := math.Max(0.35*atr[i], (resistance-support)*0.06)
		if bars[i].Low > support+tolerance || bars[i].Low <= bars[spring].Low || bars[i].Close < support { continue }
		q := v3TestQuality(bars[spring], bars[i], volMA[i])
		if q > bestTestQuality { test, bestTestQuality = i, q }
	}
	if test < 0 || bestTestQuality < 0.50 { return out }
	out.HasTest = true
	out.TestQuality = bestTestQuality
	out.StructureConfidence = 0.84
	out.Events = append(out.Events, v3Event(V3EventTest, test, bars, volMA, atr, bestTestQuality))

	// TradeScore is deliberately separate from structural confidence. The first
	// V3 study score weights the Test slightly more heavily because supply drying
	// up on the Test is the confirmation we actually care about for entry timing.
	out.TradeScore = 0.45*out.SpringQuality + 0.55*out.TestQuality
	out.ReadyForStudy = out.TradeScore >= 0.60
	return out
}

func v3SpringQuality(b models.OHLCV, support, atr, volMA float64) float64 {
	if atr <= 0 { return 0 }
	penetration := (support-b.Low)/atr
	// Best zone is roughly 0.25-0.9 ATR below support: meaningful shakeout,
	// without becoming an uncontrolled breakdown.
	penetrationScore := 1.0 - math.Min(math.Abs(penetration-0.55)/0.90, 1.0)
	closeScore := v2ClosePosition(b)
	rejection := (b.Close-b.Low)/atr
	rejectionScore := math.Min(math.Max(rejection/1.25, 0), 1)
	volumeScore := 0.7
	if volMA > 0 {
		rel := b.Volume/volMA
		// Moderate/strong effort is welcome, but extreme volume gets no bonus.
		if rel >= 1.0 && rel <= 2.2 { volumeScore = 1.0 } else if rel < 0.75 || rel > 3.0 { volumeScore = 0.45 }
	}
	return clamp01(0.25*penetrationScore + 0.30*closeScore + 0.30*rejectionScore + 0.15*volumeScore)
}

func v3TestQuality(spring, test models.OHLCV, volMA float64) float64 {
	springSpread := spring.High-spring.Low
	testSpread := test.High-test.Low
	spreadScore := 0.5
	if springSpread > 0 { spreadScore = clamp01(1.0-testSpread/springSpread) }
	volumeScore := 0.5
	if spring.Volume > 0 { volumeScore = clamp01(1.0-test.Volume/spring.Volume) }
	if volMA > 0 && test.Volume <= 0.9*volMA { volumeScore = math.Max(volumeScore, 0.75) }
	closeScore := v2ClosePosition(test)
	return clamp01(0.40*volumeScore + 0.30*spreadScore + 0.30*closeScore)
}

// v3PriorDowntrend requires both net decline and a falling linear-regression
// slope over the 24 bars preceding the candidate SC.
func v3PriorDowntrend(bars []models.OHLCV, end int) bool {
	const lookback = 24
	if end < lookback { return false }
	start := end-lookback
	first := bars[start].Close
	last := bars[end-1].Close
	if first <= 0 || last >= first*0.97 { return false }
	var sumX, sumY, sumXY, sumXX float64
	n := float64(lookback)
	for j := 0; j < lookback; j++ {
		x := float64(j); y := bars[start+j].Close
		sumX += x; sumY += y; sumXY += x*y; sumXX += x*x
	}
	denom := n*sumXX-sumX*sumX
	if denom == 0 { return false }
	slope := (n*sumXY-sumX*sumY)/denom
	return slope < 0
}

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

func v3Event(t V3EventType, i int, bars []models.OHLCV, volMA, atr []float64, quality float64) V3Event {
	rel := 0.0
	if volMA[i] > 0 { rel = bars[i].Volume/volMA[i] }
	return V3Event{Type:t, BarIndex:i, Time:bars[i].OpenTime.Unix(), Price:bars[i].Close, VolumeRel:rel, ATR:atr[i], Quality:quality}
}

func clamp01(v float64) float64 {
	if v < 0 { return 0 }
	if v > 1 { return 1 }
	return v
}
