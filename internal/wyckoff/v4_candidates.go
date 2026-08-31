package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// V4CandidateResult measures one fixed confirmation rule. V4 candidates are
// research-only: they do not change V3 detection or live signals.
type V4CandidateResult struct {
	Name            string  `json:"name"`
	Signals         int     `json:"signals"`
	WinRate16H      float64 `json:"win_rate_16h"`
	AvgReturn16H    float64 `json:"avg_return_16h_pct"`
	ConfirmedTrades int     `json:"confirmed_trades"`
	ConfirmRate     float64 `json:"confirm_rate"`
	AvgConfirmedR3  float64 `json:"avg_confirmed_r3"`
}

type V4CandidateReport struct {
	Candidates []V4CandidateResult `json:"candidates"`
}

// ValidateV4Candidates compares a deliberately small, predefined rule set.
// Nothing is optimized per symbol. Every rule uses the same V3 signals and the
// same eight-candle post-Test observation window.
func ValidateV4Candidates(input []models.OHLCV, validation V3ValidationSummary) V4CandidateReport {
	obs := v4Observations(input, validation)
	rules := []struct {
		name string
		keep func(V3ConfirmationObservation) bool
	}{
		{"A MIDPOINT", func(o V3ConfirmationObservation) bool { return o.MidpointReclaim }},
		{"B MIDPOINT+HL", func(o V3ConfirmationObservation) bool { return o.MidpointReclaim && o.HigherLow }},
		{"C MIDPOINT+ABOVE", func(o V3ConfirmationObservation) bool { return o.MidpointReclaim && o.CloseAboveSignalHigh }},
		{"D MIDPOINT+(HL|ABOVE)", func(o V3ConfirmationObservation) bool { return o.MidpointReclaim && (o.HigherLow || o.CloseAboveSignalHigh) }},
		{"E FEATURES>=4", func(o V3ConfirmationObservation) bool { return o.FeatureCount >= 4 }},
		{"F FEATURES>=5", func(o V3ConfirmationObservation) bool { return o.FeatureCount >= 5 }},
		{"G MIDPOINT+FEATURES>=4", func(o V3ConfirmationObservation) bool { return o.MidpointReclaim && o.FeatureCount >= 4 }},
	}
	out := V4CandidateReport{Candidates: make([]V4CandidateResult, 0, len(rules))}
	for _, rule := range rules {
		out.Candidates = append(out.Candidates, summarizeV4Candidate(rule.name, filterConfirmation(obs, rule.keep)))
	}
	return out
}

func v4Observations(input []models.OHLCV, validation V3ValidationSummary) []V3ConfirmationObservation {
	bars := v2Chronological(input)
	obs := make([]V3ConfirmationObservation, 0, len(validation.Events))
	for _, e := range validation.Events {
		if e.BarIndex < 199 || e.BarIndex+64 >= len(bars) { continue }
		a := AnalyzeV3Foundation(validation.Symbol, "15M", bars[e.BarIndex-199:e.BarIndex+1])
		if !a.HasTest || !a.HasSpring { continue }

		testIdx := -1
		for _, ev := range a.Events {
			if ev.Type == V3EventTest { testIdx = ev.BarIndex; break }
		}
		if testIdx < 0 { continue }
		globalTest := e.BarIndex-199+testIdx
		if globalTest < 0 || globalTest >= len(bars) { continue }
		testBar := bars[globalTest]
		mid := (a.Range.Support+a.Range.Resistance)/2
		end := e.BarIndex+8
		if end >= len(bars) { end=len(bars)-1 }

		o := V3ConfirmationObservation{BarIndex:e.BarIndex, TradeScore:e.TradeScore, Return16H:e.Return16H}
		minLow := 1e308
		for i:=e.BarIndex+1; i<=end; i++ {
			b := bars[i]
			if b.Low < minLow { minLow=b.Low }
			spread := b.High-b.Low
			testSpread := testBar.High-testBar.Low
			bull := b.Close>b.Open
			bear := b.Close<b.Open
			if b.Close>bars[e.BarIndex].High { o.CloseAboveSignalHigh=true }
			if bull && spread>testSpread && v2ClosePosition(b)>=0.60 { o.UpSpreadExpansion=true }
			if bull && b.Volume>testBar.Volume { o.UpVolumeExpansion=true }
			if bear && b.Volume<testBar.Volume { o.DryPullback=true }
			if mid>0 && b.Close>mid { o.MidpointReclaim=true }
		}
		o.HigherLow = minLow>testBar.Low
		for _, v := range []bool{o.CloseAboveSignalHigh,o.HigherLow,o.UpSpreadExpansion,o.UpVolumeExpansion,o.DryPullback,o.MidpointReclaim} {
			if v { o.FeatureCount++ }
		}

		idx, entry := v3ConfirmationEntry(bars,e.BarIndex,8)
		if idx>=0 && entry>0 {
			stop := e.SpringLow-0.75*e.SpringATR
			if stop>0 && stop<entry {
				o.Confirmed=true
				o.ConfirmedR3=simulateRTrade(bars,idx,64,entry,stop,3)
			}
		}
		obs=append(obs,o)
	}
	return obs
}

func summarizeV4Candidate(name string, obs []V3ConfirmationObservation) V4CandidateResult {
	r := V4CandidateResult{Name:name,Signals:len(obs)}
	if len(obs)==0 { return r }
	var wins int
	for _, o := range obs {
		r.AvgReturn16H += o.Return16H
		if o.Return16H > 0 { wins++ }
		if o.Confirmed {
			r.ConfirmedTrades++
			r.AvgConfirmedR3 += o.ConfirmedR3
		}
	}
	n := float64(len(obs))
	r.WinRate16H = float64(wins)/n*100
	r.AvgReturn16H /= n
	r.ConfirmRate = float64(r.ConfirmedTrades)/n*100
	if r.ConfirmedTrades > 0 { r.AvgConfirmedR3 /= float64(r.ConfirmedTrades) }
	return r
}
