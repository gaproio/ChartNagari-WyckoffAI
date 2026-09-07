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

// init appends one bounded intraday-concentration robustness diagnostic for the
// fixed 10-18 UTC frozen-trade block. It removes each predeclared 2-hour entry
// cohort once and recomputes the remaining NetR/PF. Detector, entry, stop,
// target, hold, confirmation and cost rules are unchanged; this is descriptive
// only and does not create or optimize a sub-session filter.
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

	tradeLine := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) .*\|.* net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)
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
	if len(trades) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 leave-one-2H-block-out stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within the fixed 10-18 UTC entry block only, removes each predeclared 2-hour UTC entry cohort once and recomputes remaining NetR/PF. This tests intraday concentration; it does not create or optimize a sub-session filter.")

	for start := 10; start < 18; start += 2 {
		removedN := 0
		removedR := 0.0
		remainN := 0
		remainR := 0.0
		wins := 0
		profit := 0.0
		loss := 0.0
		for _, tr := range trades {
			inBlock := tr.ts.Hour() >= start && tr.ts.Hour() < start+2
			if inBlock {
				removedN++
				removedR += tr.netR
				continue
			}
			remainN++
			remainR += tr.netR
			if tr.netR > 0 {
				wins++
				profit += tr.netR
			} else if tr.netR < 0 {
				loss += -tr.netR
			}
		}
		if removedN == 0 || remainN == 0 {
			continue
		}
		pf := math.Inf(1)
		if loss > 0 {
			pf = profit / loss
		}
		fmt.Printf("omit %02d-%02d UTC | removed %2d trades %+.3fR | remaining %2d trades %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			start, start+2, removedN, removedR, remainN, remainR, remainR/float64(remainN),
			float64(wins)/float64(remainN)*100, pf)
	}
}
