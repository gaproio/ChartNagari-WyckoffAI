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

// init appends one bounded leave-one-calendar-year-out diagnostic for the
// favorable 08-16 UTC frozen-trade block. It tests whether that session's net-R
// contribution is dominated by a single year. No detector, confirmation,
// execution, stop, target, holding-period, cost, or session-filter rule changes.
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
		year int
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
		trades = append(trades, trade{year: ts.Year(), netR: netR})
	}
	if len(trades) == 0 {
		return
	}

	yearSet := make(map[int]struct{})
	for _, tr := range trades {
		yearSet[tr.year] = struct{}{}
	}
	years := make([]int, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Ints(years)

	fmt.Println()
	fmt.Println("BTC 15M UTC 08-16 leave-one-year-out concentration stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within the 08-16 UTC entry block only, removes each represented calendar year in turn and recomputes remaining NetR. This tests whether session concentration depends on one year; it does not create a year/session filter.")

	for _, omitYear := range years {
		removedN := 0
		removedNet := 0.0
		remainingN := 0
		remainingNet := 0.0
		wins := 0
		grossProfit := 0.0
		grossLoss := 0.0

		for _, tr := range trades {
			if tr.year == omitYear {
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
		fmt.Printf("omit %d | removed %2d trades %+.3fR | remaining %2d trades %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			omitYear, removedN, removedNet, remainingN, remainingNet, remainingNet/float64(remainingN), float64(wins)/float64(remainingN)*100, pf)
	}
}
