package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// init appends one bounded monotonic-trend robustness diagnostic for yearly
// B-confirmation incidence across 2017-2022. It uses equal-weight calendar-year
// rates and an exact six-year rank permutation; no detector, confirmation,
// execution, stop, target, hold, or baseline rule is changed.
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

	type trendYearCount struct{ accepted, rejected int }
	counts := map[int]trendYearCount{}
	yearHeader := regexp.MustCompile(`^(20[0-9]{2}):$`)
	countLine := regexp.MustCompile(`^  B_(ACCEPTED|REJECTED)\s+n=\s*([0-9]+)\b`)
	inSection, currentYear := false, 0

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
			currentYear, _ = strconv.Atoi(m[1])
			continue
		}
		if m := countLine.FindStringSubmatch(line); len(m) == 3 && currentYear != 0 {
			n, e := strconv.Atoi(m[2])
			if e != nil {
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
	}

	years := []int{2017, 2018, 2019, 2020, 2021, 2022}
	rates := make([]float64, len(years))
	for i, year := range years {
		c, ok := counts[year]
		if !ok || c.accepted+c.rejected == 0 {
			return
		}
		rates[i] = float64(c.accepted) / float64(c.accepted+c.rejected) * 100
	}

	// Average ranks make the diagnostic well-defined even if two yearly rates tie.
	rank := func(values []float64) []float64 {
		idx := make([]int, len(values))
		for i := range idx {
			idx[i] = i
		}
		sort.Slice(idx, func(i, j int) bool { return values[idx[i]] < values[idx[j]] })
		out := make([]float64, len(values))
		for start := 0; start < len(idx); {
			end := start + 1
			for end < len(idx) && math.Abs(values[idx[end]]-values[idx[start]]) < 1e-12 {
				end++
			}
			avgRank := (float64(start+1) + float64(end)) / 2.0
			for j := start; j < end; j++ {
				out[idx[j]] = avgRank
			}
			start = end
		}
		return out
	}

	corr := func(x, y []float64) float64 {
		mx, my := 0.0, 0.0
		for i := range x {
			mx += x[i]
			my += y[i]
		}
		mx /= float64(len(x))
		my /= float64(len(y))
		num, dx, dy := 0.0, 0.0, 0.0
		for i := range x {
			a, b := x[i]-mx, y[i]-my
			num += a * b
			dx += a * a
			dy += b * b
		}
		if dx == 0 || dy == 0 {
			return 0
		}
		return num / math.Sqrt(dx*dy)
	}

	yearRanks := []float64{1, 2, 3, 4, 5, 6}
	rateRanks := rank(rates)
	observed := corr(yearRanks, rateRanks)

	permuted := append([]float64(nil), rateRanks...)
	total, asExtreme := 0, 0
	var enumerate func(int)
	enumerate = func(pos int) {
		if pos == len(permuted) {
			rho := corr(yearRanks, permuted)
			total++
			if math.Abs(rho)+1e-12 >= math.Abs(observed) {
				asExtreme++
			}
			return
		}
		for i := pos; i < len(permuted); i++ {
			permuted[pos], permuted[i] = permuted[i], permuted[pos]
			enumerate(pos + 1)
			permuted[pos], permuted[i] = permuted[i], permuted[pos]
		}
	}
	enumerate(0)

	fmt.Println()
	fmt.Println("BTC 15M B-rate six-year monotonic-trend permutation diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Uses equal-weight calendar-year B incidence for 2017-2022 and an exact permutation of the six yearly ranks. This tests chronological monotonic association only; no outcome filter, threshold tuning, or baseline change.")
	for i, year := range years {
		fmt.Printf("%d B incidence: %.1f%%\n", year, rates[i])
	}
	fmt.Printf("Observed Spearman rho(year, B incidence): %+.3f\n", observed)
	fmt.Printf("Exact two-sided rank permutations: abs(rho) >= observed in %d/%d (%.1f%%)\n", asExtreme, total, float64(asExtreme)/float64(total)*100)
	fmt.Println("Interpretation: this complements the contiguous-window check by asking whether the six annual rates form a broadly monotonic chronological trend rather than relying on a single era boundary. Small year count means descriptive robustness only.")
}
