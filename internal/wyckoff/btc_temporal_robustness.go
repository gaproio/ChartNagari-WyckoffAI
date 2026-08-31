package wyckoff

import "time"

// BTCTemporalRobustnessBucket summarizes the exact frozen BTCUSDT/15M trades
// inside predeclared calendar blocks. The blocks are fixed before looking at
// outcomes and are descriptive only; they do not create an era filter.
type BTCTemporalRobustnessBucket struct {
	Name                 string  `json:"name"`
	Trades               int     `json:"trades"`
	TotalNetR            float64 `json:"total_net_r"`
	AvgNetR              float64 `json:"avg_net_r"`
	MedianNetR           float64 `json:"median_net_r"`
	ProfitFactor         float64 `json:"profit_factor"`
	MaxDrawdownR         float64 `json:"max_drawdown_r"`
	MaxConsecutiveLosses int     `json:"max_consecutive_losses"`
	NetWinRate           float64 `json:"net_win_rate"`
}

// ValidateBTCTemporalRobustness uses fixed calendar periods:
// 2017-2019, 2020-2022, 2023-2025, and the current partial 2026 period.
// These are not selected from performance and are not tradable filters.
func ValidateBTCTemporalRobustness(report BTCMasterReport) []BTCTemporalRobustnessBucket {
	type period struct {
		name       string
		start, end int
	}
	periods := []period{
		{"2017-2019", 2017, 2019},
		{"2020-2022", 2020, 2022},
		{"2023-2025", 2023, 2025},
		{"2026 PARTIAL", 2026, 2026},
	}

	out := make([]BTCTemporalRobustnessBucket, 0, len(periods))
	for _, p := range periods {
		sub := BTCMasterReport{}
		wins := 0
		for _, t := range report.Trades {
			y := time.Unix(t.EntryTime, 0).UTC().Year()
			if y < p.start || y > p.end { continue }
			sub.Trades = append(sub.Trades, t)
			if t.NetR > 0 { wins++ }
		}
		seq := ValidateBTCSequenceDiagnostic(sub)
		b := BTCTemporalRobustnessBucket{
			Name: p.name,
			Trades: seq.Trades,
			TotalNetR: seq.TotalNetR,
			MedianNetR: seq.MedianNetR,
			ProfitFactor: seq.ProfitFactor,
			MaxDrawdownR: seq.MaxDrawdownR,
			MaxConsecutiveLosses: seq.MaxConsecutiveLosses,
		}
		if b.Trades > 0 {
			b.AvgNetR = b.TotalNetR / float64(b.Trades)
			b.NetWinRate = float64(wins) / float64(b.Trades) * 100
		}
		out = append(out, b)
	}
	return out
}
