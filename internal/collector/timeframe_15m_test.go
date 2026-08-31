package collector

import (
	"testing"
	"time"
)

func TestBinance15MinuteTimeframeMapping(t *testing.T) {
	if got := BinanceTFMap["15M"]; got != "15m" {
		t.Fatalf("BinanceTFMap[15M] = %q, want %q", got, "15m")
	}
	if got := tfDuration["15M"]; got != 15*time.Minute {
		t.Fatalf("tfDuration[15M] = %v, want %v", got, 15*time.Minute)
	}
	if got := binanceIntervalToTF("15m"); got != "15M" {
		t.Fatalf("binanceIntervalToTF(15m) = %q, want %q", got, "15M")
	}
}
