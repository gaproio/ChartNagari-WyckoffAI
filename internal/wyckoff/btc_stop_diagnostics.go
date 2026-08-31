package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// BTCStopDiagnostic is descriptive research for the frozen BTCUSDT/15M profile.
// It does not change the post-Test stop. It asks whether trades that hit the
// frozen stop later recovered to the original 1R/2R/3R targets within the same
// 16-hour window.
type BTCStopDiagnostic struct {
	StopTrades          int     `json:"stop_trades"`
	AvgBarsToStop       float64 `json:"avg_bars_to_stop"`
	AvgMFEBeforeStopR   float64 `json:"avg_mfe_before_stop_r"`
	AvgRecoveryAfterR   float64 `json:"avg_recovery_after_stop_r"`
	RecoveredToEntry    int     `json:"recovered_to_entry"`
	Hit1RAfterStop      int     `json:"hit_1r_after_stop"`
	Hit2RAfterStop      int     `json:"hit_2r_after_stop"`
	Hit3RAfterStop      int     `json:"hit_3r_after_stop"`
}

// ValidateBTCStopDiagnostic uses the frozen BTC profile:
// B confirmation <=8 bars -> next-open entry -> post-Test stop -> 3R target ->
// 64-bar (16h) horizon. Only stopped trades are inspected after the stop event.
func ValidateBTCStopDiagnostic(input []models.OHLCV, validation V3ValidationSummary) BTCStopDiagnostic {
	bars := v2Chronological(input)
	var out BTCStopDiagnostic

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
		decisionIdx := v4VariantEntry(bars, e.BarIndex, testGlobal, midpoint, 8, v4EntryProspectiveHL)
		if decisionIdx < 0 { continue }
		execIdx := decisionIdx + 1
		if execIdx >= len(bars) || execIdx+64 >= len(bars) { continue }

		stop := v4VariantStop(bars, testGlobal, decisionIdx, e.SpringLow, e.SpringATR, v4StopPostTest)
		entry := bars[execIdx].Open
		if entry <= 0 || stop <= 0 || stop >= entry { continue }
		risk := entry - stop
		if risk <= 0 { continue }

		_, outcome, stopIdx := simulateBTCMasterOutcome(bars, execIdx, 64, entry, stop, 3)
		if outcome != "STOP" { continue }
		out.StopTrades++
		out.AvgBarsToStop += float64(stopIdx-execIdx)

		maxBefore := entry
		for j:=execIdx; j<=stopIdx; j++ {
			if bars[j].High > maxBefore { maxBefore = bars[j].High }
		}
		out.AvgMFEBeforeStopR += (maxBefore-entry)/risk

		end := execIdx+64
		maxAfter := stop
		for j:=stopIdx+1; j<=end; j++ {
			if bars[j].High > maxAfter { maxAfter = bars[j].High }
		}
		recoveryR := (maxAfter-entry)/risk
		out.AvgRecoveryAfterR += recoveryR
		if maxAfter >= entry { out.RecoveredToEntry++ }
		if maxAfter >= entry+risk { out.Hit1RAfterStop++ }
		if maxAfter >= entry+2*risk { out.Hit2RAfterStop++ }
		if maxAfter >= entry+3*risk { out.Hit3RAfterStop++ }
	}

	if out.StopTrades > 0 {
		n := float64(out.StopTrades)
		out.AvgBarsToStop /= n
		out.AvgMFEBeforeStopR /= n
		out.AvgRecoveryAfterR /= n
	}
	return out
}
