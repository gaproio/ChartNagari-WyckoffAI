package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// init appends one bounded exact randomization diagnostic for the already-fixed
// 10-18 UTC regime-matched contrast. Within each existing causal 30D regime it
// keeps the observed number of 10-18 trades fixed and enumerates every possible
// reassignment of that label among the regime's frozen trades. The statistic is
// the same equal-weight mean of within-regime NetR differences used by the prior
// diagnostic. This is descriptive evidence under an exchangeability null, not a
// tradable filter or a change to any frozen detector/execution rule.
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
	type rrTrade struct {
		netR   float64
		inside bool
	}
	byRegime := map[string][]rrTrade{}

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
		byRegime[m[2]] = append(byRegime[m[2]], rrTrade{
			netR:   netR,
			inside: ts.Hour() >= 10 && ts.Hour() < 18,
		})
	}

	regimeOrder := []string{"BEAR_30D", "SIDEWAYS_30D", "BULL_30D", "UNKNOWN"}
	type rrRegime struct {
		name string
		vals []float64
		inN  int
		obs  float64
	}
	regimes := make([]rrRegime, 0, len(regimeOrder))
	observedSum := 0.0

	for _, name := range regimeOrder {
		trades := byRegime[name]
		if len(trades) == 0 {
			continue
		}
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
		regimes = append(regimes, rrRegime{name: name, vals: vals, inN: inN, obs: obs})
		observedSum += obs
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime-matched exact randomization (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within each common-support 30D regime, holds the observed 10-18 trade count fixed and exactly enumerates all session-label assignments. Statistic is the prior equal-weight within-regime mean-NetR contrast. This tests an exchangeability null only; it does not create a session/regime filter.")
	if len(regimes) == 0 {
		fmt.Println("exact randomization unavailable: no regime has trades both inside and outside 10-18 UTC")
		return
	}

	observed := observedSum / float64(len(regimes))
	perRegime := make([][]float64, len(regimes))
	totalAssignments := int64(1)
	for i, rg := range regimes {
		perRegime[i] = rrEnumerateFixedCountDiffs(rg.vals, rg.inN)
		if len(perRegime[i]) == 0 {
			fmt.Printf("exact randomization unavailable for %s\n", rg.name)
			return
		}
		totalAssignments *= int64(len(perRegime[i]))
	}

	const eps = 1e-12
	var geOneSided int64
	var geTwoSided int64
	var visited int64
	var combine func(int, float64)
	combine = func(idx int, sum float64) {
		if idx == len(perRegime) {
			stat := sum / float64(len(perRegime))
			visited++
			if stat >= observed-eps {
				geOneSided++
			}
			if math.Abs(stat) >= math.Abs(observed)-eps {
				geTwoSided++
			}
			return
		}
		for _, diff := range perRegime[idx] {
			combine(idx+1, sum+diff)
		}
	}
	combine(0, 0)

	fmt.Printf("observed equal-weight contrast: %+.3fR across %d common-support regimes\n", observed, len(regimes))
	fmt.Printf("fixed-margin assignments enumerated: %d (expected %d)\n", visited, totalAssignments)
	fmt.Printf("exact one-sided P(random contrast >= observed) = %.4f\n", float64(geOneSided)/float64(visited))
	fmt.Printf("exact two-sided P(|random contrast| >= |observed|) = %.4f\n", float64(geTwoSided)/float64(visited))
}

// rrEnumerateFixedCountDiffs returns the inside-minus-outside mean difference
// for every subset of size inN. Names are deliberately prefixed to avoid
// collisions with other package-level research helpers.
func rrEnumerateFixedCountDiffs(vals []float64, inN int) []float64 {
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
