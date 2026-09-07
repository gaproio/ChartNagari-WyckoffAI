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

// init appends one bounded two-hour concentration diagnostic for the fixed
// 10-18 UTC frozen-trade block. It tests whether the later-session result is
// dominated by a narrow sub-block. Frozen detector/execution/risk rules remain
// unchanged; this diagnostic must not be interpreted as a session filter.
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
	type stat struct {
		n           int
		netR        float64
		wins        int
		grossProfit float64
		grossLoss   float64
	}
	blocks := map[int]*stat{10: {}, 12: {}, 14: {}, 16: {}}
	totalN := 0

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
		start := 10 + ((ts.Hour() - 10) / 2 * 2)
		st := blocks[start]
		st.n++
		st.netR += netR
		totalN++
		if netR > 0 {
			st.wins++
			st.grossProfit += netR
		} else if netR < 0 {
			st.grossLoss += -netR
		}
	}
	if totalN < 2 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 two-hour concentration diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Exact frozen trades in the fixed 10-18 UTC entry block grouped into four predeclared 2-hour UTC sub-blocks. This measures concentration only; it does not create or optimize a session filter.")
	maxAbsNet := 0.0
	maxBlock := -1
	for _, start := range []int{10, 12, 14, 16} {
		st := blocks[start]
		pf := math.Inf(1)
		if st.grossLoss > 0 {
			pf = st.grossProfit / st.grossLoss
		}
		winRate := 0.0
		avg := 0.0
		if st.n > 0 {
			winRate = float64(st.wins) / float64(st.n) * 100
			avg = st.netR / float64(st.n)
		}
		fmt.Printf("%02d-%02d UTC | n=%2d | net %+.3fR avg %+.3fR | net-win %.1f%% | PF %.2f\n",
			start, start+2, st.n, st.netR, avg, winRate, pf)
		if math.Abs(st.netR) > maxAbsNet {
			maxAbsNet = math.Abs(st.netR)
			maxBlock = start
		}
	}
	if maxBlock >= 0 {
		fmt.Printf("largest absolute 2h NetR contribution: %02d-%02d UTC at %.3fR absolute\n", maxBlock, maxBlock+2, maxAbsNet)
	}
}
