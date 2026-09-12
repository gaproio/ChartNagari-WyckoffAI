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
	"time"
)

// init appends one bounded exact randomization diagnostic for the already-fixed
// 10-18 UTC session contrast while controlling jointly for the existing causal
// 30D market regime and broad, predeclared calendar eras. Within each
// regime-by-era stratum that has common support, the observed number of 10-18
// trades is held fixed and session labels are exactly reassigned. This is a
// descriptive robustness check only; it does not change the frozen detector,
// execution, session boundary, costs, or any trading rule.
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

	tradeLine := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s+\|\s+([A-Z0-9_]+)\s+.*\|.* gross [+-]?[0-9]+(?:\.[0-9]+)?R net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)
	type reTrade struct {
		netR   float64
		inside bool
	}
	byStratum := map[string][]reTrade{}

	s := bufio.NewScanner(f)
	for s.Scan() {
		m := tradeLine.FindStringSubmatch(s.Text())
		if len(m) != 4 {
			continue
		}
		ts, err0 := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
		netR, err1 := strconv.ParseFloat(m[3], 64)
		if err0 != nil || err1 != nil {
			continue
		}
		era := reFixedEra(ts.Year())
		key := m[2] + "|" + era
		byStratum[key] = append(byStratum[key], reTrade{
			netR:   netR,
			inside: ts.Hour() >= 10 && ts.Hour() < 18,
		})
	}

	type reStratum struct {
		name string
		vals []float64
		inN  int
		obs  float64
	}
	keys := make([]string, 0, len(byStratum))
	for key := range byStratum {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	strata := make([]reStratum, 0, len(keys))
	observedSum := 0.0
	for _, key := range keys {
		trades := byStratum[key]
		vals := make([]float64, 0, len(trades))
		inN := 0
		inSum := 0.0
		outSum := 0.0
		for _, tr := range trades {
			vals = append(vals, tr.netR)
			if tr.inside {
				inN++
				inSum += tr.netR
			} else {
				outSum += tr.netR
			}
		}
		outN := len(trades) - inN
		if inN == 0 || outN == 0 {
			continue
		}
		obs := inSum/float64(inN) - outSum/float64(outN)
		strata = append(strata, reStratum{name: key, vals: vals, inN: inN, obs: obs})
		observedSum += obs
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime+era matched exact randomization (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Strata are causal 30D regime x fixed calendar era (2017-19, 2020-22, 2023-25, 2026 partial). Within each common-support stratum, holds the observed 10-18 trade count fixed and exactly enumerates session-label assignments. Statistic is the equal-weight mean within-stratum NetR contrast; no session/regime/era filter is created.")
	if len(strata) == 0 {
		fmt.Println("regime+era exact randomization unavailable: no stratum has trades both inside and outside 10-18 UTC")
		return
	}

	observed := observedSum / float64(len(strata))
	perStratum := make([][]float64, len(strata))
	totalAssignments := int64(1)
	const maxAssignments int64 = 2000000
	for i, st := range strata {
		perStratum[i] = reEnumerateFixedCountDiffs(st.vals, st.inN)
		if len(perStratum[i]) == 0 {
			fmt.Printf("regime+era exact randomization unavailable for %s\n", st.name)
			return
		}
		if totalAssignments > maxAssignments/int64(len(perStratum[i])) {
			fmt.Printf("regime+era exact randomization bounded at %d assignments; required space exceeds cap\n", maxAssignments)
			return
		}
		totalAssignments *= int64(len(perStratum[i]))
	}

	const eps = 1e-12
	var geOneSided int64
	var geTwoSided int64
	var visited int64
	var combine func(int, float64)
	combine = func(idx int, sum float64) {
		if idx == len(perStratum) {
			stat := sum / float64(len(perStratum))
			visited++
			if stat >= observed-eps {
				geOneSided++
			}
			if math.Abs(stat) >= math.Abs(observed)-eps {
				geTwoSided++
			}
			return
		}
		for _, diff := range perStratum[idx] {
			combine(idx+1, sum+diff)
		}
	}
	combine(0, 0)

	fmt.Printf("common-support regime+era strata: %d\n", len(strata))
	for _, st := range strata {
		fmt.Printf("  %-24s n=%2d | 10-18 n=%2d | observed diff %+.3fR\n", st.name, len(st.vals), st.inN, st.obs)
	}
	fmt.Printf("observed equal-weight regime+era contrast: %+.3fR\n", observed)
	fmt.Printf("fixed-margin assignments enumerated: %d (expected %d)\n", visited, totalAssignments)
	fmt.Printf("exact one-sided P(random contrast >= observed) = %.4f\n", float64(geOneSided)/float64(visited))
	fmt.Printf("exact two-sided P(|random contrast| >= |observed|) = %.4f\n", float64(geTwoSided)/float64(visited))
}

func reFixedEra(year int) string {
	switch {
	case year <= 2019:
		return "2017-2019"
	case year <= 2022:
		return "2020-2022"
	case year <= 2025:
		return "2023-2025"
	default:
		return "2026-PARTIAL"
	}
}

// reEnumerateFixedCountDiffs returns the inside-minus-outside mean difference
// for every subset of size inN within one fixed regime-by-era stratum.
func reEnumerateFixedCountDiffs(vals []float64, inN int) []float64 {
	if inN <= 0 || inN >= len(vals) {
		return nil
	}
	total := 0.0
	for _, v := range vals {
		total += v
	}
	outN := len(vals) - inN
	out := make([]float64, 0)
	var choose func(start, left int, inSum float64)
	choose = func(start, left int, inSum float64) {
		if left == 0 {
			outSum := total - inSum
			out = append(out, inSum/float64(inN)-outSum/float64(outN))
			return
		}
		last := len(vals) - left
		for i := start; i <= last; i++ {
			choose(i+1, left-1, inSum+vals[i])
		}
	}
	choose(0, inN, 0)
	return out
}
