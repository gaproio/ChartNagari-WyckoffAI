package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ju571nK/Chatter/internal/wyckoff"
	"github.com/Ju571nK/Chatter/pkg/models"
)

const binanceKlinesURL = "https://api.binance.com/api/v3/klines"

func main() {
	symbol := flag.String("symbol", "BTCUSDT", "Binance symbol")
	days := flag.Int("days", 90, "history length in days")
	flag.Parse()

	if *days < 7 {
		fmt.Fprintln(os.Stderr, "days must be at least 7")
		os.Exit(2)
	}

	bars, err := fetch15M(strings.ToUpper(*symbol), *days)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch failed:", err)
		os.Exit(1)
	}

	s := wyckoff.ValidateV2(strings.ToUpper(*symbol), bars)
	fmt.Printf("\nWyckoff V2 validation — %s 15M\n", s.Symbol)
	fmt.Printf("Bars: %d | Distinct triggers: %d\n", s.Bars, s.Triggers)
	if s.Triggers == 0 {
		fmt.Println("No qualifying V2 triggers found in this period.")
		return
	}
	fmt.Printf("4h : win %.1f%% | avg return %+.3f%%\n", s.WinRate4H, s.AvgReturn4H)
	fmt.Printf("8h : win %.1f%% | avg return %+.3f%%\n", s.WinRate8H, s.AvgReturn8H)
	fmt.Printf("16h: win %.1f%% | avg return %+.3f%%\n", s.WinRate16H, s.AvgReturn16H)
	fmt.Printf("16h avg MFE %+.3f%% | avg MAE %+.3f%%\n", s.AvgMFE16H, s.AvgMAE16H)
	fmt.Println("\nRecent triggers:")
	start := 0
	if len(s.Events) > 10 { start = len(s.Events)-10 }
	for _, e := range s.Events[start:] {
		fmt.Printf("%s | phase %s | conf %.0f%% | 4h %+.2f%% | 8h %+.2f%% | 16h %+.2f%%\n",
			time.Unix(e.Time, 0).UTC().Format("2006-01-02 15:04 UTC"), e.Phase, e.Confidence*100,
			e.Return4H, e.Return8H, e.Return16H)
	}
}

func fetch15M(symbol string, days int) ([]models.OHLCV, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	end := time.Now().UTC()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)
	startMS := start.UnixMilli()
	endMS := end.UnixMilli()
	bars := make([]models.OHLCV, 0, days*96)

	for startMS < endMS {
		url := fmt.Sprintf("%s?symbol=%s&interval=15m&limit=1000&startTime=%d&endTime=%d", binanceKlinesURL, symbol, startMS, endMS)
		resp, err := client.Get(url)
		if err != nil { return nil, err }
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil { return nil, err }
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Binance HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var raw [][]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil { return nil, err }
		if len(raw) == 0 { break }

		lastOpen := int64(0)
		for _, row := range raw {
			if len(row) < 6 { continue }
			var openMS int64
			if err := json.Unmarshal(row[0], &openMS); err != nil { continue }
			bar := models.OHLCV{
				Symbol: symbol, Timeframe: "15M", OpenTime: time.UnixMilli(openMS).UTC(),
				Open: rawFloat(row[1]), High: rawFloat(row[2]), Low: rawFloat(row[3]), Close: rawFloat(row[4]), Volume: rawFloat(row[5]),
			}
			if bar.Close > 0 { bars = append(bars, bar) }
			if openMS > lastOpen { lastOpen = openMS }
		}
		if lastOpen == 0 { break }
		next := lastOpen + int64(15*time.Minute/time.Millisecond)
		if next <= startMS { break }
		startMS = next
		if len(raw) < 1000 { break }
		time.Sleep(120 * time.Millisecond)
	}
	return bars, nil
}

func rawFloat(raw json.RawMessage) float64 {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	var v float64
	_ = json.Unmarshal(raw, &v)
	return v
}
