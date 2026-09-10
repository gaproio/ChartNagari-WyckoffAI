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

// init appends one bounded session-boundary perturbation diagnostic around the
// already-studied 10-18 UTC frozen-trade cohort. It compares only the fixed
// one-hour shifts 09-17 and 11-19 against 10-18; it does not search session
// boundaries, create a filter, or change any detector/execution/risk rule.
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
		if err0 != nil || err1 != nil {
			continue
		}
		trades = append(trades, trade{ts: ts, netR: netR})
	}
	if len(trades) == 0 {
		return
	}

	type window struct {
		label string
		start int
		end   int
	}
	windows := []window{
		{label: "09-17 UTC (-1h)", start: 9, end: 17},
		{label: "10-18 UTC anchor", start: 10, end: 18},
		{label: "11-19 UTC (+1h)", start: 11, end: 19},
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 session-boundary perturbation (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Compares only the predeclared one-hour shifts 09-17 and 11-19 with the existing 10-18 UTC cohort. This is a bounded boundary-sensitivity check, not a session search or filter.")

	for _, w := range windows {
		n := 0
		wins := 0
		net := 0.0
		grossProfit := 0.0
		grossLoss := 0.0
		for _, tr := range trades {
			h := tr.ts.Hour()
			if h < w.start || h >= w.end {
				continue
			}
			n++
			net += tr.netR
			if tr.netR > 0 {
				wins++
				grossProfit += tr.netR
			} else if tr.netR < 0 {
				grossLoss += -tr.netR
			}
		}
		if n == 0 {
			fmt.Printf("%-18s | n=0\n", w.label)
			continue
		}
		pf := math.Inf(1)
		if grossLoss > 0 {
			pf = grossProfit / grossLoss
		}
		fmt.Printf("%-18s | n=%2d | net %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			w.label, n, net, net/float64(n), float64(wins)/float64(n)*100, pf)
	}
}
