package wyckoff

import "github.com/Ju571nK/Chatter/pkg/models"

// BTCRiskATRBucket studies the exact frozen BTCUSDT/15M trades after expressing
// structural entry-to-stop distance in multiples of the causal 14-bar ATR known
// at the B decision candle. It is descriptive only and does not add a filter.
type BTCRiskATRBucket struct {
	Name         string  `json:"name"`
	Trades       int     `json:"trades"`
	TargetHits   int     `json:"target_hits"`
	StopHits     int     `json:"stop_hits"`
	TimeExits    int     `json:"time_exits"`
	NetWinRate   float64 `json:"net_win_rate"`
	AvgRiskATR   float64 `json:"avg_risk_atr_multiple"`
	AvgRiskPct   float64 `json:"avg_risk_pct"`
	AvgATRPercent float64 `json:"avg_atr_pct"`
	AvgNetR      float64 `json:"avg_net_r"`
	AvgReturn16H float64 `json:"avg_return_16h_pct"`
	AvgMFE16H    float64 `json:"avg_mfe_16h_pct"`
	AvgMAE16H    float64 `json:"avg_mae_16h_pct"`
}

type btcRiskATRObservation struct {
	trade   BTCMasterTrade
	riskATR float64
	atrPct  float64
}

// ValidateBTCRiskATRDiagnostic groups frozen trades into deliberately broad,
// predeclared stop-distance zones: <=3 ATR, 3-6 ATR, and >6 ATR. These round
// zones are diagnostic, not thresholds selected from trade outcomes.
func ValidateBTCRiskATRDiagnostic(input []models.OHLCV, report BTCMasterReport) []BTCRiskATRBucket {
	bars := v2Chronological(input)
	atr := v3ATRSeries(bars, 14)
	indexByTime := make(map[int64]int, len(bars))
	for i := range bars {
		indexByTime[bars[i].OpenTime.Unix()] = i
	}

	groups := map[string][]btcRiskATRObservation{
		"RISK <=3 ATR": {},
		"RISK 3-6 ATR": {},
		"RISK >6 ATR":  {},
	}

	for _, t := range report.Trades {
		entryIdx, ok := indexByTime[t.EntryTime]
		if !ok || entryIdx < 2 { continue }
		decisionIdx := entryIdx - 1
		if decisionIdx < 14 || atr[decisionIdx] <= 0 || bars[decisionIdx].Close <= 0 { continue }
		atrPct := atr[decisionIdx] / bars[decisionIdx].Close * 100
		if atrPct <= 0 { continue }
		riskATR := t.RiskPct / atrPct
		name := "RISK >6 ATR"
		if riskATR <= 3 {
			name = "RISK <=3 ATR"
		} else if riskATR <= 6 {
			name = "RISK 3-6 ATR"
		}
		groups[name] = append(groups[name], btcRiskATRObservation{trade:t, riskATR:riskATR, atrPct:atrPct})
	}

	order := []string{"RISK <=3 ATR", "RISK 3-6 ATR", "RISK >6 ATR"}
	out := make([]BTCRiskATRBucket, 0, len(order))
	for _, name := range order {
		out = append(out, summarizeBTCRiskATRBucket(name, groups[name]))
	}
	return out
}

func summarizeBTCRiskATRBucket(name string, obs []btcRiskATRObservation) BTCRiskATRBucket {
	r := BTCRiskATRBucket{Name:name, Trades:len(obs)}
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
		r.AvgRiskATR += o.riskATR
		r.AvgRiskPct += t.RiskPct
		r.AvgATRPercent += o.atrPct
		r.AvgNetR += t.NetR
		r.AvgReturn16H += t.Return16H
		r.AvgMFE16H += t.MFE16H
		r.AvgMAE16H += t.MAE16H
	}
	n := float64(len(obs))
	r.NetWinRate = float64(wins)/n*100
	r.AvgRiskATR /= n
	r.AvgRiskPct /= n
	r.AvgATRPercent /= n
	r.AvgNetR /= n
	r.AvgReturn16H /= n
	r.AvgMFE16H /= n
	r.AvgMAE16H /= n
	return r
}
