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

// init appends one bounded robustness diagnostic for the already-fixed 10-18 UTC
// regime+era matched exact randomization. It repeats the exact fixed-margin test
// while omitting each common-support regime-by-era stratum once. This checks
// whether the matched-session result is dominated by a single stratum. It is
// descriptive only and does not change the frozen detector, execution, costs,
// session boundary, regime definition, era definition, or any trading rule.
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
	type jkTrade struct {
		netR   float64
		inside bool
	}
	byStratum := map[string][]jkTrade{}

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
		key := m[2] + "|" + reFixedEra(ts.Year())
		byStratum[key] = append(byStratum[key], jkTrade{
			netR:   netR,
			inside: ts.Hour() >= 10 && ts.Hour() < 18,
		})
	}

	type jkStratum struct {
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

	strata := make([]jkStratum, 0, len(keys))
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
		strata = append(strata, jkStratum{name: key, vals: vals, inN: inN, obs: obs})
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime+era exact leave-one-stratum-out stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Repeats the fixed-margin regime+era exact randomization after omitting each common-support stratum once. Statistic remains the equal-weight mean within-stratum NetR contrast; this is a concentration diagnostic, not a filter or parameter search.")
	if len(strata) < 3 {
		fmt.Printf("regime+era exact jackknife unavailable: need at least 3 common-support strata, found %d\n", len(strata))
		return
	}

	const maxAssignments int64 = 2000000
	const eps = 1e-12
	for omit := range strata {
		observedSum := 0.0
		used := 0
		perStratum := make([][]float64, 0, len(strata)-1)
		totalAssignments := int64(1)
		bounded := false

		for i, st := range strata {
			if i == omit {
				continue
			}
			diffs := reEnumerateFixedCountDiffs(st.vals, st.inN)
			if len(diffs) == 0 {
				bounded = true
				break
			}
			if totalAssignments > maxAssignments/int64(len(diffs)) {
				bounded = true
				break
			}
			totalAssignments *= int64(len(diffs))
			perStratum = append(perStratum, diffs)
			observedSum += st.obs
			used++
		}

		if bounded || used == 0 {
			fmt.Printf("omit %-24s | exact enumeration unavailable within %d-assignment cap\n", strata[omit].name, maxAssignments)
			continue
		}

		observed := observedSum / float64(used)
		var geTwoSided int64
		var visited int64
		var combine func(int, float64)
		combine = func(idx int, sum float64) {
			if idx == len(perStratum) {
				stat := sum / float64(used)
				visited++
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

		fmt.Printf("omit %-24s | strata %d | observed %+.3fR | assignments %d | exact two-sided p=%.4f\n",
			strata[omit].name, used, observed, visited, float64(geTwoSided)/float64(visited))
	}
}
