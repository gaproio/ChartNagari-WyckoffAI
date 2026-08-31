package wyckoff

import (
	"math"

	"github.com/Ju571nK/Chatter/pkg/models"
)

// V2Phase is the structural Wyckoff phase within an accumulation range.
type V2Phase string

const (
	V2PhaseUnknown V2Phase = "unknown"
	V2PhaseA       V2Phase = "A"
	V2PhaseB       V2Phase = "B"
	V2PhaseC       V2Phase = "C"
	V2PhaseD       V2Phase = "D"
	V2PhaseE       V2Phase = "E"
)

// V2EventType identifies classical accumulation events.
type V2EventType string

const (
	V2EventSC     V2EventType = "SC"
	V2EventAR     V2EventType = "AR"
	V2EventST     V2EventType = "ST"
	V2EventSpring V2EventType = "SPRING"
	V2EventTest   V2EventType = "TEST"
	V2EventSOS    V2EventType = "SOS"
	V2EventLPS    V2EventType = "LPS"
)

// V2Event records one structural Wyckoff event.
type V2Event struct {
	Type      V2EventType `json:"type"`
	BarIndex  int         `json:"bar_index"`
	Time      int64       `json:"time"`
	Price     float64     `json:"price"`
	VolumeRel float64     `json:"volume_rel"`
}

// TradingRange describes the detected accumulation range.
type TradingRange struct {
	Support    float64 `json:"support"`
	Resistance float64 `json:"resistance"`
	StartIndex int     `json:"start_index"`
	EndIndex   int     `json:"end_index"`
	WidthPct   float64 `json:"width_pct"`
}

// V2Analysis is the structured accumulation analysis used by the next engine.
type V2Analysis struct {
	Symbol       string       `json:"symbol"`
	Timeframe    string       `json:"timeframe"`
	Phase        V2Phase      `json:"phase"`
	Range        TradingRange `json:"range"`
	Events       []V2Event    `json:"events"`
	Confidence   float64      `json:"confidence"`
	HasSpring    bool         `json:"has_spring"`
	HasTest      bool         `json:"has_test"`
	HasSOS       bool         `json:"has_sos"`
	HasLPS       bool         `json:"has_lps"`
	ReadyForLong bool         `json:"ready_for_long"`
}

// AnalyzeV2 performs structural Wyckoff accumulation analysis.
// Bars may arrive oldest-first or newest-first; the function normalizes them.
//
// The first V2 implementation deliberately favors explainable structure over
// curve-fitted thresholds. It detects:
//   SC -> AR -> ST -> optional Spring -> Test -> SOS -> LPS
// and maps those events to phases A-E.
func AnalyzeV2(symbol, timeframe string, input []models.OHLCV) V2Analysis {
	bars := v2Chronological(input)
	out := V2Analysis{Symbol: symbol, Timeframe: timeframe, Phase: V2PhaseUnknown}
	if len(bars) < 30 {
		return out
	}

	volMA := v2VolumeMA(bars, 20)
	atr := v2ATR(bars, 14)
	if atr <= 0 {
		return out
	}

	// Phase A begins with a selling climax selected from the older portion of
	// the analysis window. A valid SC combines downside expansion and volume.
	sc := -1
	searchEnd := len(bars) - 12
	if searchEnd < 15 {
		return out
	}
	for i := 10; i < searchEnd; i++ {
		if volMA[i] <= 0 {
			continue
		}
		spread := bars[i].High - bars[i].Low
		closePos := v2ClosePosition(bars[i])
		if bars[i].Volume >= 1.6*volMA[i] && spread >= 1.25*atr && closePos >= 0.35 {
			if sc == -1 || bars[i].Low < bars[sc].Low {
				sc = i
			}
		}
	}
	if sc < 0 {
		return out
	}

	// Automatic Rally: strongest rally high shortly after SC.
	arStart := sc + 1
	arEnd := minInt(len(bars)-1, sc+12)
	if arStart > arEnd {
		return out
	}
	ar := arStart
	for i := arStart + 1; i <= arEnd; i++ {
		if bars[i].High > bars[ar].High {
			ar = i
		}
	}
	if ar <= sc || bars[ar].High <= bars[sc].High {
		return out
	}

	support := bars[sc].Low
	resistance := bars[ar].High
	mid := (support + resistance) / 2
	if mid <= 0 || resistance <= support {
		return out
	}
	widthPct := (resistance - support) / mid * 100
	if widthPct < 2 || widthPct > 35 {
		return out
	}

	out.Range = TradingRange{Support: support, Resistance: resistance, StartIndex: sc, EndIndex: len(bars) - 1, WidthPct: widthPct}
	out.Events = append(out.Events,
		v2Event(V2EventSC, sc, bars, volMA),
		v2Event(V2EventAR, ar, bars, volMA),
	)
	out.Phase = V2PhaseA
	out.Confidence = 0.25

	// Secondary Test: revisit support after AR without materially breaking it.
	st := -1
	tolerance := math.Max(0.35*atr, (resistance-support)*0.06)
	for i := ar + 1; i < len(bars); i++ {
		if bars[i].Low <= support+tolerance && bars[i].Close >= support-tolerance {
			st = i
			break
		}
	}
	if st < 0 {
		return out
	}
	out.Events = append(out.Events, v2Event(V2EventST, st, bars, volMA))
	out.Phase = V2PhaseB
	out.Confidence = 0.45

	// Spring: penetration below support followed by a close back inside range.
	spring := -1
	for i := st + 1; i < len(bars); i++ {
		penetration := support - bars[i].Low
		if penetration > 0 && penetration <= 1.5*atr && bars[i].Close > support {
			spring = i
			break
		}
	}
	if spring >= 0 {
		out.Events = append(out.Events, v2Event(V2EventSpring, spring, bars, volMA))
		out.HasSpring = true
		out.Phase = V2PhaseC
		out.Confidence = 0.65
	}

	// Test: after a Spring, price revisits the lower range on lower/equal effort
	// while holding above the Spring low. No Spring means we remain Phase B.
	test := -1
	if spring >= 0 {
		for i := spring + 1; i < len(bars); i++ {
			nearSupport := bars[i].Low <= support+tolerance
			holdsSpring := bars[i].Low > bars[spring].Low
			lighterVolume := volMA[i] <= 0 || bars[i].Volume <= 1.1*volMA[i]
			if nearSupport && holdsSpring && bars[i].Close >= support && lighterVolume {
				test = i
				break
			}
		}
	}
	if test >= 0 {
		out.Events = append(out.Events, v2Event(V2EventTest, test, bars, volMA))
		out.HasTest = true
		out.Confidence = 0.78
	}

	// Sign of Strength: decisive close above resistance with expanding spread.
	sosStart := st + 1
	if test >= 0 {
		sosStart = test + 1
	} else if spring >= 0 {
		sosStart = spring + 1
	}
	sos := -1
	for i := sosStart; i < len(bars); i++ {
		spread := bars[i].High - bars[i].Low
		volOK := volMA[i] <= 0 || bars[i].Volume >= 1.05*volMA[i]
		if bars[i].Close > resistance+0.15*atr && spread >= 0.9*atr && volOK {
			sos = i
			break
		}
	}
	if sos >= 0 {
		out.Events = append(out.Events, v2Event(V2EventSOS, sos, bars, volMA))
		out.HasSOS = true
		out.Phase = V2PhaseD
		out.Confidence = 0.9
	}

	// Last Point of Support: post-SOS pullback that holds around old resistance.
	lps := -1
	if sos >= 0 {
		for i := sos + 1; i < len(bars); i++ {
			if bars[i].Low >= resistance-0.5*atr && bars[i].Close >= resistance {
				lps = i
				break
			}
		}
	}
	if lps >= 0 {
		out.Events = append(out.Events, v2Event(V2EventLPS, lps, bars, volMA))
		out.HasLPS = true
		out.Confidence = 0.97
	}

	last := bars[len(bars)-1]
	if sos >= 0 && last.Close > resistance+atr {
		out.Phase = V2PhaseE
		out.Confidence = 1.0
	}

	// A conservative alert gate: a Phase-C long requires Spring + Test; Phase-D
	// and E structures are also actionable after SOS confirmation.
	out.ReadyForLong = (out.HasSpring && out.HasTest) || out.HasSOS
	return out
}

func v2Chronological(src []models.OHLCV) []models.OHLCV {
	out := append([]models.OHLCV(nil), src...)
	if len(out) > 1 && out[0].OpenTime.After(out[len(out)-1].OpenTime) {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

func v2VolumeMA(bars []models.OHLCV, period int) []float64 {
	out := make([]float64, len(bars))
	var sum float64
	for i := range bars {
		sum += bars[i].Volume
		if i >= period {
			sum -= bars[i-period].Volume
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

func v2ATR(bars []models.OHLCV, period int) float64 {
	if len(bars) < period+1 {
		return 0
	}
	start := len(bars) - period
	var sum float64
	for i := start; i < len(bars); i++ {
		prevClose := bars[i-1].Close
		tr := math.Max(bars[i].High-bars[i].Low, math.Max(math.Abs(bars[i].High-prevClose), math.Abs(bars[i].Low-prevClose)))
		sum += tr
	}
	return sum / float64(period)
}

func v2ClosePosition(b models.OHLCV) float64 {
	spread := b.High - b.Low
	if spread <= 0 {
		return 0.5
	}
	return (b.Close - b.Low) / spread
}

func v2Event(t V2EventType, i int, bars []models.OHLCV, volMA []float64) V2Event {
	rel := 0.0
	if i >= 0 && i < len(volMA) && volMA[i] > 0 {
		rel = bars[i].Volume / volMA[i]
	}
	return V2Event{Type: t, BarIndex: i, Time: bars[i].OpenTime.Unix(), Price: bars[i].Close, VolumeRel: rel}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
