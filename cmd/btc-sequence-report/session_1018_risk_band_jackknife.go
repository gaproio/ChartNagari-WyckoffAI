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

// init appends one bounded descriptive robustness diagnostic for the already-
// studied 10-18 UTC cohort. It uses the same broad structural-risk bands that
// already appear in the master report (<=2%, 2-4%, >4%) and leaves each band
// out once. This tests whether the apparent session result is merely risk-mix
// concentration; it creates no risk/session filter and changes no frozen rule.
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

	tradeLine := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) .*\| risk ([0-9]+(?:\.[0-9]+)?)% \|.* gross [+-]?[0-9]+(?:\.[0-9]+)?R net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)
	type trade struct {
		ts   time.Time
		risk float64
		netR float64
	}
	var trades []trade

	s := bufio.NewScanner(f)
	for s.Scan() {
		m := tradeLine.FindStringSubmatch(s.Text())
		if len(m) != 4 {
			continue
		}
		ts, err0 := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
		risk, err1 := strconv.ParseFloat(m[2], 64)
		netR, err2 := strconv.ParseFloat(m[3], 64)
		if err0 != nil || err1 != nil || err2 != nil {
			continue
		}
		if ts.Hour() >= 10 && ts.Hour() < 18 {
			trades = append(trades, trade{ts: ts, risk: risk, netR: netR})
		}
	}
	if len(trades) == 0 {
		return
	}

	type band struct {
		label string
		in    func(float64) bool
	}
	bands := []band{
		{label: "risk <=2%", in: func(r float64) bool { return r <= 2 }},
		{label: "risk 2-4%", in: func(r float64) bool { return r > 2 && r <= 4 }},
		{label: "risk >4%", in: func(r float64) bool { return r > 4 }},
	}

	allTotal := 0.0
	for _, tr := range trades {
		allTotal += tr.netR
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 leave-one-risk-band-out stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within the fixed 10-18 UTC cohort, removes each already-predeclared broad structural-risk band once and recomputes the remainder. This tests risk-mix concentration only; it does not create a risk/session filter.")

	minRemain := math.Inf(1)
	maxRemain := math.Inf(-1)
	represented := 0
	for _, b := range bands {
		removedN := 0
		removed := 0.0
		remainN := 0
		remain := 0.0
		wins := 0
		grossProfit := 0.0
		grossLoss := 0.0
		for _, tr := range trades {
			if b.in(tr.risk) {
				removedN++
				removed += tr.netR
				continue
			}
			remainN++
			remain += tr.netR
			if tr.netR > 0 {
				wins++
				grossProfit += tr.netR
			} else if tr.netR < 0 {
				grossLoss += -tr.netR
			}
		}
		if removedN == 0 {
			continue
		}
		represented++
		if remain < minRemain {
			minRemain = remain
		}
		if remain > maxRemain {
			maxRemain = remain
		}
		pf := "+Inf"
		if grossLoss > 0 {
			pf = fmt.Sprintf("%.2f", grossProfit/grossLoss)
		}
		avg := 0.0
		winPct := 0.0
		if remainN > 0 {
			avg = remain / float64(remainN)
			winPct = 100 * float64(wins) / float64(remainN)
		}
		fmt.Printf("omit %-10s | removed %2d trades %+.3fR | remaining %2d trades %+.3fR avg %+.3fR | net-win %.1f%% | PF %s\n",
			b.label, removedN, removed, remainN, remain, avg, winPct, pf)
	}
	if represented > 0 {
		fmt.Printf("risk-band jackknife all NetR %+.3fR | remaining-NetR range: %+.3fR to %+.3fR across %d represented bands\n",
			allTotal, minRemain, maxRemain, represented)
	}
}
