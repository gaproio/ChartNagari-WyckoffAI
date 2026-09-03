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
// for B-confirmation incidence in the two well-populated fixed eras. It uses
// accepted/rejected structure counts only and does not inspect trade outcomes
// or change any detector, confirmation, execution, stop, target, or hold rule.
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

	type yc struct{ a, r int }
	counts := map[int]yc{}
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
			c.a = n
		} else {
			c.r = n
		}
		counts[currentYear] = c
	}

	early := []int{2017, 2018, 2019}
	later := []int{2020, 2021, 2022}
	complete := func(years []int) bool {
		for _, y := range years {
			c, ok := counts[y]
			if !ok || c.a+c.r == 0 {
				return false
			}
		}
		return true
	}
	if !complete(early) || !complete(later) {
		return
	}

	rateExcluding := func(years []int, omit int) float64 {
		a, n := 0, 0
		for _, y := range years {
			if y == omit {
				continue
			}
			c := counts[y]
			a += c.a
			n += c.a + c.r
		}
		return float64(a) / float64(n) * 100
	}

	fmt.Println()
	fmt.Println("BTC 15M B-rate leave-one-year-out robustness (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Recomputes pooled B incidence after omitting one calendar year at a time inside each well-populated fixed era. Accepted/(accepted+rejected) only; no outcome filter or threshold tuning.")
	for _, omitEarly := range early {
		e := rateExcluding(early, omitEarly)
		fmt.Printf("2017-2019 omit %d -> %5.1f%%\n", omitEarly, e)
	}
	for _, omitLater := range later {
		l := rateExcluding(later, omitLater)
		fmt.Printf("2020-2022 omit %d -> %5.1f%%\n", omitLater, l)
	}
	fmt.Println("Cross-era delta under matched single-year omissions (later minus early):")
	for i := 0; i < len(early); i++ {
		e := rateExcluding(early, early[i])
		l := rateExcluding(later, later[i])
		fmt.Printf("omit %d / %d -> %+5.1f pp\n", early[i], later[i], l-e)
	}
}
