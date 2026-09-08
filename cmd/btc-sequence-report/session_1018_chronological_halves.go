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

// init appends one bounded chronological-stability diagnostic for the fixed
// 10-18 UTC frozen-trade block. It splits the exact trades once at the median
// observation by time and reports each half independently. Detector, entry,
// stop, target, hold, confirmation, costs and session definition are unchanged;
// this is descriptive only and does not create or optimize a time filter.
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
	if len(trades) < 2 {
		return
	}

	sort.Slice(trades, func(i, j int) bool { return trades[i].ts.Before(trades[j].ts) })
	cut := len(trades) / 2
	groups := []struct {
		label  string
		trades []trade
	}{
		{"EARLY HALF", trades[:cut]},
		{"LATE HALF", trades[cut:]},
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 chronological-half stability diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Exact fixed 10-18 UTC trades sorted by entry time and split once at the median observation. This is a bounded temporal-stability check, not a date/session filter or parameter search.")

	for _, g := range groups {
		if len(g.trades) == 0 {
			continue
		}
		netR := 0.0
		wins := 0
		profit := 0.0
		loss := 0.0
		for _, tr := range g.trades {
			netR += tr.netR
			if tr.netR > 0 {
				wins++
				profit += tr.netR
			} else if tr.netR < 0 {
				loss += -tr.netR
			}
		}
		pf := math.Inf(1)
		if loss > 0 {
			pf = profit / loss
		}
		fmt.Printf("%-10s | %s -> %s | n=%2d | NetR %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			g.label,
			g.trades[0].ts.Format("2006-01-02"),
			g.trades[len(g.trades)-1].ts.Format("2006-01-02"),
			len(g.trades), netR, netR/float64(len(g.trades)),
			float64(wins)/float64(len(g.trades))*100, pf)
	}
}
