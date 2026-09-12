package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// init appends one bounded regime-matched descriptive comparison for the fixed
// 10-18 UTC cohort. It compares mean NetR inside versus outside the session
// within each already-defined causal 30D regime, then reports an equal-weight
// common-support contrast across represented regimes. Frozen detector,
// execution, stop, target, hold, cost, confirmation and session definitions are
// unchanged; this diagnostic must not be interpreted as a regime/session filter.
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

	tradeLine := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s+\|\s+([A-Z0-9_]+)\s+.*\|.* gross [+-]?[0-9]+(?:\.[0-9]+)?R net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)
	type bucket struct {
		inN     int
		outN    int
		inSum   float64
		outSum  float64
	}
	byRegime := map[string]*bucket{}

	s := bufio.NewScanner(f)
	for s.Scan() {
		m := tradeLine.FindStringSubmatch(s.Text())
		if len(m) != 4 {
			continue
		}
		ts, err0 := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
		netR, err1 := strconv.ParseFloat(m[3], 64)
		if err0 != nil || err1 != nil {
			continue
		}
		regime := m[2]
		b := byRegime[regime]
		if b == nil {
			b = &bucket{}
			byRegime[regime] = b
		}
		if ts.Hour() >= 10 && ts.Hour() < 18 {
			b.inN++
			b.inSum += netR
		} else {
			b.outN++
			b.outSum += netR
		}
	}

	regimeOrder := []string{"BEAR_30D", "SIDEWAYS_30D", "BULL_30D", "UNKNOWN"}
	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime-matched contrast (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Compares mean NetR inside versus outside the fixed 10-18 UTC window within each existing causal 30D regime. The final contrast equal-weights only regimes with trades on both sides, reducing regime-mix confounding without creating a session or regime filter.")

	common := 0
	diffSum := 0.0
	for _, regime := range regimeOrder {
		b := byRegime[regime]
		if b == nil || b.inN == 0 || b.outN == 0 {
			continue
		}
		inAvg := b.inSum / float64(b.inN)
		outAvg := b.outSum / float64(b.outN)
		diff := inAvg - outAvg
		common++
		diffSum += diff
		fmt.Printf("%-12s | 10-18 n=%2d avg %+.3fR | outside n=%2d avg %+.3fR | within-regime diff %+.3fR\n",
			regime, b.inN, inAvg, b.outN, outAvg, diff)
	}
	if common > 0 {
		fmt.Printf("equal-weight common-support contrast across %d regimes: %+.3fR (10-18 minus outside)\n",
			common, diffSum/float64(common))
	} else {
		fmt.Println("common-support contrast unavailable: no regime has trades both inside and outside 10-18 UTC")
	}
}
