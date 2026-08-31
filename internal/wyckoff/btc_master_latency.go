package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// BTCMasterLatencyBucket is descriptive research only. It classifies each
// midpoint-valid V3 structure by when the existing prospective-HL B decision
// would first become available. The live/frozen rule remains <=8 bars.
type BTCMasterLatencyBucket struct {
	Name         string  `json:"name"`
	Structures   int     `json:"structures"`
	WinRate16H   float64 `json:"win_rate_16h"`
	AvgReturn16H float64 `json:"avg_return_16h_pct"`
	AvgMFE16H    float64 `json:"avg_mfe_16h_pct"`
	AvgMAE16H    float64 `json:"avg_mae_16h_pct"`
}

type btcMasterLatencyObservation struct {
	ret16 float64
	mfe16 float64
	mae16 float64
}

// ValidateBTCMasterLatency measures B-confirmation timing from a common causal
// V3 next-open anchor. Classification can look as far as 32 candles after the
// V3 signal, so this is NOT an executable filter at the anchor and does not
// alter the frozen <=8-bar BTC rule.
func ValidateBTCMasterLatency(input []models.OHLCV, validation V3ValidationSummary) []BTCMasterLatencyBucket {
	bars := v2Chronological(input)
	groups := map[string][]btcMasterLatencyObservation{}

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

		name := "NO_B_32"
		if d8 := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, 8, v4EntryProspectiveHL); d8 >= 0 {
			name = "B_1_8"
		} else if d16 := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, 16, v4EntryProspectiveHL); d16 >= 0 {
			name = "B_9_16"
		} else if d32 := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, 32, v4EntryProspectiveHL); d32 >= 0 {
			name = "B_17_32"
		}

		anchorIdx := e.BarIndex + 1
		if anchorIdx < 0 || anchorIdx+64 >= len(bars) { continue }
		anchor := bars[anchorIdx].Open
		if anchor <= 0 { continue }
		maxHigh, minLow := anchor, anchor
		for j := anchorIdx; j <= anchorIdx+64; j++ {
			if bars[j].High > maxHigh { maxHigh = bars[j].High }
			if bars[j].Low < minLow { minLow = bars[j].Low }
		}
		groups[name] = append(groups[name], btcMasterLatencyObservation{
			ret16: pctReturn(anchor, bars[anchorIdx+64].Close),
			mfe16: pctReturn(anchor, maxHigh),
			mae16: pctReturn(anchor, minLow),
		})
	}

	out := make([]BTCMasterLatencyBucket, 0, 4)
	for _, name := range []string{"B_1_8", "B_9_16", "B_17_32", "NO_B_32"} {
		out = append(out, summarizeBTCMasterLatency(name, groups[name]))
	}
	return out
}

func summarizeBTCMasterLatency(name string, observations []btcMasterLatencyObservation) BTCMasterLatencyBucket {
	b := BTCMasterLatencyBucket{Name: name, Structures: len(observations)}
	if len(observations) == 0 { return b }
	wins := 0
	for _, o := range observations {
		b.AvgReturn16H += o.ret16
		b.AvgMFE16H += o.mfe16
		b.AvgMAE16H += o.mae16
		if o.ret16 > 0 { wins++ }
	}
	n := float64(len(observations))
	b.WinRate16H = float64(wins) / n * 100
	b.AvgReturn16H /= n
	b.AvgMFE16H /= n
	b.AvgMAE16H /= n
	return b
}
