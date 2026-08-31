package engine

import (
	"testing"
	"time"

	"github.com/Ju571nK/Chatter/pkg/models"
)

func TestChronologicalTimeframesReversesDescendingCandlesWithoutMutatingSource(t *testing.T) {
	now := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	newest := models.OHLCV{OpenTime: now, Close: 300}
	middle := models.OHLCV{OpenTime: now.Add(-time.Hour), Close: 200}
	oldest := models.OHLCV{OpenTime: now.Add(-2 * time.Hour), Close: 100}

	src := []models.OHLCV{newest, middle, oldest}
	ctx := models.AnalysisContext{Timeframes: map[string][]models.OHLCV{"1H": src}}
	got := chronologicalTimeframes(ctx)

	bars := got.Timeframes["1H"]
	if bars[0].Close != 100 || bars[1].Close != 200 || bars[2].Close != 300 {
		t.Fatalf("normalized closes = [%v %v %v], want [100 200 300]", bars[0].Close, bars[1].Close, bars[2].Close)
	}
	if src[0].Close != 300 || src[2].Close != 100 {
		t.Fatalf("source slice mutated: [%v ... %v]", src[0].Close, src[2].Close)
	}
}
