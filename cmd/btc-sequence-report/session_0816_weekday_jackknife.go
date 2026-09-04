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

// init appends one bounded leave-one-weekday-out robustness diagnostic for the
// favorable 08-16 UTC frozen-trade block. It asks whether that session result
// depends on one weekday cohort. Frozen detector/execution/risk rules are
// unchanged and this diagnostic must not be interpreted as a weekday/session
// filter.
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

	weekdayOrder := []time.Weekday{
		time.Monday,
		time.Tuesday,
		time.Wednesday,
		time.Thursday,
		time.Friday,
		time.Saturday,
		time.Sunday,
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 08-16 leave-one-weekday-out stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within the 08-16 UTC entry block only, removes each represented UTC weekday cohort once and recomputes remaining NetR/PF. This tests calendar concentration inside the session; it does not create a weekday/session filter.")

	minNet := math.Inf(1)
	maxNet := math.Inf(-1)
	represented := 0
	for _, wd := range weekdayOrder {
		removedN := 0
		removedNet := 0.0
		remainingN := 0
		remainingNet := 0.0
		wins := 0
		grossProfit := 0.0
		grossLoss := 0.0

		for _, tr := range trades {
			if tr.ts.Weekday() == wd {
				removedN++
				removedNet += tr.netR
				continue
			}
			remainingN++
			remainingNet += tr.netR
			if tr.netR > 0 {
				wins++
				grossProfit += tr.netR
			} else if tr.netR < 0 {
				grossLoss += -tr.netR
			}
		}
		if removedN == 0 || remainingN == 0 {
			continue
		}
		represented++
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
		fmt.Printf("omit %-9s | removed %2d trades %+.3fR | remaining %2d trades %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			wd.String(), removedN, removedNet, remainingN, remainingNet,
			remainingNet/float64(remainingN), float64(wins)/float64(remainingN)*100, pf)
	}
	if represented > 0 {
		fmt.Printf("weekday-jackknife remaining-NetR range: %+.3fR to %+.3fR across %d represented weekdays\n", minNet, maxNet, represented)
	}
}
