package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type boundaryBandStats struct {
	trades   int
	wins     int
	total    float64
	positive float64
	negative float64
}

func init() {
	path := sequenceReportInputPath(os.Args[1:])
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	var early boundaryBandStats // 08-10: present only in the original 08-16 window.
	var core boundaryBandStats  // 10-16: common to both 08-16 and 10-18.
	var late boundaryBandStats  // 16-18: present only in the shifted 10-18 window.

	s := bufio.NewScanner(f)
	for s.Scan() {
		m := tradeR.FindStringSubmatch(s.Text())
		if len(m) != 4 {
			continue
		}
		entryTime, err0 := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
		netR, err1 := strconv.ParseFloat(m[3], 64)
		if err0 != nil || err1 != nil {
			continue
		}

		hour := entryTime.Hour()
		switch {
		case hour >= 8 && hour < 10:
			addBoundaryBandTrade(&early, netR)
		case hour >= 10 && hour < 16:
			addBoundaryBandTrade(&core, netR)
		case hour >= 16 && hour < 18:
			addBoundaryBandTrade(&late, netR)
		}
	}

	if early.trades+core.trades+late.trades == 0 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M 08-16 vs 10-18 boundary replacement attribution (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Decomposes the two fixed 8-hour windows into the 08-10 band removed by the shift, their common 10-16 core, and the 16-18 band added by the shift. This explains the boundary perturbation without selecting or optimizing a session filter.")
	printBoundaryBand("08-10 BASE-ONLY", early)
	printBoundaryBand("10-16 COMMON", core)
	printBoundaryBand("16-18 SHIFT-ONLY", late)

	base := combineBoundaryBands(early, core)
	shifted := combineBoundaryBands(core, late)
	printBoundaryBand("08-16 REBUILT", base)
	printBoundaryBand("10-18 REBUILT", shifted)
	fmt.Printf("replacement delta: drop 08-10 %+.3fR, add 16-18 %+.3fR => 10-18 minus 08-16 %+.3fR\n",
		-early.total, late.total, shifted.total-base.total)
}

func sequenceReportInputPath(args []string) string {
	path := "research/btc15m/latest.txt"
	for i := 0; i < len(args); i++ {
		if args[i] == "-file" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], "-file=") {
			return strings.TrimPrefix(args[i], "-file=")
		}
	}
	return path
}

func addBoundaryBandTrade(s *boundaryBandStats, netR float64) {
	s.trades++
	s.total += netR
	if netR > 0 {
		s.wins++
		s.positive += netR
	} else if netR < 0 {
		s.negative += netR
	}
}

func combineBoundaryBands(a, b boundaryBandStats) boundaryBandStats {
	return boundaryBandStats{
		trades:   a.trades + b.trades,
		wins:     a.wins + b.wins,
		total:    a.total + b.total,
		positive: a.positive + b.positive,
		negative: a.negative + b.negative,
	}
}

func printBoundaryBand(label string, s boundaryBandStats) {
	avg := 0.0
	winRate := 0.0
	if s.trades > 0 {
		avg = s.total / float64(s.trades)
		winRate = float64(s.wins) / float64(s.trades) * 100
	}
	pf := "n/a"
	if s.negative < 0 {
		pf = fmt.Sprintf("%.2f", s.positive/-s.negative)
	} else if s.positive > 0 {
		pf = "inf"
	}
	fmt.Printf("%-18s n=%2d | net-win %.1f%% | total %+.3fR avg %+.3fR | PF %s\n",
		label, s.trades, winRate, s.total, avg, pf)
}
