package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

var bRateDecompIncidenceR = regexp.MustCompile(`^(2017-2019|2020-2022)\s+n=\s*(\d+)\s+\|\s+midpoint\s+(\d+)\s+\([^)]*\)\s+\|\s+prospective-HL\s+(\d+)\s+\([^)]*\)\s+\|\s+BOTH/B\s+(\d+)\s+\([^)]*\)$`)

type bRateDecompRow struct {
	structures int
	midpoint   int
	both       int
}

// init emits a fixed-era, two-factor decomposition from counts already present
// in the master report. It performs no market-data fetch and changes no signal,
// execution, stop, target, hold, cost, or trading rule.
func init() {
	path := "research/btc15m/latest.txt"
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-file" && i+1 < len(os.Args) {
			path = os.Args[i+1]
			break
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	rows := map[string]bRateDecompRow{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		m := bRateDecompIncidenceR.FindStringSubmatch(s.Text())
		if len(m) != 6 {
			continue
		}
		structures, err0 := strconv.Atoi(m[2])
		midpoint, err1 := strconv.Atoi(m[3])
		both, err2 := strconv.Atoi(m[5])
		if err0 != nil || err1 != nil || err2 != nil || structures <= 0 || midpoint <= 0 {
			continue
		}
		rows[m[1]] = bRateDecompRow{structures: structures, midpoint: midpoint, both: both}
	}

	early, okEarly := rows["2017-2019"]
	later, okLater := rows["2020-2022"]
	if !okEarly || !okLater {
		return
	}

	m1 := float64(early.midpoint) / float64(early.structures)
	m2 := float64(later.midpoint) / float64(later.structures)
	c1 := float64(early.both) / float64(early.midpoint)
	c2 := float64(later.both) / float64(later.midpoint)
	b1 := float64(early.both) / float64(early.structures)
	b2 := float64(later.both) / float64(later.structures)

	// Symmetric two-factor (Shapley-style) decomposition of B = midpoint incidence
	// * BOTH-given-midpoint conversion. The two effects sum exactly to the observed
	// fixed-era B-rate change and do not depend on choosing an ordering.
	incidenceEffect := (m2 - m1) * (c1 + c2) / 2
	conversionEffect := (c2 - c1) * (m1 + m2) / 2
	totalChange := b2 - b1
	incidenceShare, conversionShare := 0.0, 0.0
	if totalChange != 0 {
		incidenceShare = incidenceEffect / totalChange * 100
		conversionShare = conversionEffect / totalChange * 100
	}

	fmt.Println()
	fmt.Println("BTC 15M B-rate fixed-era decomposition (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Decomposes 2017-2019 -> 2020-2022 change in BOTH/B incidence into midpoint-incidence and BOTH-given-midpoint conversion effects. Symmetric two-factor attribution; no outcome filter or rule change.")
	fmt.Printf("2017-2019 midpoint %.1f%% | BOTH/midpoint %.1f%% | BOTH/B %.1f%%\n", m1*100, c1*100, b1*100)
	fmt.Printf("2020-2022 midpoint %.1f%% | BOTH/midpoint %.1f%% | BOTH/B %.1f%%\n", m2*100, c2*100, b2*100)
	fmt.Printf("B-rate change %+.2f pp | midpoint-incidence effect %+.2f pp (%.1f%% of change) | conversion effect %+.2f pp (%.1f%% of change)\n",
		totalChange*100, incidenceEffect*100, incidenceShare, conversionEffect*100, conversionShare)
}
