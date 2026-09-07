package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// init appends one bounded leave-one-weekday-out concentration stress for the
// fixed 10-18 UTC frozen-trade block. It tests whether the later-session result
// is concentrated in one weekday cohort. Frozen detector/execution/risk rules
// remain unchanged; this diagnostic must not be interpreted as a weekday filter.
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
		if err0 != nil || err1 != nil || ts.Hour() < 10 || ts.Hour() >= 18 {
			continue
		}
		trades = append(trades, trade{ts: ts, netR: netR})
	}
	if len(trades) < 2 {
		return
	}

	weekdaySet := make(map[time.Weekday]struct{})
	for _, tr := range trades {
		weekdaySet[tr.ts.Weekday()] = struct{}{}
	}
	weekdays := make([]time.Weekday, 0, len(weekdaySet))
	for wd := range weekdaySet {
		weekdays = append(weekdays, wd)
	}
	sort.Slice(weekdays, func(i, j int) bool { return weekdays[i] < weekdays[j] })

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 leave-one-weekday-out concentration stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within the fixed 10-18 UTC entry block only, removes each represented UTC weekday cohort once and recomputes remaining NetR/PF. This tests weekday concentration; it does not create a weekday/session filter.")

	minNet := math.Inf(1)
	maxNet := math.Inf(-1)
	for _, omitWD := range weekdays {
		removedN := 0
		removedNet := 0.0
		remainingN := 0
		remainingNet := 0.0
		wins := 0
		grossProfit := 0.0
		grossLoss := 0.0

		for _, tr := range trades {
			if tr.ts.Weekday() == omitWD {
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
		if remainingN == 0 {
			continue
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
		fmt.Printf("omit %-9s | removed %2d trades %+.3fR | remaining %2d trades %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			omitWD.String(), removedN, removedNet, remainingN, remainingNet,
			remainingNet/float64(remainingN), float64(wins)/float64(remainingN)*100, pf)
	}
	if !math.IsInf(minNet, 1) {
		fmt.Printf("weekday-omission remaining-NetR range: %+.3fR to %+.3fR across %d represented weekdays\n", minNet, maxNet, len(weekdays))
	}
}
