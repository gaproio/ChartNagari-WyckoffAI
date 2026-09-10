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
// 10-18 UTC session neighborhood. Because the 09-17, 10-18, and 11-19
// perturbation windows overlap heavily, this separates their shared 11-17 UTC
// core from the four one-hour boundary bands (09-11 and 17-19). It is purely
// descriptive and does not create a session filter or change frozen rules.
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
	fmt.Println("BTC 15M UTC 10-18 core-edge robustness diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Separates the shared 11-17 UTC core of the three fixed boundary windows from their four one-hour edge bands. This checks whether perturbation robustness is merely shared-sample overlap; no session filter is created.")

	for _, c := range cohorts {
		n := 0
		wins := 0
		net := 0.0
		grossProfit := 0.0
		grossLoss := 0.0
		for _, tr := range trades {
			if !c.keep(tr.ts.Hour()) {
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
			fmt.Printf("%-22s | n=0\n", c.label)
			continue
		}
		pf := math.Inf(1)
		if grossLoss > 0 {
			pf = grossProfit / grossLoss
		}
		fmt.Printf("%-22s | n=%2d | net %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			c.label, n, net, net/float64(n), float64(wins)/float64(n)*100, pf)
	}
}
