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

// init appends one bounded structural-risk concentration diagnostic for the
// favorable 08-16 UTC frozen-trade block. It removes each pre-specified broad
// entry-to-stop risk band once and recomputes the remaining NetR/PF. Detector,
// entry, stop, target, hold, confirmation and cost rules are unchanged; this is
// descriptive only and does not create a risk/session filter.
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

	tradeLine := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) .*\| risk ([0-9]+(?:\.[0-9]+)?)% \|.* net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)
	type trade struct {
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
		if err0 != nil || err1 != nil || err2 != nil || ts.Hour() < 8 || ts.Hour() >= 16 {
			continue
		}
		trades = append(trades, trade{risk: risk, netR: netR})
	}
	if len(trades) == 0 {
		return
	}

	type band struct {
		name string
		in   func(float64) bool
	}
	bands := []band{
		{name: "risk <=2%", in: func(r float64) bool { return r <= 2 }},
		{name: "risk 2-4%", in: func(r float64) bool { return r > 2 && r <= 4 }},
		{name: "risk >4%", in: func(r float64) bool { return r > 4 }},
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 08-16 leave-one-risk-band-out stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within the 08-16 UTC entry block only, removes each fixed broad entry-to-stop risk cohort once and recomputes remaining NetR/PF. This tests structural-risk concentration; it does not create a risk/session filter.")

	for _, b := range bands {
		removedN := 0
		removedR := 0.0
		remainN := 0
		remainR := 0.0
		wins := 0
		profit := 0.0
		loss := 0.0
		for _, tr := range trades {
			if b.in(tr.risk) {
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
		fmt.Printf("omit %-10s | removed %2d trades %+.3fR | remaining %2d trades %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			b.name, removedN, removedR, remainN, remainR, remainR/float64(remainN),
			float64(wins)/float64(remainN)*100, pf)
	}
}
