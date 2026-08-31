package wyckoff

import (
	"testing"
	"time"

	"github.com/Ju571nK/Chatter/pkg/models"
)

func TestWyckoffSpringRule_EmitsV2LongOn15MStructure(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]models.OHLCV, 0, 48)
	price := 100.0

	for i := 0; i < 20; i++ {
		bars = append(bars, models.OHLCV{Symbol: "BTCUSDT", Timeframe: "15M", OpenTime: start.Add(time.Duration(i) * 15 * time.Minute), Open: price, High: price + 1, Low: price - 1, Close: price, Volume: 100})
	}
	add := func(open, high, low, close, vol float64) {
		i := len(bars)
		bars = append(bars, models.OHLCV{Symbol: "BTCUSDT", Timeframe: "15M", OpenTime: start.Add(time.Duration(i) * 15 * time.Minute), Open: open, High: high, Low: low, Close: close, Volume: vol})
	}

	add(100, 101, 90, 96, 220)
	add(96, 101, 95, 100, 120)
	add(100, 105, 99, 104, 120)
	add(104, 104, 100, 101, 95)
	add(101, 102, 96, 98, 90)
	add(98, 100, 91, 95, 90)
	add(95, 99, 94, 98, 90)
	add(98, 103, 97, 102, 95)
	add(102, 103, 95, 96, 90)
	add(96, 98, 89, 94, 130)
	add(94, 98, 93, 97, 90)
	add(97, 99, 91.5, 95, 80)
	add(95, 102, 94, 101, 100)
	add(101, 108, 100, 107, 140)
	add(107, 108, 104.7, 106, 90)
	add(106, 111, 106, 110, 120)

	rule := &WyckoffSpringRule{}
	sig, err := rule.Analyze(models.AnalysisContext{
		Symbol:     "BTCUSDT",
		Timeframes: map[string][]models.OHLCV{"15M": bars},
		Indicators: map[string]float64{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sig == nil {
		t.Fatal("expected V2 signal, got nil")
	}
	if sig.Rule != "wyckoff_v2_long" {
		t.Fatalf("rule = %q, want wyckoff_v2_long", sig.Rule)
	}
	if sig.Timeframe != "15M" || sig.Direction != "LONG" {
		t.Fatalf("unexpected signal: %+v", sig)
	}
	if sig.Score < 2.34 { // 0.78 structural confidence * 3.0 raw scaling
		t.Fatalf("raw score %.2f too low for V2 trigger", sig.Score)
	}
}
