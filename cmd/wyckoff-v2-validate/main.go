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
	fmt.Printf("Bars: %d | Unique ranges: %d | Stage triggers: %d\n", s.Bars, s.UniqueRanges, s.Overall.Triggers)
	if s.Overall.Triggers == 0 {
		fmt.Println("No qualifying V2 triggers found in this period.")
		return
	}

	fmt.Println("\nOverall:")
	printBucket(s.Overall)
	fmt.Println("\nBy entry stage:")
	for _, b := range s.ByStage { printBucket(b) }
	fmt.Println("\nBy higher-timeframe context (close vs EMA50):")
	for _, b := range s.ByHTF { printBucket(b) }

	fmt.Println("\nRecent stage triggers:")
	start := 0
	if len(s.Events) > 12 { start = len(s.Events)-12 }
	for _, e := range s.Events[start:] {
		fmt.Printf("%s | %-11s | phase %s | conf %.0f%% | 1H %s 4H %s | 4h %+.2f%% | 8h %+.2f%% | 16h %+.2f%%\n",
			time.Unix(e.Time, 0).UTC().Format("2006-01-02 15:04 UTC"), e.Stage, e.Phase, e.Confidence*100,
			e.HTF1H, e.HTF4H, e.Return4H, e.Return8H, e.Return16H)
	}
}

func printBucket(b wyckoff.ValidationBucket) {
	if b.Triggers == 0 {
		fmt.Printf("%-12s n=%3d\n", b.Name, b.Triggers)
		return
	}
	fmt.Printf("%-12s n=%3d | 4h %.1f%% %+.3f%% | 8h %.1f%% %+.3f%% | 16h %.1f%% %+.3f%% | MFE %+.3f%% MAE %+.3f%%\n",
		b.Name, b.Triggers,
		b.WinRate4H, b.AvgReturn4H,
		b.WinRate8H, b.AvgReturn8H,
		b.WinRate16H, b.AvgReturn16H,
		b.AvgMFE16H, b.AvgMAE16H)
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
