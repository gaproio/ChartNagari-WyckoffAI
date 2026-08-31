package wyckoff

import "sort"

// BTCCostSensitivityResult rescales only the already-assumed research costs on
// the exact frozen BTCUSDT/15M trades. It is a robustness diagnostic, not an
// estimate of any exchange's actual fee schedule and not an optimization input.
type BTCCostSensitivityResult struct {
	Name           string  `json:"name"`
	CostMultiplier float64 `json:"cost_multiplier"`
	Trades         int     `json:"trades"`
	NetWins        int     `json:"net_wins"`
	NetWinRate     float64 `json:"net_win_rate"`
	TotalNetR      float64 `json:"total_net_r"`
	AvgNetR        float64 `json:"avg_net_r"`
	MedianNetR     float64 `json:"median_net_r"`
	ProfitFactor   float64 `json:"profit_factor"`
}

// ValidateBTCCostSensitivity applies four predeclared stress levels to the exact
// baseline per-trade costR = GrossR-NetR: frictionless, half baseline, baseline,
// and double baseline. No signal or execution rule changes.
func ValidateBTCCostSensitivity(report BTCMasterReport) []BTCCostSensitivityResult {
	tests := []struct {
		name string
		mult float64
	}{
		{"0X FRICTIONLESS", 0},
		{"0.5X COST", 0.5},
		{"1X BASELINE", 1},
		{"2X COST STRESS", 2},
	}

	out := make([]BTCCostSensitivityResult, 0, len(tests))
	for _, tc := range tests {
		r := BTCCostSensitivityResult{Name: tc.name, CostMultiplier: tc.mult, Trades: len(report.Trades)}
		if len(report.Trades) == 0 {
			out = append(out, r)
			continue
		}

		values := make([]float64, 0, len(report.Trades))
		grossProfit := 0.0
		grossLoss := 0.0
		for _, t := range report.Trades {
			baselineCostR := t.GrossR - t.NetR
			net := t.GrossR - tc.mult*baselineCostR
			values = append(values, net)
			r.TotalNetR += net
			if net > 0 {
				r.NetWins++
				grossProfit += net
			} else if net < 0 {
				grossLoss += -net
			}
		}
		r.AvgNetR = r.TotalNetR / float64(r.Trades)
		r.NetWinRate = float64(r.NetWins) / float64(r.Trades) * 100
		if grossLoss > 0 {
			r.ProfitFactor = grossProfit / grossLoss
		}

		sort.Float64s(values)
		n := len(values)
		if n%2 == 1 {
			r.MedianNetR = values[n/2]
		} else {
			r.MedianNetR = (values[n/2-1] + values[n/2]) / 2
		}
		out = append(out, r)
	}
	return out
}
