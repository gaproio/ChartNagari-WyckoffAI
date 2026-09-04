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

// init appends one bounded leave-one-trade-out jackknife for the favorable
// 08-16 UTC frozen-trade block. It tests whether that session's net-R result is
// dependent on any single observation. Frozen detector/execution/risk rules are
// unchanged and this diagnostic must not be interpreted as a session filter.
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

	tradeLine := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) .*\|.* gross [+-]?[0-9]+(?:\.[0-9]+)?R net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)
	type trade struct {
		ts   time.Time
		netR float64
	}
	var trades []trade

	s := bufio.NewScanner(f)
	for s.Scan() {
		m := tradeLine.FindStringSubmatch(s.Text())
		if len(m) != 3 {
			continue
		}
		ts, err0 := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
		netR, err1 := strconv.ParseFloat(m[2], 64)
		if err0 != nil || err1 != nil || ts.Hour()/8 != 1 {
			continue
		}
		trades = append(trades, trade{ts: ts, netR: netR})
	}
	if len(trades) < 2 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 08-16 single-trade jackknife (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within the 08-16 UTC entry block only, removes each frozen trade once and recomputes remaining NetR/PF. This tests single-observation dependence; it does not create a trade/session filter.")

	minNet := math.Inf(1)
	maxNet := math.Inf(-1)
	for omit := range trades {
		remainingNet := 0.0
		wins := 0
		grossProfit := 0.0
		grossLoss := 0.0
		for i, tr := range trades {
			if i == omit {
				continue
			}
			remainingNet += tr.netR
			if tr.netR > 0 {
				wins++
				grossProfit += tr.netR
			} else if tr.netR < 0 {
				grossLoss += -tr.netR
			}
		}
		pf := math.Inf(1)
		if grossLoss > 0 {
			pf = grossProfit / grossLoss
		}
		if remainingNet < minNet {
			minNet = remainingNet
		}
		if remainingNet > maxNet {
			maxNet = remainingNet
		}
		remainingN := len(trades) - 1
		fmt.Printf("omit %s (%+.3fR) | remaining %2d trades %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			trades[omit].ts.Format("2006-01-02 15:04"), trades[omit].netR,
			remainingN, remainingNet, remainingNet/float64(remainingN), float64(wins)/float64(remainingN)*100, pf)
	}
	fmt.Printf("jackknife remaining-NetR range: %+.3fR to %+.3fR across %d omissions\n", minNet, maxNet, len(trades))
}
