package wyckoff

import (
	"math"
	"testing"

	"github.com/Ju571nK/Chatter/pkg/models"
)

func TestV4VariantEntriesAreCausal(t *testing.T) {
	bars := []models.OHLCV{
		{Open: 9.8, High: 10.0, Low: 9.0, Close: 9.7}, // Test
		{Open: 9.7, High: 10.5, Low: 9.4, Close: 9.8}, // V3 signal
		{Open: 9.9, High: 10.4, Low: 9.5, Close: 10.2},
		{Open: 10.2, High: 10.8, Low: 10.0, Close: 10.6},
	}

	if got := v4VariantEntry(bars, 1, 0, 10.0, 8, v4EntryMidpoint); got != 2 {
		t.Fatalf("midpoint entry = %d, want 2", got)
	}
	if got := v4VariantEntry(bars, 1, 0, 10.0, 8, v4EntryProspectiveHL); got != 2 {
		t.Fatalf("prospective-HL entry = %d, want 2", got)
	}
	if got := v4VariantEntry(bars, 1, 0, 10.0, 8, v4EntryAboveSignalHigh); got != 3 {
		t.Fatalf("above-signal-high entry = %d, want 3", got)
	}
}

func TestV4ProspectiveHLRejectsTestLowBreak(t *testing.T) {
	bars := []models.OHLCV{
		{Open: 9.8, High: 10.0, Low: 9.0, Close: 9.7},
		{Open: 9.7, High: 10.2, Low: 9.4, Close: 9.8},
		{Open: 9.8, High: 10.0, Low: 8.9, Close: 9.5}, // breaks Test low
		{Open: 9.6, High: 10.8, Low: 9.5, Close: 10.6},
	}
	if got := v4VariantEntry(bars, 1, 0, 10.0, 8, v4EntryProspectiveHL); got != -1 {
		t.Fatalf("prospective-HL entry after Test-low break = %d, want -1", got)
	}
}

func TestV4VariantStops(t *testing.T) {
	bars := []models.OHLCV{
		{Low: 9.5},
		{Low: 9.8},
		{Low: 9.7},
	}
	if got := v4VariantStop(bars, 0, 2, 8.5, 1.0, v4StopSpring); math.Abs(got-7.75) > 1e-9 {
		t.Fatalf("spring stop = %.4f, want 7.75", got)
	}
	if got := v4VariantStop(bars, 0, 2, 8.5, 1.0, v4StopPostTest); math.Abs(got-9.25) > 1e-9 {
		t.Fatalf("post-Test stop = %.4f, want 9.25", got)
	}
}
