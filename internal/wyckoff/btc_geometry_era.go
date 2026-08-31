package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// BTCAcceptedGeometryEra summarizes the geometry of the exact V3 structures
// accepted by the frozen <=8-bar B confirmation, grouped into fixed calendar
// eras. It is descriptive only and does not alter any detector or execution rule.
type BTCAcceptedGeometryEra struct {
	Name                  string  `json:"name"`
	Structures            int     `json:"structures"`
	AvgRangeWidthPct      float64 `json:"avg_range_width_pct"`
	AvgPSToTestBars       float64 `json:"avg_ps_to_test_bars"`
	AvgSpringPenATR       float64 `json:"avg_spring_penetration_atr"`
	AvgSpringToTestBars   float64 `json:"avg_spring_to_test_bars"`
	AvgTestToBDecisionBars float64 `json:"avg_test_to_b_decision_bars"`
	AvgStopDistanceATR    float64 `json:"avg_stop_distance_atr"`
	AvgSpringQuality      float64 `json:"avg_spring_quality"`
	AvgTestQuality        float64 `json:"avg_test_quality"`
}

type btcAcceptedGeometryObservation struct {
	rangeWidthPct float64
	psToTest      float64
	springPenATR  float64
	springToTest  float64
	testToB       float64
	stopATR       float64
	springQuality float64
	testQuality   float64
}

// ValidateBTCAcceptedGeometryByEra replays the existing accepted structures and
// measures geometry using only candles available by the B decision. The calendar
// buckets are fixed a priori and match the temporal-robustness diagnostic.
func ValidateBTCAcceptedGeometryByEra(input []models.OHLCV, validation V3ValidationSummary) []BTCAcceptedGeometryEra {
	bars := v2Chronological(input)
	atr := v3ATRSeries(bars, 14)
	groups := map[string][]btcAcceptedGeometryObservation{
		"2017-2019": {},
		"2020-2022": {},
		"2023-2025": {},
		"2026 PARTIAL": {},
	}

	for _, e := range validation.Events {
		if e.BarIndex < 199 || e.BarIndex >= len(bars) || e.SpringATR <= 0 { continue }
		start := e.BarIndex - 199
		a := AnalyzeV3Foundation(validation.Symbol, "15M", bars[start:e.BarIndex+1])
		if !a.HasSpring || !a.HasTest || a.Range.WidthPct <= 0 { continue }

		psLocal, springLocal, testLocal := -1, -1, -1
		springEventATR := 0.0
		for _, ev := range a.Events {
			switch ev.Type {
			case V3EventPS:
				psLocal = ev.BarIndex
			case V3EventSpring:
				springLocal = ev.BarIndex
				springEventATR = ev.ATR
			case V3EventTest:
				testLocal = ev.BarIndex
			}
		}
		if psLocal < 0 || springLocal < 0 || testLocal < 0 || springEventATR <= 0 { continue }
		psGlobal := start + psLocal
		springGlobal := start + springLocal
		testGlobal := start + testLocal
		if psGlobal < 0 || springGlobal < 0 || testGlobal < 0 || testGlobal >= len(bars) { continue }

		midpoint := (a.Range.Support + a.Range.Resistance) / 2
		if midpoint <= 0 { continue }
		decisionIdx := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, 8, v4EntryProspectiveHL)
		if decisionIdx < 0 || decisionIdx+1 >= len(bars) || decisionIdx >= len(atr) || atr[decisionIdx] <= 0 { continue }

		stop := v4VariantStop(bars, testGlobal, decisionIdx, e.SpringLow, e.SpringATR, v4StopPostTest)
		entry := bars[decisionIdx+1].Open
		if entry <= 0 || stop <= 0 || stop >= entry { continue }

		year := bars[decisionIdx+1].OpenTime.UTC().Year()
		era := btcGeometryEraName(year)
		if era == "" { continue }
		springPenATR := (a.Range.Support - bars[springGlobal].Low) / springEventATR
		if springPenATR < 0 { continue }

		groups[era] = append(groups[era], btcAcceptedGeometryObservation{
			rangeWidthPct: a.Range.WidthPct,
			psToTest: float64(testGlobal - psGlobal),
			springPenATR: springPenATR,
			springToTest: float64(testGlobal - springGlobal),
			testToB: float64(decisionIdx - testGlobal),
			stopATR: (entry - stop) / atr[decisionIdx],
			springQuality: a.SpringQuality,
			testQuality: a.TestQuality,
		})
	}

	order := []string{"2017-2019", "2020-2022", "2023-2025", "2026 PARTIAL"}
	out := make([]BTCAcceptedGeometryEra, 0, len(order))
	for _, name := range order {
		out = append(out, summarizeBTCAcceptedGeometryEra(name, groups[name]))
	}
	return out
}

func btcGeometryEraName(year int) string {
	switch {
	case year >= 2017 && year <= 2019:
		return "2017-2019"
	case year >= 2020 && year <= 2022:
		return "2020-2022"
	case year >= 2023 && year <= 2025:
		return "2023-2025"
	case year == 2026:
		return "2026 PARTIAL"
	default:
		return ""
	}
}

func summarizeBTCAcceptedGeometryEra(name string, obs []btcAcceptedGeometryObservation) BTCAcceptedGeometryEra {
	r := BTCAcceptedGeometryEra{Name: name, Structures: len(obs)}
	if len(obs) == 0 { return r }
	for _, o := range obs {
		r.AvgRangeWidthPct += o.rangeWidthPct
		r.AvgPSToTestBars += o.psToTest
		r.AvgSpringPenATR += o.springPenATR
		r.AvgSpringToTestBars += o.springToTest
		r.AvgTestToBDecisionBars += o.testToB
		r.AvgStopDistanceATR += o.stopATR
		r.AvgSpringQuality += o.springQuality
		r.AvgTestQuality += o.testQuality
	}
	n := float64(len(obs))
	r.AvgRangeWidthPct /= n
	r.AvgPSToTestBars /= n
	r.AvgSpringPenATR /= n
	r.AvgSpringToTestBars /= n
	r.AvgTestToBDecisionBars /= n
	r.AvgStopDistanceATR /= n
	r.AvgSpringQuality /= n
	r.AvgTestQuality /= n
	return r
}
