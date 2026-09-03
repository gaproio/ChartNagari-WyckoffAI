package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// init appends one bounded calendar-year-balanced B-incidence diagnostic for
// the two well-populated fixed eras. It uses only accepted/rejected V3 counts
// already present in the report and does not inspect trade outcomes or alter
// any detector, confirmation, execution, stop, target, or holding rule.
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
		Accepted int
		Rejected int
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
			c.Accepted = n
		} else {
			c.Rejected = n
		}
		counts[currentYear] = c
	}

	early := []int{2017, 2018, 2019}
	later := []int{2020, 2021, 2022}
	complete := func(years []int) bool {
		for _, y := range years {
			c, ok := counts[y]
			if !ok || c.Accepted+c.Rejected == 0 {
				return false
			}
		}
		return true
	}
	if !complete(early) || !complete(later) {
		return
	}

	balancedRates := func(years []int) (pooled, balanced float64) {
		totalA, totalN := 0, 0
		sumYearRate := 0.0
		for _, y := range years {
			c := counts[y]
			n := c.Accepted + c.Rejected
			totalA += c.Accepted
			totalN += n
			sumYearRate += float64(c.Accepted) / float64(n) * 100
		}
		return float64(totalA) / float64(totalN) * 100, sumYearRate / float64(len(years))
	}

	printEra := func(name string, years []int, pooled, balanced float64) {
		fmt.Printf("%-9s pooled %5.1f%% | year-balanced %5.1f%% | yearly", name, pooled, balanced)
		for _, y := range years {
			c := counts[y]
			n := c.Accepted + c.Rejected
			fmt.Printf(" %d %d/%d=%.1f%%", y, c.Accepted, n, float64(c.Accepted)/float64(n)*100)
		}
		fmt.Println()
	}

	earlyPooled, earlyBalanced := balancedRates(early)
	laterPooled, laterBalanced := balancedRates(later)

	fmt.Println()
	fmt.Println("BTC 15M B-rate calendar-year-balanced diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Compares pooled B incidence with an equal-weight mean of each calendar year's incidence inside the two fixed eras. Accepted/(accepted+rejected) only; no outcome filter or era rule.")
	printEra("2017-2019", early, earlyPooled, earlyBalanced)
	printEra("2020-2022", later, laterPooled, laterBalanced)
	fmt.Printf("2020-2022 minus 2017-2019: pooled %+5.1f pp | year-balanced %+5.1f pp\n",
		laterPooled-earlyPooled, laterBalanced-earlyBalanced)
}
