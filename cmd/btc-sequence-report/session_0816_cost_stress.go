package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// init appends one bounded transaction-cost robustness diagnostic for the
// favorable 08-16 UTC frozen-trade block. It re-prices the exact same frozen
// trades at baseline, 1.5x, 2x and 3x the baseline round-trip friction inferred
// from gross-vs-net R. Detector, entry, stop, target, hold and confirmation
// rules are unchanged; this is not a session or execution-rule optimization.
func init() {
	path := "research/btc15m/latest.txt"
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-file" && i+1 < len(os.Args) {
			path = os.Args[i+1]
			break
		}
		if strings.HasPrefix(arg, "-file=") {
			path = strings.TrimPrefix(arg, "-file=")
			break
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	tradeLine := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) .*\|.* gross ([+-]?[0-9]+(?:\.[0-9]+)?)R net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)
	type trade struct {
		grossR float64
		netR   float64
	}
	var trades []trade

	s := bufio.NewScanner(f)
	for s.Scan() {
		m := tradeLine.FindStringSubmatch(s.Text())
		if len(m) != 4 {
			continue
		}
		ts, err0 := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
		grossR, err1 := strconv.ParseFloat(m[2], 64)
		netR, err2 := strconv.ParseFloat(m[3], 64)
		if err0 != nil || err1 != nil || err2 != nil || ts.Hour()/8 != 1 {
			continue
		}
		trades = append(trades, trade{grossR: grossR, netR: netR})
	}
	if len(trades) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 08-16 transaction-cost stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Exact same 08-16 UTC frozen trades re-priced at fixed multiples of baseline friction. Baseline cost is inferred trade-by-trade as grossR-netR; no signals, fills, stops, targets or holding rules change.")

	multipliers := []float64{1.0, 1.5, 2.0, 3.0}
	for _, mult := range multipliers {
		total := 0.0
		wins := 0
		grossProfit := 0.0
		grossLoss := 0.0
		costTotal := 0.0
		for _, tr := range trades {
			baseCostR := tr.grossR - tr.netR
			adjusted := tr.grossR - mult*baseCostR
			total += adjusted
			costTotal += mult * baseCostR
			if adjusted > 0 {
				wins++
				grossProfit += adjusted
			} else if adjusted < 0 {
				grossLoss += -adjusted
			}
		}
		pf := math.Inf(1)
		if grossLoss > 0 {
			pf = grossProfit / grossLoss
		}
		fmt.Printf("cost x%.1f | n=%2d | NetR %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f | total friction %.3fR\n",
			mult, len(trades), total, total/float64(len(trades)),
			float64(wins)/float64(len(trades))*100, pf, costTotal)
	}
}
