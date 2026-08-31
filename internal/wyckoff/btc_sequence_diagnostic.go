package wyckoff

import "sort"

// BTCSequenceDiagnostic measures path dependence of the exact frozen BTCUSDT/15M
// trade sequence. It is descriptive only: no entries, exits, filters, sizing, or
// detector rules are changed.
type BTCSequenceDiagnostic struct {
	Trades                    int     `json:"trades"`
	TotalNetR                 float64 `json:"total_net_r"`
	MedianNetR                float64 `json:"median_net_r"`
	ProfitFactor              float64 `json:"profit_factor"`
	MaxDrawdownR              float64 `json:"max_drawdown_r"`
	MaxConsecutiveLosses      int     `json:"max_consecutive_losses"`
	LargestWinnerR            float64 `json:"largest_winner_r"`
	LargestWinnerSharePct     float64 `json:"largest_winner_share_pct_of_total_net"`
	Top3WinnersR              float64 `json:"top3_winners_r"`
	Top3WinnersSharePct       float64 `json:"top3_winners_share_pct_of_total_net"`
	PositiveTrades            int     `json:"positive_trades"`
	NegativeTrades            int     `json:"negative_trades"`
	FlatTrades                int     `json:"flat_trades"`
}

// ValidateBTCSequenceDiagnostic evaluates the frozen trade list in chronological
// order. Max drawdown is measured on cumulative NetR, so it describes the actual
// historical sequence rather than a shuffled or Monte Carlo path.
func ValidateBTCSequenceDiagnostic(report BTCMasterReport) BTCSequenceDiagnostic {
	r := BTCSequenceDiagnostic{Trades: len(report.Trades)}
	if len(report.Trades) == 0 { return r }

	values := make([]float64, 0, len(report.Trades))
	winners := make([]float64, 0, len(report.Trades))
	grossProfit := 0.0
	grossLoss := 0.0
	cumulative := 0.0
	peak := 0.0
	lossStreak := 0

	for _, t := range report.Trades {
		v := t.NetR
		values = append(values, v)
		r.TotalNetR += v

		switch {
		case v > 0:
			r.PositiveTrades++
			grossProfit += v
			winners = append(winners, v)
			lossStreak = 0
		case v < 0:
			r.NegativeTrades++
			grossLoss += -v
			lossStreak++
			if lossStreak > r.MaxConsecutiveLosses { r.MaxConsecutiveLosses = lossStreak }
		default:
			r.FlatTrades++
			lossStreak = 0
		}

		cumulative += v
		if cumulative > peak { peak = cumulative }
		dd := peak - cumulative
		if dd > r.MaxDrawdownR { r.MaxDrawdownR = dd }
	}

	sort.Float64s(values)
	n := len(values)
	if n%2 == 1 {
		r.MedianNetR = values[n/2]
	} else {
		r.MedianNetR = (values[n/2-1] + values[n/2]) / 2
	}

	if grossLoss > 0 { r.ProfitFactor = grossProfit / grossLoss }

	sort.Sort(sort.Reverse(sort.Float64Slice(winners)))
	if len(winners) > 0 { r.LargestWinnerR = winners[0] }
	limit := 3
	if len(winners) < limit { limit = len(winners) }
	for i := 0; i < limit; i++ { r.Top3WinnersR += winners[i] }

	if r.TotalNetR != 0 {
		r.LargestWinnerSharePct = r.LargestWinnerR / r.TotalNetR * 100
		r.Top3WinnersSharePct = r.Top3WinnersR / r.TotalNetR * 100
	}
	return r
}
