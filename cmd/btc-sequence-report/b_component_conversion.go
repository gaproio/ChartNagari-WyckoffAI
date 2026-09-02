package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

var bComponentIncidenceR = regexp.MustCompile(`^(2017-2019|2020-2022|2023-2025|2026 PARTIAL)\s+n=\s*(\d+)\s+\|\s+midpoint\s+(\d+)\s+\([^)]*\)\s+\|\s+prospective-HL\s+(\d+)\s+\([^)]*\)\s+\|\s+BOTH/B\s+(\d+)\s+\([^)]*\)$`)

type bComponentConversionRow struct {
	name       string
	structures int
	midpoint   int
	hl         int
	both       int
}

// init emits one descriptive diagnostic from the master report that already
// exists before btc-sequence-report runs. It performs no market-data fetch and
// changes no detector, entry, stop, target, hold, cost, or execution rule.
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

	rows := make([]bComponentConversionRow, 0, 4)
	s := bufio.NewScanner(f)
	for s.Scan() {
		m := bComponentIncidenceR.FindStringSubmatch(s.Text())
		if len(m) != 6 {
			continue
		}
		structures, err0 := strconv.Atoi(m[2])
		midpoint, err1 := strconv.Atoi(m[3])
		hl, err2 := strconv.Atoi(m[4])
		both, err3 := strconv.Atoi(m[5])
		if err0 != nil || err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		rows = append(rows, bComponentConversionRow{name: m[1], structures: structures, midpoint: midpoint, hl: hl, both: both})
	}
	if len(rows) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M B-component conjunction conversion by fixed era (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Among structures where each component appears within the frozen 8-bar window, shows how often that component participates in an actual same-candle BOTH/B event. No outcome filter or rule change.")
	for _, r := range rows {
		midConv := 0.0
		hlConv := 0.0
		if r.midpoint > 0 {
			midConv = float64(r.both) / float64(r.midpoint) * 100
		}
		if r.hl > 0 {
			hlConv = float64(r.both) / float64(r.hl) * 100
		}
		fmt.Printf("%-12s n=%3d | BOTH/midpoint %2d/%2d = %5.1f%% | BOTH/prospective-HL %2d/%2d = %5.1f%% | unmatched midpoint %2d | unmatched HL %2d\n",
			r.name, r.structures, r.both, r.midpoint, midConv, r.both, r.hl, hlConv, r.midpoint-r.both, r.hl-r.both)
	}
}
