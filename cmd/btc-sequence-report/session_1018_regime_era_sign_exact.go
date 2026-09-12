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
// regime+era comparison. It discards NetR magnitude entirely and retains only the
// sign of each frozen trade outcome, then exactly randomizes the fixed number of
// 10-18 labels within each common-support regime+era stratum. This asks whether
// the session association survives as a win/loss incidence effect rather than
// being carried by payoff magnitude. It is descriptive only and changes no
// detector, execution, cost, session, regime, era, or trading rule.
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
	type trade struct {
		positive bool
		inside   bool
	}
	byStratum := map[string][]trade{}

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
		key := m[2] + "|" + signFixedEra1018(ts.Year())
		byStratum[key] = append(byStratum[key], trade{
			positive: netR > 0,
			inside:   ts.Hour() >= 10 && ts.Hour() < 18,
		})
	}

	type stratum struct {
		name   string
		values []float64
		inN    int
		obs    float64
	}
	keys := make([]string, 0, len(byStratum))
	for key := range byStratum {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	strata := make([]stratum, 0, len(keys))
	for _, key := range keys {
		trades := byStratum[key]
		inN := 0
		inWins := 0
		outWins := 0
		values := make([]float64, len(trades))
		for i, tr := range trades {
			if tr.positive {
				values[i] = 1
			}
			if tr.inside {
				inN++
				if tr.positive {
					inWins++
				}
			} else if tr.positive {
				outWins++
			}
		}
		outN := len(trades) - inN
		if inN == 0 || outN == 0 {
			continue
		}
		obs := float64(inWins)/float64(inN) - float64(outWins)/float64(outN)
		strata = append(strata, stratum{name: key, values: values, inN: inN, obs: obs})
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime+era matched exact sign diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within each common-support regime+era stratum, reduces frozen NetR to positive versus non-positive outcome and exactly randomizes the fixed number of 10-18 labels. This bounded binary-outcome stress ignores payoff magnitude and changes no filter or trading rule.")
	if len(strata) == 0 {
		fmt.Println("regime+era exact sign diagnostic unavailable: no common-support strata")
		return
	}

	const maxAssignments int64 = 2000000
	const eps = 1e-12
	perStratum := make([][]float64, 0, len(strata))
	totalAssignments := int64(1)
	observedSum := 0.0
	for _, st := range strata {
		diffs := signEnumerateFixedCountDiffs1018(st.values, st.inN)
		if len(diffs) == 0 || totalAssignments > maxAssignments/int64(len(diffs)) {
			fmt.Printf("exact sign enumeration unavailable within %d-assignment cap\n", maxAssignments)
			return
		}
		totalAssignments *= int64(len(diffs))
		perStratum = append(perStratum, diffs)
		observedSum += st.obs
		fmt.Printf("  %-24s | n=%2d | 10-18 n=%2d | observed positive-rate diff %+.3f\n", st.name, len(st.values), st.inN, st.obs)
	}

	observed := observedSum / float64(len(strata))
	var geOne, geTwo, visited int64
	var combine func(int, float64)
	combine = func(idx int, sum float64) {
		if idx == len(perStratum) {
			stat := sum / float64(len(strata))
			visited++
			if stat >= observed-eps {
				geOne++
			}
			if math.Abs(stat) >= math.Abs(observed)-eps {
				geTwo++
			}
			return
		}
		for _, diff := range perStratum[idx] {
			combine(idx+1, sum+diff)
		}
	}
	combine(0, 0)

	fmt.Printf("observed equal-weight regime+era positive-rate contrast: %+.3f across %d common-support strata\n", observed, len(strata))
	fmt.Printf("fixed-margin assignments enumerated: %d (expected %d)\n", visited, totalAssignments)
	fmt.Printf("exact one-sided P(random sign contrast >= observed) = %.4f\n", float64(geOne)/float64(visited))
	fmt.Printf("exact two-sided P(|random sign contrast| >= |observed|) = %.4f\n", float64(geTwo)/float64(visited))
}

func signFixedEra1018(year int) string {
	switch {
	case year >= 2017 && year <= 2019:
		return "2017-2019"
	case year >= 2020 && year <= 2022:
		return "2020-2022"
	case year >= 2023 && year <= 2025:
		return "2023-2025"
	case year == 2026:
		return "2026-PARTIAL"
	default:
		return fmt.Sprintf("YEAR-%d", year)
	}
}

func signEnumerateFixedCountDiffs1018(values []float64, choose int) []float64 {
	n := len(values)
	if choose <= 0 || choose >= n {
		return nil
	}
	total := 0.0
	for _, v := range values {
		total += v
	}
	outN := n - choose
	diffs := make([]float64, 0)
	var rec func(int, int, float64)
	rec = func(start int, left int, inSum float64) {
		if left == 0 {
			outSum := total - inSum
			diffs = append(diffs, inSum/float64(choose)-outSum/float64(outN))
			return
		}
		if n-start < left {
			return
		}
		for i := start; i <= n-left; i++ {
			rec(i+1, left-1, inSum+values[i])
		}
	}
	rec(0, choose, 0)
	return diffs
}
