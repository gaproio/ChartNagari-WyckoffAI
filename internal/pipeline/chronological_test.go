package pipeline

import (
	"testing"
	"time"

	"github.com/Ju571nK/Chatter/pkg/models"
)

func TestChronologicalBars_ReversesWithoutMutatingSource(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	newest := models.OHLCV{Symbol: "BTCUSDT", Timeframe: "1H", OpenTime: now, Close: 300}
	middle := models.OHLCV{Symbol: "BTCUSDT", Timeframe: "1H", OpenTime: now.Add(-time.Hour), Close: 200}
	oldest := models.OHLCV{Symbol: "BTCUSDT", Timeframe: "1H", OpenTime: now.Add(-2 * time.Hour), Close: 100}

	src := map[string][]models.OHLCV{"1H": {newest, middle, oldest}}
	got := chronologicalBars(src)

	if got["1H"][0].Close != 100 || got["1H"][1].Close != 200 || got["1H"][2].Close != 300 {
		t.Fatalf("chronologicalBars() = closes [%v %v %v], want [100 200 300]",
			got["1H"][0].Close, got["1H"][1].Close, got["1H"][2].Close)
	}
	if src["1H"][0].Close != 300 || src["1H"][2].Close != 100 {
		t.Fatalf("chronologicalBars mutated source: closes [%v ... %v]", src["1H"][0].Close, src["1H"][2].Close)
	}
}
