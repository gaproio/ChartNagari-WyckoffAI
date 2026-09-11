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

// init appends one bounded robustness diagnostic for the already-studied
// 10-18 UTC boundary neighborhood. The preceding core-edge diagnostic showed
// positive aggregate NetR in both the shared 11-17 core and the non-overlapping
// 09-11/17-19 edge bands. This check removes exactly the single best and single
// worst trade from each cohort to show whether that positivity is dominated by
// outcome extremes. It is descriptive only and changes no frozen rule.
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

	type cohort struct {
		label string
		keep  func(int) bool
	}
	cohorts := []cohort{
		{label: "CORE 11-17 UTC", keep: func(h int) bool { return h >= 11 && h < 17 }},
		{label: "EDGES 09-11 + 17-19", keep: func(h int) bool { return (h >= 9 && h < 11) || (h >= 17 && h < 19) }},
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 core-edge extreme-trim stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within each fixed core/edge cohort, removes exactly its single best and single worst NetR observation, then recomputes the remainder. This is an influence check only; it does not create a trade/session filter.")

	for _, c := range cohorts {
		n := 0
		total := 0.0
		best := math.Inf(-1)
		worst := math.Inf(1)
		for _, tr := range trades {
			if !c.keep(tr.ts.Hour()) {
				continue
			}
			n++
			total += tr.netR
			if tr.netR > best {
				best = tr.netR
			}
			if tr.netR < worst {
				worst = tr.netR
			}
		}
		if n < 3 {
			fmt.Printf("%-22s | n=%2d | insufficient observations for one-best/one-worst trim\n", c.label, n)
			continue
		}
		trimmedN := n - 2
		trimmed := total - best - worst
		fmt.Printf("%-22s | n=%2d | raw %+.3fR | best %+.3fR worst %+.3fR | trimmed n=%2d net %+.3fR avg %+.3fR\n",
			c.label, n, total, best, worst, trimmedN, trimmed, trimmed/float64(trimmedN))
	}
}
