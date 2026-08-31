package wyckoff

// BTCRiskBucket is descriptive research for the already-frozen BTC 15M trades.
// It does not filter or alter entries. Buckets are deliberately broad so we can
// see whether structural stop distance is associated with different outcomes
// without parameter-mining a precise cutoff.
type BTCRiskBucket struct {
	Name          string  `json:"name"`
	Trades        int     `json:"trades"`
	TargetHits    int     `json:"target_hits"`
	StopHits      int     `json:"stop_hits"`
	TimeExits     int     `json:"time_exits"`
	NetWinRate    float64 `json:"net_win_rate"`
	AvgRiskPct    float64 `json:"avg_risk_pct"`
	AvgGrossR     float64 `json:"avg_gross_r"`
	AvgNetR       float64 `json:"avg_net_r"`
	AvgCostR      float64 `json:"avg_cost_r"`
	AvgReturn16H  float64 `json:"avg_return_16h_pct"`
	AvgMFE16H     float64 `json:"avg_mfe_16h_pct"`
	AvgMAE16H     float64 `json:"avg_mae_16h_pct"`
}

// ValidateBTCRiskDiagnostic groups the exact BTCMasterReport trades into fixed
// broad stop-distance buckets. It is descriptive only; no max-risk rule is
// created or applied here.
func ValidateBTCRiskDiagnostic(report BTCMasterReport) []BTCRiskBucket {
	groups := map[string][]BTCMasterTrade{
		"RISK <=2%": {},
		"RISK 2-4%": {},
		"RISK 4-6%": {},
		"RISK >6%":  {},
	}
	for _, t := range report.Trades {
		name := "RISK >6%"
		switch {
		case t.RiskPct <= 2:
			name = "RISK <=2%"
		case t.RiskPct <= 4:
			name = "RISK 2-4%"
		case t.RiskPct <= 6:
			name = "RISK 4-6%"
		}
		groups[name] = append(groups[name], t)
	}

	order := []string{"RISK <=2%", "RISK 2-4%", "RISK 4-6%", "RISK >6%"}
	out := make([]BTCRiskBucket, 0, len(order))
	for _, name := range order {
		out = append(out, summarizeBTCRiskBucket(name, groups[name]))
	}
	return out
}

func summarizeBTCRiskBucket(name string, trades []BTCMasterTrade) BTCRiskBucket {
	r := BTCRiskBucket{Name: name, Trades: len(trades)}
	if len(trades) == 0 { return r }
	wins := 0
	for _, t := range trades {
		switch t.Outcome {
		case "TARGET": r.TargetHits++
		case "STOP": r.StopHits++
		case "TIME": r.TimeExits++
		}
		if t.NetR > 0 { wins++ }
		r.AvgRiskPct += t.RiskPct
		r.AvgGrossR += t.GrossR
		r.AvgNetR += t.NetR
		r.AvgCostR += t.GrossR - t.NetR
		r.AvgReturn16H += t.Return16H
		r.AvgMFE16H += t.MFE16H
		r.AvgMAE16H += t.MAE16H
	}
	n := float64(len(trades))
	r.NetWinRate = float64(wins) / n * 100
	r.AvgRiskPct /= n
	r.AvgGrossR /= n
	r.AvgNetR /= n
	r.AvgCostR /= n
	r.AvgReturn16H /= n
	r.AvgMFE16H /= n
	r.AvgMAE16H /= n
	return r
}
