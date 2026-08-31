package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// BTCVolatilityBucket studies the frozen BTCUSDT/15M trades after normalizing
// volatility to each trade's own era. It is descriptive only and does not alter
// the detector, B confirmation, entry, stop, target, or holding horizon.
type BTCVolatilityBucket struct {
	Name             string  `json:"name"`
	Trades           int     `json:"trades"`
	TargetHits       int     `json:"target_hits"`
	StopHits         int     `json:"stop_hits"`
	TimeExits        int     `json:"time_exits"`
	NetWinRate       float64 `json:"net_win_rate"`
	AvgATRPercent    float64 `json:"avg_atr_pct"`
	AvgVolPercentile float64 `json:"avg_vol_percentile"`
	AvgRiskPct       float64 `json:"avg_risk_pct"`
	AvgRiskATR       float64 `json:"avg_risk_atr_multiple"`
	AvgNetR          float64 `json:"avg_net_r"`
	AvgReturn16H     float64 `json:"avg_return_16h_pct"`
	AvgMFE16H        float64 `json:"avg_mfe_16h_pct"`
	AvgMAE16H        float64 `json:"avg_mae_16h_pct"`
}

type btcVolatilityObservation struct {
	trade      BTCMasterTrade
	atrPct     float64
	percentile float64
	riskATR    float64
}

// ValidateBTCVolatilityDiagnostic classifies each frozen trade by the causal
// percentile of 14-bar ATR% at the B decision candle versus the preceding 30
// days of 15M candles. LOW/MID/HIGH are fixed terciles (<=33, 33-67, >67), not
// thresholds selected from trade outcomes. Using a rolling percentile makes the
// comparison meaningful across very different BTC price/volatility eras.
//
// The same causal observations are also grouped into deliberately broad
// structural-risk zones measured in ATR multiples (<=3, 3-6, >6). These are
// descriptive diagnostics only; they do not create a max-risk rule.
func ValidateBTCVolatilityDiagnostic(input []models.OHLCV, report BTCMasterReport) []BTCVolatilityBucket {
	bars := v2Chronological(input)
	atr := v3ATRSeries(bars, 14)
	indexByTime := make(map[int64]int, len(bars))
	for i := range bars {
		indexByTime[bars[i].OpenTime.Unix()] = i
	}

	volGroups := map[string][]btcVolatilityObservation{
		"VOL LOW <=33P":  {},
		"VOL MID 33-67P": {},
		"VOL HIGH >67P":  {},
	}
	riskATRGroups := map[string][]btcVolatilityObservation{
		"RISKATR <=3": {},
		"RISKATR 3-6": {},
		"RISKATR >6":  {},
	}
	const lookback = 30 * 24 * 4 // 30 days of 15M bars

	for _, t := range report.Trades {
		entryIdx, ok := indexByTime[t.EntryTime]
		if !ok || entryIdx < 2 { continue }
		decisionIdx := entryIdx - 1 // fully known when next-open entry is placed
		if decisionIdx < 14 || atr[decisionIdx] <= 0 || bars[decisionIdx].Close <= 0 { continue }
		atrPct := atr[decisionIdx] / bars[decisionIdx].Close * 100
		start := decisionIdx - lookback
		if start < 14 { start = 14 }
		if decisionIdx-start < 96 { continue }

		valid, le := 0, 0
		for i := start; i < decisionIdx; i++ {
			if atr[i] <= 0 || bars[i].Close <= 0 { continue }
			v := atr[i] / bars[i].Close * 100
			valid++
			if v <= atrPct { le++ }
		}
		if valid == 0 { continue }
		percentile := float64(le) / float64(valid) * 100
		riskATR := 0.0
		if atrPct > 0 { riskATR = t.RiskPct / atrPct }
		o := btcVolatilityObservation{trade:t, atrPct:atrPct, percentile:percentile, riskATR:riskATR}

		volName := "VOL HIGH >67P"
		if percentile <= 33 {
			volName = "VOL LOW <=33P"
		} else if percentile <= 67 {
			volName = "VOL MID 33-67P"
		}
		volGroups[volName] = append(volGroups[volName], o)

		riskName := "RISKATR >6"
		if riskATR <= 3 {
			riskName = "RISKATR <=3"
		} else if riskATR <= 6 {
			riskName = "RISKATR 3-6"
		}
		riskATRGroups[riskName] = append(riskATRGroups[riskName], o)
	}

	out := make([]BTCVolatilityBucket, 0, 6)
	for _, name := range []string{"VOL LOW <=33P", "VOL MID 33-67P", "VOL HIGH >67P"} {
		out = append(out, summarizeBTCVolatilityBucket(name, volGroups[name]))
	}
	for _, name := range []string{"RISKATR <=3", "RISKATR 3-6", "RISKATR >6"} {
		out = append(out, summarizeBTCVolatilityBucket(name, riskATRGroups[name]))
	}
	return out
}

func summarizeBTCVolatilityBucket(name string, obs []btcVolatilityObservation) BTCVolatilityBucket {
	r := BTCVolatilityBucket{Name: name, Trades: len(obs)}
	if len(obs) == 0 { return r }
	wins := 0
	for _, o := range obs {
		t := o.trade
		switch t.Outcome {
		case "TARGET": r.TargetHits++
		case "STOP": r.StopHits++
		case "TIME": r.TimeExits++
		}
		if t.NetR > 0 { wins++ }
		r.AvgATRPercent += o.atrPct
		r.AvgVolPercentile += o.percentile
		r.AvgRiskPct += t.RiskPct
		r.AvgRiskATR += o.riskATR
		r.AvgNetR += t.NetR
		r.AvgReturn16H += t.Return16H
		r.AvgMFE16H += t.MFE16H
		r.AvgMAE16H += t.MAE16H
	}
	n := float64(len(obs))
	r.NetWinRate = float64(wins) / n * 100
	r.AvgATRPercent /= n
	r.AvgVolPercentile /= n
	r.AvgRiskPct /= n
	r.AvgRiskATR /= n
	r.AvgNetR /= n
	r.AvgReturn16H /= n
	r.AvgMFE16H /= n
	r.AvgMAE16H /= n
	return r
}
