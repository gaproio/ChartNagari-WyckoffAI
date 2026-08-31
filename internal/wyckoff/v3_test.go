package wyckoff

import (
	"testing"
	"time"

	"github.com/Ju571nK/Chatter/pkg/models"
)

func TestAnalyzeV3FoundationRequiresDowntrendAndDetectsPhaseB(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]models.OHLCV, 0, 80)
	add := func(open, high, low, close, vol float64) {
		i := len(bars)
		bars = append(bars, models.OHLCV{Symbol:"BTCUSDT", Timeframe:"15M", OpenTime:start.Add(time.Duration(i)*15*time.Minute), Open:open, High:high, Low:low, Close:close, Volume:vol})
	}

	price := 112.0
	for i := 0; i < 34; i++ {
		next := price - 0.35
		add(price, price+0.45, next-0.45, next, 100)
		price = next
	}
	add(price, price+0.8, price-1.4, price-0.6, 135)
	price -= 0.6
	add(price, price+0.7, 94.0, 97.0, 230)
	add(97, 100, 96.5, 99, 115)
	add(99, 104.5, 98.5, 103.5, 120)
	add(103.5, 104, 100, 101, 95)
	add(101, 102, 98, 99, 90)
	add(99, 100, 94.8, 97, 85)
	for i := 0; i < 12; i++ { add(97, 101, 96, 100, 95) }

	got := AnalyzeV3Foundation("BTCUSDT", "15M", bars)
	if !got.PriorDowntrend || !got.HasPS || !got.HasSC || !got.HasAR || !got.HasST {
		t.Fatalf("expected complete V3 Phase-B foundation, got %+v", got)
	}
	if got.Phase != V2PhaseB { t.Fatalf("expected Phase B, got %s", got.Phase) }
	if got.StructureConfidence < 0.65 { t.Fatalf("unexpected confidence %.2f", got.StructureConfidence) }
	if got.ReadyForStudy { t.Fatalf("Phase-B-only foundation must not be trade-ready") }
}

func TestAnalyzeV3FoundationDetectsQualitySpringAndTest(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]models.OHLCV, 0, 90)
	add := func(open, high, low, close, vol float64) {
		i := len(bars)
		bars = append(bars, models.OHLCV{Symbol:"BTCUSDT", Timeframe:"15M", OpenTime:start.Add(time.Duration(i)*15*time.Minute), Open:open, High:high, Low:low, Close:close, Volume:vol})
	}

	price := 116.0
	for i := 0; i < 34; i++ {
		next := price - 0.45
		add(price, price+0.45, next-0.45, next, 100)
		price = next
	}
	add(price, price+0.8, price-1.5, price-0.7, 140) // PS
	price -= 0.7
	add(price, price+0.8, 98.0, 101.5, 240)          // SC
	add(101.5, 104, 101, 103.5, 120)
	add(103.5, 109, 103, 108, 125)                  // AR
	add(108, 108.5, 103, 104, 95)
	add(104, 105, 100.0, 102, 88)                   // ST
	add(102, 105, 101, 104, 92)
	add(104, 104.5, 97.2, 103.5, 150)               // Spring: controlled break + rejection
	add(103.5, 105, 101.0, 104, 90)
	add(104, 104.5, 99.0, 102.5, 65)                // Test: holds spring, lower vol/spread
	for i := 0; i < 10; i++ { add(102.5, 106, 102, 105, 95) }

	got := AnalyzeV3Foundation("BTCUSDT", "15M", bars)
	if !got.HasSpring || !got.HasTest {
		t.Fatalf("expected Spring+Test, got %+v", got)
	}
	if got.SpringQuality < 0.50 || got.TestQuality < 0.50 {
		t.Fatalf("expected qualifying quality, spring %.3f test %.3f", got.SpringQuality, got.TestQuality)
	}
	if got.TradeScore <= 0 { t.Fatalf("expected positive trade score") }
}

func TestAnalyzeV3FoundationRejectsFlatPrecondition(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]models.OHLCV, 70)
	for i := range bars {
		bars[i] = models.OHLCV{Symbol:"ETHUSDT", Timeframe:"15M", OpenTime:start.Add(time.Duration(i)*15*time.Minute), Open:100, High:101, Low:99, Close:100, Volume:100}
	}
	bars[40].High = 102; bars[40].Low = 92; bars[40].Close = 98; bars[40].Volume = 250
	got := AnalyzeV3Foundation("ETHUSDT", "15M", bars)
	if got.HasSC || got.PriorDowntrend { t.Fatalf("flat market should be rejected, got %+v", got) }
}

func TestV3ATRSeriesIsHistorical(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]models.OHLCV, 40)
	for i := range bars {
		bars[i] = models.OHLCV{OpenTime:start.Add(time.Duration(i)*15*time.Minute), Open:100, High:101, Low:99, Close:100}
	}
	before := v3ATRSeries(bars, 14)
	bars[39].High = 150; bars[39].Low = 50
	after := v3ATRSeries(bars, 14)
	for i := 14; i < 39; i++ {
		if before[i] != after[i] { t.Fatalf("future volatility changed ATR at index %d: %f vs %f", i, before[i], after[i]) }
	}
	if after[39] <= before[39] { t.Fatalf("expected latest ATR to react to volatility") }
}
