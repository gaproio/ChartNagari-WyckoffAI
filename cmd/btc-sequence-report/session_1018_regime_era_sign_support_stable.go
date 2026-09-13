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

// init appends one bounded robustness diagnostic motivated by the prior
// single-trade influence result. The existing regime+era sign comparison has
// several 1-vs-1 strata whose common support disappears after one deletion.
// This check therefore repeats the same fixed-margin sign randomization using
// only strata with at least two observations on BOTH session sides. The 2-per-
// side rule is a minimal support-stability requirement, not an outcome-tuned
// trading filter. Frozen detector, execution, costs, and 10-18 UTC definition
// remain unchanged.
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

	type stableStratum struct {
		name   string
		values []float64
		inN    int
		outN   int
		obs    float64
	}
	keys := make([]string, 0, len(byStratum))
	for key := range byStratum {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	stable := make([]stableStratum, 0)
	for _, key := range keys {
		trades := byStratum[key]
		inN, inWins, outWins := 0, 0, 0
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
		if inN < 2 || outN < 2 {
			continue
		}
		obs := float64(inWins)/float64(inN) - float64(outWins)/float64(outN)
		stable = append(stable, stableStratum{name: key, values: values, inN: inN, outN: outN, obs: obs})
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime+era support-stable exact sign diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Repeats the existing fixed-margin positive/non-positive randomization only in regime+era strata with at least 2 trades inside AND 2 outside 10-18 UTC. This is a bounded common-support stability check prompted by one-trade-fragile strata; it does not create a trading filter.")
	if len(stable) == 0 {
		fmt.Println("support-stable exact sign diagnostic unavailable: no strata have >=2 observations on both session sides")
		return
	}

	const maxAssignments int64 = 2000000
	const eps = 1e-12
	perStratum := make([][]float64, 0, len(stable))
	totalAssignments := int64(1)
	observedSum := 0.0
	for _, st := range stable {
		diffs := signEnumerateFixedCountDiffs1018(st.values, st.inN)
		if len(diffs) == 0 || totalAssignments > maxAssignments/int64(len(diffs)) {
			fmt.Printf("support-stable exact sign enumeration unavailable within %d-assignment cap\n", maxAssignments)
			return
		}
		totalAssignments *= int64(len(diffs))
		perStratum = append(perStratum, diffs)
		observedSum += st.obs
		fmt.Printf("  %-24s | n=%2d | 10-18 n=%2d | outside n=%2d | observed positive-rate diff %+.3f\n", st.name, len(st.values), st.inN, st.outN, st.obs)
	}

	observed := observedSum / float64(len(stable))
	var visited, geOne, geTwo int64
	var combine func(int, float64)
	combine = func(idx int, sum float64) {
		if idx == len(perStratum) {
			stat := sum / float64(len(stable))
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

	fmt.Printf("support-stable strata retained: %d of the original common-support set\n", len(stable))
	fmt.Printf("observed equal-weight support-stable positive-rate contrast: %+.3f\n", observed)
	fmt.Printf("fixed-margin assignments enumerated: %d (expected %d)\n", visited, totalAssignments)
	fmt.Printf("exact one-sided P(random sign contrast >= observed) = %.4f\n", float64(geOne)/float64(visited))
	fmt.Printf("exact two-sided P(|random sign contrast| >= |observed|) = %.4f\n", float64(geTwo)/float64(visited))
}
