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

	// Controlled decline establishes the prerequisite trend without large ATR.
	price := 112.0
	for i := 0; i < 34; i++ {
		next := price - 0.35
		add(price, price+0.45, next-0.45, next, 100)
		price = next
	}
	add(price, price+0.8, price-1.4, price-0.6, 135) // PS
	price -= 0.6
	add(price, price+0.7, 94.0, 97.0, 230)            // SC
	add(97, 100, 96.5, 99, 115)
	add(99, 104.5, 98.5, 103.5, 120)                 // AR
	add(103.5, 104, 100, 101, 95)
	add(101, 102, 98, 99, 90)
	add(99, 100, 94.8, 97, 85)                       // ST
	for i := 0; i < 12; i++ { add(97, 101, 96, 100, 95) }

	got := AnalyzeV3Foundation("BTCUSDT", "15M", bars)
	if !got.PriorDowntrend || !got.HasPS || !got.HasSC || !got.HasAR || !got.HasST {
		t.Fatalf("expected complete V3 Phase-B foundation, got %+v", got)
	}
	if got.Phase != V2PhaseB { t.Fatalf("expected Phase B, got %s", got.Phase) }
	if got.StructureConfidence < 0.65 { t.Fatalf("unexpected confidence %.2f", got.StructureConfidence) }
}

func TestAnalyzeV3FoundationRejectsFlatPrecondition(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]models.OHLCV, 70)
	for i := range bars {
		bars[i] = models.OHLCV{Symbol:"ETHUSDT", Timeframe:"15M", OpenTime:start.Add(time.Duration(i)*15*time.Minute), Open:100, High:101, Low:99, Close:100, Volume:100}
	}
	// A climax-like candle alone must not create an accumulation structure.
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
