package wyckoff

import (
	"testing"
	"time"

	"github.com/Ju571nK/Chatter/pkg/models"
)

func TestAnalyzeV2_DetectsAccumulationSequence(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]models.OHLCV, 0, 48)
	price := 100.0

	// Build a quiet pre-range so volume MA and ATR are established.
	for i := 0; i < 20; i++ {
		bars = append(bars, models.OHLCV{Symbol: "BTCUSDT", Timeframe: "15M", OpenTime: start.Add(time.Duration(i) * 15 * time.Minute), Open: price, High: price + 1, Low: price - 1, Close: price, Volume: 100})
	}

	add := func(open, high, low, close, vol float64) {
		i := len(bars)
		bars = append(bars, models.OHLCV{Symbol: "BTCUSDT", Timeframe: "15M", OpenTime: start.Add(time.Duration(i) * 15 * time.Minute), Open: open, High: high, Low: low, Close: close, Volume: vol})
	}

	// SC, AR, then ordinary range action.
	add(100, 101, 90, 96, 220) // SC
	add(96, 101, 95, 100, 120)
	add(100, 105, 99, 104, 120) // AR
	add(104, 104, 100, 101, 95)
	add(101, 102, 96, 98, 90)
	add(98, 100, 91, 95, 90) // ST near support
	add(95, 99, 94, 98, 90)
	add(98, 103, 97, 102, 95)
	add(102, 103, 95, 96, 90)
	add(96, 98, 89, 94, 130) // Spring: below support, closes back above
	add(94, 98, 93, 97, 90)
	add(97, 99, 91.5, 95, 80) // Test: holds above spring low, light volume
	add(95, 102, 94, 101, 100)
	add(101, 108, 100, 107, 140) // SOS above AR resistance
	add(107, 108, 104.7, 106, 90) // LPS near old resistance
	add(106, 111, 106, 110, 120)

	got := AnalyzeV2("BTCUSDT", "15M", bars)
	if !got.HasSpring {
		t.Fatalf("expected Spring, got %+v", got)
	}
	if !got.HasTest {
		t.Fatalf("expected Test, got %+v", got)
	}
	if !got.HasSOS {
		t.Fatalf("expected SOS, got %+v", got)
	}
	if !got.ReadyForLong {
		t.Fatalf("expected ReadyForLong, got %+v", got)
	}
	if got.Phase != V2PhaseD && got.Phase != V2PhaseE {
		t.Fatalf("expected Phase D/E, got %s", got.Phase)
	}
}

func TestAnalyzeV2_AcceptsNewestFirstBars(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]models.OHLCV, 31)
	for i := range bars {
		bars[i] = models.OHLCV{Symbol: "ETHUSDT", Timeframe: "15M", OpenTime: start.Add(time.Duration(i) * 15 * time.Minute), Open: 100, High: 101, Low: 99, Close: 100, Volume: 100}
	}
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}
	got := AnalyzeV2("ETHUSDT", "15M", bars)
	if got.Symbol != "ETHUSDT" || got.Timeframe != "15M" {
		t.Fatalf("unexpected identity: %+v", got)
	}
}
