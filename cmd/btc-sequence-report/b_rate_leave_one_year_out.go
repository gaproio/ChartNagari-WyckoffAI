package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// init appends one bounded leave-one-calendar-year-out robustness diagnostic
// for the B-incidence gap between the two well-populated fixed eras. It uses
// only accepted/rejected V3 counts already present in the report. No detector,
// confirmation, execution, stop, target, holding, or filtering rule changes.
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

	type yearCounts struct {
		accepted int
		rejected int
	}
	counts := map[int]yearCounts{}
	yearHeader := regexp.MustCompile(`^(20[0-9]{2}):$`)
	countLine := regexp.MustCompile(`^  B_(ACCEPTED|REJECTED)\s+n=\s*([0-9]+)\b`)
	inSection := false
	currentYear := 0

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if line == "B decision selection by year (same common V3 next-open anchor):" {
			inSection = true
			currentYear = 0
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(line, "B confirmation latency study") {
			break
		}
		if m := yearHeader.FindStringSubmatch(line); len(m) == 2 {
			y, err := strconv.Atoi(m[1])
			if err == nil {
				currentYear = y
			}
			continue
		}
		m := countLine.FindStringSubmatch(line)
		if len(m) != 3 || currentYear == 0 {
			continue
		}
		n, err := strconv.Atoi(m[2])
		if err != nil || n < 0 {
			continue
		}
		c := counts[currentYear]
		if m[1] == "ACCEPTED" {
			c.accepted = n
		} else {
			c.rejected = n
		}
		counts[currentYear] = c
	}

	early := []int{2017, 2018, 2019}
	later := []int{2020, 2021, 2022}
	for _, years := range [][]int{early, later} {
		for _, y := range years {
			c, ok := counts[y]
			if !ok || c.accepted+c.rejected == 0 {
				return
			}
		}
	}

	pooledRateExcluding := func(years []int, omit int) float64 {
		a, n := 0, 0
		for _, y := range years {
			if y == omit {
				continue
			}
			c := counts[y]
			a += c.accepted
			n += c.accepted + c.rejected
		}
		return float64(a) / float64(n) * 100
	}

	minGap := 1e9
	maxGap := -1e9
	negativeOrZero := 0
	total := 0

	fmt.Println()
	fmt.Println("BTC 15M B-rate leave-one-year-out diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Drops one calendar year from each fixed era in all 3x3 combinations, then recomputes pooled BOTH/B incidence. This tests whether the era gap depends on any single year; no outcome filter or rule change.")
	for _, omitEarly := range early {
		for _, omitLater := range later {
			earlyRate := pooledRateExcluding(early, omitEarly)
			laterRate := pooledRateExcluding(later, omitLater)
			gap := laterRate - earlyRate
			if gap < minGap {
				minGap = gap
			}
			if gap > maxGap {
				maxGap = gap
			}
			if gap <= 0 {
				negativeOrZero++
			}
			total++
			fmt.Printf("omit %d / %d | early %5.1f%% | later %5.1f%% | later-early %+5.1f pp\n",
				omitEarly, omitLater, earlyRate, laterRate, gap)
		}
	}
	fmt.Printf("gap range across %d bounded omissions: %+5.1f to %+5.1f pp | later<=early in %d/%d combinations\n",
		total, minGap, maxGap, negativeOrZero, total)
}
