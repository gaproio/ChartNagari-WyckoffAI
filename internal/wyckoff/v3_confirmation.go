package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// V3ConfirmationObservation measures post-Test demand features without using
// them to accept/reject a V3 signal. This is research-only: the detector and
// TradeScore remain unchanged.
type V3ConfirmationObservation struct {
	BarIndex             int     `json:"bar_index"`
	TradeScore           float64 `json:"trade_score"`
	FeatureCount         int     `json:"feature_count"`
	CloseAboveSignalHigh bool    `json:"close_above_signal_high"`
	HigherLow            bool    `json:"higher_low"`
	UpSpreadExpansion    bool    `json:"up_spread_expansion"`
	UpVolumeExpansion    bool    `json:"up_volume_expansion"`
	DryPullback          bool    `json:"dry_pullback"`
	MidpointReclaim      bool    `json:"midpoint_reclaim"`
	Return16H            float64 `json:"return_16h_pct"`
	Confirmed            bool    `json:"confirmed"`
	ConfirmedR3          float64 `json:"confirmed_r3"`
}

type V3ConfirmationBucket struct {
	Name          string  `json:"name"`
	Signals       int     `json:"signals"`
	AvgFeatures   float64 `json:"avg_features"`
	WinRate16H    float64 `json:"win_rate_16h"`
	AvgReturn16H  float64 `json:"avg_return_16h_pct"`
	ConfirmRate   float64 `json:"confirm_rate"`
	AvgConfirmedR3 float64 `json:"avg_confirmed_r3"`
}

type V3ConfirmationReport struct {
	Overall   V3ConfirmationBucket   `json:"overall"`
	ByFeature []V3ConfirmationBucket `json:"by_feature_count"`
	Features  []V3ConfirmationBucket `json:"individual_features"`
}

// ValidateV3Confirmation inspects only the eight candles AFTER each existing
// V3 signal. No observation here changes which signals exist.
func ValidateV3Confirmation(input []models.OHLCV, validation V3ValidationSummary) V3ConfirmationReport {
	bars := v2Chronological(input)
	obs := make([]V3ConfirmationObservation, 0, len(validation.Events))
	for _, e := range validation.Events {
		if e.BarIndex < 199 || e.BarIndex+64 >= len(bars) { continue }
		a := AnalyzeV3Foundation(validation.Symbol, "15M", bars[e.BarIndex-199:e.BarIndex+1])
		if !a.HasTest || !a.HasSpring { continue }

		testIdx := -1
		for _, ev := range a.Events { if ev.Type == V3EventTest { testIdx = ev.BarIndex; break } }
		if testIdx < 0 { continue }
		globalTest := e.BarIndex-199+testIdx
		if globalTest < 0 || globalTest >= len(bars) { continue }
		testBar := bars[globalTest]
		mid := (a.Range.Support+a.Range.Resistance)/2
		end := e.BarIndex+8; if end >= len(bars) { end=len(bars)-1 }

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
		for _,v:=range []bool{o.CloseAboveSignalHigh,o.HigherLow,o.UpSpreadExpansion,o.UpVolumeExpansion,o.DryPullback,o.MidpointReclaim} { if v { o.FeatureCount++ } }

		idx,entry := v3ConfirmationEntry(bars,e.BarIndex,8)
		if idx>=0 && entry>0 {
			stop:=e.SpringLow-0.75*e.SpringATR
			if stop>0 && stop<entry { o.Confirmed=true; o.ConfirmedR3=simulateRTrade(bars,idx,64,entry,stop,3) }
		}
		obs=append(obs,o)
	}

	r := V3ConfirmationReport{Overall:summarizeConfirmation("ALL",obs)}
	for _,x:=range []struct{name string; min,max int}{{"FEATURES 0-2",0,2},{"FEATURES 3-4",3,4},{"FEATURES 5-6",5,6}} {
		r.ByFeature=append(r.ByFeature,summarizeConfirmation(x.name,filterConfirmation(obs,func(o V3ConfirmationObservation) bool{return o.FeatureCount>=x.min&&o.FeatureCount<=x.max})))
	}
	features:=[]struct{name string; keep func(V3ConfirmationObservation)bool}{
		{"ABOVE SIGNAL HIGH",func(o V3ConfirmationObservation)bool{return o.CloseAboveSignalHigh}},
		{"HIGHER LOW",func(o V3ConfirmationObservation)bool{return o.HigherLow}},
		{"UP SPREAD EXPAND",func(o V3ConfirmationObservation)bool{return o.UpSpreadExpansion}},
		{"UP VOLUME EXPAND",func(o V3ConfirmationObservation)bool{return o.UpVolumeExpansion}},
		{"DRY PULLBACK",func(o V3ConfirmationObservation)bool{return o.DryPullback}},
		{"MIDPOINT RECLAIM",func(o V3ConfirmationObservation)bool{return o.MidpointReclaim}},
	}
	for _,f:=range features { r.Features=append(r.Features,summarizeConfirmation(f.name,filterConfirmation(obs,f.keep))) }
	return r
}

func summarizeConfirmation(name string, obs []V3ConfirmationObservation) V3ConfirmationBucket {
	b:=V3ConfirmationBucket{Name:name,Signals:len(obs)}; if len(obs)==0{return b}
	var wins,confirmed int
	for _,o:=range obs {
		b.AvgFeatures+=float64(o.FeatureCount); b.AvgReturn16H+=o.Return16H
		if o.Return16H>0{wins++}; if o.Confirmed{confirmed++; b.AvgConfirmedR3+=o.ConfirmedR3}
	}
	n:=float64(len(obs)); b.AvgFeatures/=n; b.AvgReturn16H/=n; b.WinRate16H=float64(wins)/n*100; b.ConfirmRate=float64(confirmed)/n*100
	if confirmed>0 { b.AvgConfirmedR3/=float64(confirmed) }
	return b
}

func filterConfirmation(obs []V3ConfirmationObservation, keep func(V3ConfirmationObservation)bool) []V3ConfirmationObservation {
	out:=make([]V3ConfirmationObservation,0); for _,o:=range obs{if keep(o){out=append(out,o)}}; return out
}
