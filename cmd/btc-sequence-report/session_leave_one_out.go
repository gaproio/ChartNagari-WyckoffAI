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

// init appends one bounded leave-one-session-out robustness diagnostic for the
// frozen BTCUSDT/15M trade set. It is motivated by the descriptive concentration
// of net R in the 08-16 UTC entry block. No detector, confirmation, execution,
// stop, target, holding-period, cost, or session-filter rule is changed.
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
		session int
		netR    float64
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
		trades = append(trades, trade{session: ts.Hour() / 8, netR: netR})
	}
	if len(trades) == 0 {
		return
	}

	sessionNames := []string{"UTC 00-08", "UTC 08-16", "UTC 16-24"}
	fmt.Println()
	fmt.Println("BTC 15M UTC session leave-one-block-out stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Removes each fixed 8-hour UTC entry block in turn and recomputes the remaining frozen-trade NetR path. This tests session concentration only; it does not create a session filter.")

	for omit, name := range sessionNames {
		removedN := 0
		removedNet := 0.0
		remainingN := 0
		remainingNet := 0.0
		wins := 0
		grossProfit := 0.0
		grossLoss := 0.0
		for _, tr := range trades {
			if tr.session == omit {
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
		fmt.Printf("omit %-9s | removed %2d trades %+.3fR | remaining %2d trades %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			name, removedN, removedNet, remainingN, remainingNet, remainingNet/float64(remainingN), float64(wins)/float64(remainingN)*100, pf)
	}
}
