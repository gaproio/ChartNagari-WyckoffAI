package indicator

import (
	"math"
	"testing"
	"time"

	"github.com/Ju571nK/Chatter/pkg/models"
)

func TestComputeProducesSameIndicatorsForAscendingAndDescendingCandles(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	asc := make([]models.OHLCV, 60)
	for i := range asc {
		closePrice := 100.0 + float64(i)*0.75 + float64(i%5)*0.1
		asc[i] = models.OHLCV{
			Symbol:    "BTCUSDT",
			Timeframe: "1H",
			OpenTime:  start.Add(time.Duration(i) * time.Hour),
			Open:      closePrice - 0.25,
			High:      closePrice + 1.0,
			Low:       closePrice - 1.0,
			Close:     closePrice,
			Volume:    1000 + float64(i*10),
		}
	}

	desc := make([]models.OHLCV, len(asc))
	for i, candle := range asc {
		desc[len(asc)-1-i] = candle
	}

	gotAsc := Compute(map[string][]models.OHLCV{"1H": asc})
	gotDesc := Compute(map[string][]models.OHLCV{"1H": desc})

	for _, key := range []string{"1H:EMA_20", "1H:RSI_14", "1H:ATR_14", "1H:VOLUME_MA_20"} {
		a, aOK := gotAsc[key]
		d, dOK := gotDesc[key]
		if !aOK || !dOK {
			t.Fatalf("missing indicator %s: ascending=%v descending=%v", key, aOK, dOK)
		}
		if math.Abs(a-d) > 1e-9 {
			t.Fatalf("%s differs by input order: ascending=%v descending=%v", key, a, d)
		}
	}
}
