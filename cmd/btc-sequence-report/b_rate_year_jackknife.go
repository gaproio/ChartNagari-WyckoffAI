package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	bYearHeaderR = regexp.MustCompile(`^(20\d{2}):$`)
	bYearCountR  = regexp.MustCompile(`^\s+B_(ACCEPTED|REJECTED)\s+n=\s*(\d+)\b`)
)

type bYearCounts struct {
	Accepted int
	Rejected int
}

// init appends one bounded leave-one-calendar-year-out incidence check for the
// two well-populated fixed eras already used by the B-component diagnostics.
// It reads only accepted/rejected V3 counts from the existing report; it does
// not inspect outcomes and changes no detector, confirmation, or execution rule.
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

	fmt.Println()
	fmt.Println("BTC 15M B-rate calendar-year jackknife by fixed era (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Leaves out one calendar year at a time from 2017-2019 and 2020-2022 B incidence. Accepted/(accepted+rejected) only; no outcome filter or era rule.")
	printBYearJackknifeEra("2017-2019", early, counts)
	printBYearJackknifeEra("2020-2022", later, counts)
}

func completeBYearEra(counts map[int]bYearCounts, years []int) bool {
	for _, y := range years {
		c, ok := counts[y]
		if !ok || c.Accepted+c.Rejected <= 0 {
			return false
		}
	}
	return true
}

func printBYearJackknifeEra(name string, years []int, counts map[int]bYearCounts) {
	baseA, baseN := 0, 0
	for _, y := range years {
		c := counts[y]
		baseA += c.Accepted
		baseN += c.Accepted + c.Rejected
	}
	if baseN <= 0 {
		return
	}
	fmt.Printf("%-9s base %2d/%2d = %5.1f%%\n", name, baseA, baseN, float64(baseA)/float64(baseN)*100)
	minRate, maxRate := 101.0, -1.0
	for _, omitted := range years {
		c := counts[omitted]
		a := baseA - c.Accepted
		n := baseN - c.Accepted - c.Rejected
		if n <= 0 {
			continue
		}
		rate := float64(a) / float64(n) * 100
		if rate < minRate {
			minRate = rate
		}
		if rate > maxRate {
			maxRate = rate
		}
		fmt.Printf("  leave out %d: %2d/%2d = %5.1f%% | removed %d/%d\n",
			omitted, a, n, rate, c.Accepted, c.Accepted+c.Rejected)
	}
	if maxRate >= 0 && minRate <= 100 {
		fmt.Printf("  jackknife range %.1f%% to %.1f%%\n", minRate, maxRate)
	}
}
