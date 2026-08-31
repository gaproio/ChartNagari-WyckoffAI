package wyckoff

import (
	"sort"
	"time"

	"github.com/Ju571nK/Chatter/pkg/models"
)

// BTCMasterConfig keeps the BTC study reproducible. Costs are per side.
// The regime label is descriptive only and is not used as a signal filter.
type BTCMasterConfig struct {
	FeeBpsPerSide      float64 `json:"fee_bps_per_side"`
	SlippageBpsPerSide float64 `json:"slippage_bps_per_side"`
	TargetR            float64 `json:"target_r"`
	RegimeLookbackBars int     `json:"regime_lookback_bars"`
	RegimeThresholdPct float64 `json:"regime_threshold_pct"`
}

func DefaultBTCMasterConfig() BTCMasterConfig {
	return BTCMasterConfig{
		FeeBpsPerSide:      10,
		SlippageBpsPerSide: 5,
		TargetR:            3,
		RegimeLookbackBars: 30 * 24 * 4, // 30 days of 15M candles
		RegimeThresholdPct: 10,
	}
}

type BTCMasterTrade struct {
	EntryTime      int64   `json:"entry_time"`
	EntryPrice     float64 `json:"entry_price"`
	StopPrice      float64 `json:"stop_price"`
	RiskPct        float64 `json:"risk_pct"`
	GrossR         float64 `json:"gross_r"`
	NetR           float64 `json:"net_r_after_costs"`
	Outcome        string  `json:"outcome"`
	ExitBars       int     `json:"exit_bars"`
	Return16H      float64 `json:"return_16h_pct"`
	MFE16H         float64 `json:"mfe_16h_pct"`
	MAE16H         float64 `json:"mae_16h_pct"`
	Regime         string  `json:"regime_30d"`
	RegimeReturn30 float64 `json:"regime_return_30d_pct"`
}

type BTCMasterBucket struct {
	Name          string  `json:"name"`
	Trades        int     `json:"trades"`
	TargetHits    int     `json:"target_hits"`
	StopHits      int     `json:"stop_hits"`
	TimeExits     int     `json:"time_exits"`
	NetWinRate    float64 `json:"net_win_rate"`
	AvgGrossR     float64 `json:"avg_gross_r"`
	AvgNetR       float64 `json:"avg_net_r"`
	AvgReturn16H  float64 `json:"avg_return_16h_pct"`
	AvgMFE16H     float64 `json:"avg_mfe_16h_pct"`
	AvgMAE16H     float64 `json:"avg_mae_16h_pct"`
	AvgRiskPct    float64 `json:"avg_risk_pct"`
	AvgExitBars   float64 `json:"avg_exit_bars"`
}

type BTCMasterReport struct {
	Config  BTCMasterConfig   `json:"config"`
	Overall BTCMasterBucket   `json:"overall"`
	ByYear  []BTCMasterBucket `json:"by_year"`
	ByRegime []BTCMasterBucket `json:"by_regime"`
	Trades  []BTCMasterTrade  `json:"trades"`
}

// ValidateBTCMaster studies only the frozen BTC B-profile:
// V3 detector -> midpoint + prospective higher-low confirmation -> next-bar
// open -> post-Test stop. No detector/entry parameter is optimized here.
func ValidateBTCMaster(input []models.OHLCV, validation V3ValidationSummary, cfg BTCMasterConfig) BTCMasterReport {
	bars := v2Chronological(input)
	if cfg.TargetR <= 0 { cfg.TargetR = 3 }
	if cfg.RegimeLookbackBars <= 0 { cfg.RegimeLookbackBars = 30 * 24 * 4 }
	if cfg.RegimeThresholdPct <= 0 { cfg.RegimeThresholdPct = 10 }
	out := BTCMasterReport{Config: cfg}

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
		riskPct := risk / entry * 100

		grossR, outcome, exitIdx := simulateBTCMasterOutcome(bars, execIdx, 64, entry, stop, cfg.TargetR)
		costFraction := 2 * (cfg.FeeBpsPerSide + cfg.SlippageBpsPerSide) / 10000.0
		riskFraction := risk / entry
		costR := 0.0
		if riskFraction > 0 { costR = costFraction / riskFraction }
		netR := grossR - costR

		maxHigh, minLow := entry, entry
		for j := execIdx; j <= execIdx+64; j++ {
			if bars[j].High > maxHigh { maxHigh = bars[j].High }
			if bars[j].Low < minLow { minLow = bars[j].Low }
		}
		regime, regimeRet := btc30DRegime(bars, execIdx, cfg.RegimeLookbackBars, cfg.RegimeThresholdPct)
		out.Trades = append(out.Trades, BTCMasterTrade{
			EntryTime: bars[execIdx].OpenTime.Unix(), EntryPrice: entry, StopPrice: stop,
			RiskPct: riskPct, GrossR: grossR, NetR: netR, Outcome: outcome,
			ExitBars: exitIdx-execIdx, Return16H: pctReturn(entry,bars[execIdx+64].Close),
			MFE16H: pctReturn(entry,maxHigh), MAE16H: pctReturn(entry,minLow),
			Regime: regime, RegimeReturn30: regimeRet,
		})
	}

	out.Overall = summarizeBTCMaster("ALL", out.Trades)
	years := map[int][]BTCMasterTrade{}
	regimes := map[string][]BTCMasterTrade{}
	for _, t := range out.Trades {
		y := time.Unix(t.EntryTime,0).UTC().Year()
		years[y] = append(years[y], t)
		regimes[t.Regime] = append(regimes[t.Regime], t)
	}
	keys := make([]int,0,len(years))
	for y := range years { keys = append(keys,y) }
	sort.Ints(keys)
	for _, y := range keys { out.ByYear = append(out.ByYear, summarizeBTCMaster(time.Date(y,1,1,0,0,0,0,time.UTC).Format("2006"), years[y])) }
	for _, name := range []string{"BEAR_30D","SIDEWAYS_30D","BULL_30D","UNKNOWN"} {
		if ts,ok := regimes[name]; ok { out.ByRegime = append(out.ByRegime, summarizeBTCMaster(name,ts)) }
	}
	return out
}

func simulateBTCMasterOutcome(bars []models.OHLCV, entryIndex, maxBars int, entry, stop, targetR float64) (float64,string,int) {
	risk := entry-stop
	if risk <= 0 || entryIndex < 0 || entryIndex >= len(bars) { return 0,"INVALID",entryIndex }
	target := entry + targetR*risk
	end := entryIndex+maxBars
	if end >= len(bars) { end = len(bars)-1 }
	for j:=entryIndex; j<=end; j++ {
		stopHit := bars[j].Low <= stop
		targetHit := bars[j].High >= target
		if stopHit { return -1,"STOP",j }
		if targetHit { return targetR,"TARGET",j }
	}
	return (bars[end].Close-entry)/risk,"TIME",end
}

func btc30DRegime(bars []models.OHLCV, entryIndex, lookbackBars int, thresholdPct float64) (string,float64) {
	if entryIndex < 0 || entryIndex >= len(bars) || lookbackBars <= 0 || entryIndex-lookbackBars < 0 { return "UNKNOWN",0 }
	past := bars[entryIndex-lookbackBars].Close
	now := bars[entryIndex].Open
	if past <= 0 || now <= 0 { return "UNKNOWN",0 }
	ret := pctReturn(past,now)
	if ret > thresholdPct { return "BULL_30D",ret }
	if ret < -thresholdPct { return "BEAR_30D",ret }
	return "SIDEWAYS_30D",ret
}

func summarizeBTCMaster(name string, trades []BTCMasterTrade) BTCMasterBucket {
	b := BTCMasterBucket{Name:name, Trades:len(trades)}
	if len(trades)==0 { return b }
	wins := 0
	for _,t := range trades {
		switch t.Outcome { case "TARGET": b.TargetHits++; case "STOP": b.StopHits++; case "TIME": b.TimeExits++ }
		if t.NetR > 0 { wins++ }
		b.AvgGrossR += t.GrossR
		b.AvgNetR += t.NetR
		b.AvgReturn16H += t.Return16H
		b.AvgMFE16H += t.MFE16H
		b.AvgMAE16H += t.MAE16H
		b.AvgRiskPct += t.RiskPct
		b.AvgExitBars += float64(t.ExitBars)
	}
	n := float64(len(trades))
	b.NetWinRate = float64(wins)/n*100
	b.AvgGrossR/=n; b.AvgNetR/=n; b.AvgReturn16H/=n; b.AvgMFE16H/=n; b.AvgMAE16H/=n; b.AvgRiskPct/=n; b.AvgExitBars/=n
	return b
}
