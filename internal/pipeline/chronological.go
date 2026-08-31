package pipeline

import "github.com/Ju571nK/Chatter/pkg/models"

// chronologicalBars returns a copy of each timeframe slice in oldest-first
// order. GetOHLCV returns newest-first slices, but indicator and methodology
// code is written for chronological series and treats the last element as the
// current candle. The source map and its slices are never mutated.
func chronologicalBars(src map[string][]models.OHLCV) map[string][]models.OHLCV {
	out := make(map[string][]models.OHLCV, len(src))
	for tf, bars := range src {
		reversed := make([]models.OHLCV, len(bars))
		for i, bar := range bars {
			reversed[len(bars)-1-i] = bar
		}
		out[tf] = reversed
	}
	return out
}
