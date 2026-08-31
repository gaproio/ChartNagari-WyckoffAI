package wyckoff

import (
	"testing"
	"time"

	"github.com/Ju571nK/Chatter/pkg/models"
)

func TestSimulateBTCMasterOutcomeStopWinsSameCandle(t *testing.T) {
	bars := []models.OHLCV{
		{Open:100,High:140,Low:80,Close:110},
	}
	r,outcome,idx := simulateBTCMasterOutcome(bars,0,0,100,90,3)
	if outcome != "STOP" || r != -1 || idx != 0 {
		t.Fatalf("expected conservative same-candle STOP, got outcome=%s r=%v idx=%d",outcome,r,idx)
	}
}

func TestBTC30DRegimeUsesPastReturn(t *testing.T) {
	const lookback = 4
	base := time.Date(2026,1,1,0,0,0,0,time.UTC)
	bars := make([]models.OHLCV,6)
	for i := range bars {
		bars[i].OpenTime = base.Add(time.Duration(i)*15*time.Minute)
		bars[i].Open = 100
		bars[i].Close = 100
	}
	bars[0].Close = 100
	bars[4].Open = 115
	regime,ret := btc30DRegime(bars,4,lookback,10)
	if regime != "BULL_30D" {
		t.Fatalf("expected BULL_30D, got %s (ret %.2f)",regime,ret)
	}
	if ret < 14.9 || ret > 15.1 {
		t.Fatalf("expected about 15%% return, got %.3f",ret)
	}
}
