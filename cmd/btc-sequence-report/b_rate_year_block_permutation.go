package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// init appends one bounded year-block permutation diagnostic for the observed
// 2017-2019 versus 2020-2022 B-incidence shift. It treats each calendar year
// as one block and exhaustively compares the observed 3-vs-3 split with all
// 20 possible allocations. It changes no detector or trading rule.
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
	yearR := regexp.MustCompile(`^(20[0-9]{2}):$`)
	countR := regexp.MustCompile(`^  B_(ACCEPTED|REJECTED)\s+n=\s*([0-9]+)\b`)
	inSection, year := false, 0
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if line == "B decision selection by year (same common V3 next-open anchor):" {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(line, "B confirmation latency study") {
			break
		}
		if m := yearR.FindStringSubmatch(line); len(m) == 2 {
			year, _ = strconv.Atoi(m[1])
			continue
		}
		if m := countR.FindStringSubmatch(line); len(m) == 3 && year != 0 {
			n, e := strconv.Atoi(m[2])
			if e != nil {
				continue
			}
			c := counts[year]
			if m[1] == "ACCEPTED" { c.a = n } else { c.r = n }
			counts[year] = c
		}
	}

	years := []int{2017, 2018, 2019, 2020, 2021, 2022}
	for _, y := range years {
		c, ok := counts[y]
		if !ok || c.a+c.r == 0 {
			return
		}
	}
	rate := func(y int) float64 {
		c := counts[y]
		return float64(c.a) / float64(c.a+c.r) * 100
	}
	mean := func(ix []int) float64 {
		sum := 0.0
		for _, i := range ix { sum += rate(years[i]) }
		return sum / float64(len(ix))
	}

	observed := mean([]int{3,4,5}) - mean([]int{0,1,2})
	total, asExtreme := 0, 0
	minDelta, maxDelta := math.Inf(1), math.Inf(-1)
	for i := 0; i < 4; i++ {
		for j := i+1; j < 5; j++ {
			for k := j+1; k < 6; k++ {
				late := []int{i,j,k}
				isLate := [6]bool{}
				isLate[i], isLate[j], isLate[k] = true, true, true
				early := make([]int,0,3)
				for x := 0; x < 6; x++ { if !isLate[x] { early = append(early,x) } }
				d := mean(late) - mean(early)
				total++
				if math.Abs(d)+1e-12 >= math.Abs(observed) { asExtreme++ }
				if d < minDelta { minDelta = d }
				if d > maxDelta { maxDelta = d }
			}
		}
	}

	fmt.Println()
	fmt.Println("BTC 15M B-rate calendar-year block permutation diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Treats 2017-2022 calendar years as six fixed blocks and exhaustively reallocates three years per side. Uses equal-weight yearly B incidence only; no outcome filter, threshold tuning, or baseline change.")
	fmt.Printf("Observed chronological 2020-2022 minus 2017-2019 year-balanced delta: %+.1f pp\n", observed)
	fmt.Printf("All 3-vs-3 year allocations: %d | abs(delta) >= observed: %d/%d (%.1f%%) | permutation delta range %+.1f to %+.1f pp\n", total, asExtreme, total, float64(asExtreme)/float64(total)*100, minDelta, maxDelta)
	fmt.Println("Interpretation: small six-year block count makes this a bounded robustness/placebo diagnostic, not a significance gate.")
}
