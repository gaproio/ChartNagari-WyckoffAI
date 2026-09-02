package main

import (
	"bufio"
	"fmt"
	"os"
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

	counts := map[int]bYearCounts{}
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
		if m := bYearHeaderR.FindStringSubmatch(line); len(m) == 2 {
			y, err := strconv.Atoi(m[1])
			if err == nil {
				currentYear = y
			}
			continue
		}
		m := bYearCountR.FindStringSubmatch(line)
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
	if !completeBYearEra(counts, early) || !completeBYearEra(counts, later) {
		return
	}

	earlyPooled, earlyBalanced := bYearBalancedRates(counts, early)
	laterPooled, laterBalanced := bYearBalancedRates(counts, later)

	fmt.Println()
	fmt.Println("BTC 15M B-rate calendar-year-balanced diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Compares pooled B incidence with an equal-weight mean of each calendar year's incidence inside the two fixed eras. Accepted/(accepted+rejected) only; no outcome filter or era rule.")
	printBYearBalancedEra("2017-2019", early, counts, earlyPooled, earlyBalanced)
	printBYearBalancedEra("2020-2022", later, counts, laterPooled, laterBalanced)
	fmt.Printf("2020-2022 minus 2017-2019: pooled %+5.1f pp | year-balanced %+5.1f pp\n",
		laterPooled-earlyPooled, laterBalanced-earlyBalanced)
}

func bYearBalancedRates(counts map[int]bYearCounts, years []int) (pooled, balanced float64) {
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

func printBYearBalancedEra(name string, years []int, counts map[int]bYearCounts, pooled, balanced float64) {
	fmt.Printf("%-9s pooled %5.1f%% | year-balanced %5.1f%% | yearly", name, pooled, balanced)
	for _, y := range years {
		c := counts[y]
		n := c.Accepted + c.Rejected
		fmt.Printf(" %d %d/%d=%.1f%%", y, c.Accepted, n, float64(c.Accepted)/float64(n)*100)
	}
	fmt.Println()
}
