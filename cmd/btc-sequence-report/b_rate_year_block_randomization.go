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

// init appends one bounded calendar-year block randomization diagnostic for the
// 2017-2019 versus 2020-2022 B-incidence gap. It treats each calendar year as a
// block and enumerates all 20 ways to assign three of the six years to one era,
// preserving within-year clustering. This is descriptive robustness only: no
// detector, confirmation, execution, stop, target, holding, or filter changes.
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

	years := []int{2017, 2018, 2019, 2020, 2021, 2022}
	for _, y := range years {
		c, ok := counts[y]
		if !ok || c.accepted+c.rejected == 0 {
			return
		}
	}

	rate := func(group []int) float64 {
		a, n := 0, 0
		for _, y := range group {
			c := counts[y]
			a += c.accepted
			n += c.accepted + c.rejected
		}
		return float64(a) / float64(n) * 100
	}

	observedEarly := []int{2017, 2018, 2019}
	observedLater := []int{2020, 2021, 2022}
	observedGap := rate(observedLater) - rate(observedEarly)
	obsAbs := math.Abs(observedGap)

	var gaps []float64
	extreme := 0
	for i := 0; i < len(years)-2; i++ {
		for j := i + 1; j < len(years)-1; j++ {
			for k := j + 1; k < len(years); k++ {
				left := []int{years[i], years[j], years[k]}
				inLeft := map[int]bool{years[i]: true, years[j]: true, years[k]: true}
				right := make([]int, 0, 3)
				for _, y := range years {
					if !inLeft[y] {
						right = append(right, y)
					}
				}
				gap := rate(right) - rate(left)
				gaps = append(gaps, gap)
				if math.Abs(gap)+1e-12 >= obsAbs {
					extreme++
				}
			}
		}
	}

	sort.Float64s(gaps)
	if len(gaps) != 20 {
		return
	}
	median := (gaps[9] + gaps[10]) / 2

	fmt.Println()
	fmt.Println("BTC 15M B-rate calendar-year block randomization diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Enumerates all 20 ways to split 2017-2022 into two 3-year blocks, preserving each year's accepted/rejected counts. The observed era gap is compared with this bounded year-block null; no outcome filter or rule change.")
	fmt.Printf("observed 2020-2022 minus 2017-2019: %+5.1f pp | all-split gap range %+5.1f to %+5.1f pp | median %+5.1f pp\n",
		observedGap, gaps[0], gaps[len(gaps)-1], median)
	fmt.Printf("two-sided block-randomization extremeness: %d/%d assignments have |gap| >= observed |gap| (exact enumeration proportion %.3f)\n",
		extreme, len(gaps), float64(extreme)/float64(len(gaps)))
}
