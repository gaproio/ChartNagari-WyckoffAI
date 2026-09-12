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

// init appends one bounded tail-robust diagnostic for the already-fixed 10-18 UTC
// regime+era matched comparison. Within each common-support regime-by-era stratum,
// NetR is replaced by its within-stratum midrank and the observed session-label
// assignment is compared with every fixed-count assignment. This asks whether the
// matched session result survives discarding outcome magnitudes while preserving
// only ordinal information. It is descriptive only and changes no frozen rule.
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
	type rankTrade struct {
		netR   float64
		inside bool
	}
	byStratum := map[string][]rankTrade{}

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
		key := m[2] + "|" + rankFixedEra(ts.Year())
		byStratum[key] = append(byStratum[key], rankTrade{
			netR:   netR,
			inside: ts.Hour() >= 10 && ts.Hour() < 18,
		})
	}

	type rankStratum struct {
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

	strata := make([]rankStratum, 0, len(keys))
	for _, key := range keys {
		trades := byStratum[key]
		inN := 0
		for _, tr := range trades {
			if tr.inside {
				inN++
			}
		}
		outN := len(trades) - inN
		if inN == 0 || outN == 0 {
			continue
		}

		ranks := rankMidranks(trades)
		inSum, outSum := 0.0, 0.0
		for i, tr := range trades {
			if tr.inside {
				inSum += ranks[i]
			} else {
				outSum += ranks[i]
			}
		}
		obs := inSum/float64(inN) - outSum/float64(outN)
		strata = append(strata, rankStratum{name: key, vals: ranks, inN: inN, obs: obs})
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime+era matched exact rank diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Within each common-support regime+era stratum, replaces NetR magnitudes with within-stratum midranks, then exactly randomizes the fixed number of 10-18 labels. This bounded ordinal stress reduces sensitivity to extreme winners/losers; it is not a session/regime/era filter.")
	if len(strata) == 0 {
		fmt.Println("regime+era exact rank diagnostic unavailable: no common-support strata")
		return
	}

	const maxAssignments int64 = 2000000
	const eps = 1e-12
	observedSum := 0.0
	perStratum := make([][]float64, 0, len(strata))
	totalAssignments := int64(1)
	for _, st := range strata {
		diffs := rankEnumerateFixedCountDiffs(st.vals, st.inN)
		if len(diffs) == 0 || totalAssignments > maxAssignments/int64(len(diffs)) {
			fmt.Printf("exact rank enumeration unavailable within %d-assignment cap\n", maxAssignments)
			return
		}
		totalAssignments *= int64(len(diffs))
		perStratum = append(perStratum, diffs)
		observedSum += st.obs
		fmt.Printf("  %-24s n=%2d | 10-18 n=%2d | observed rank diff %+.3f\n", st.name, len(st.vals), st.inN, st.obs)
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

	fmt.Printf("observed equal-weight regime+era rank contrast: %+.3f across %d common-support strata\n", observed, len(strata))
	fmt.Printf("fixed-margin assignments enumerated: %d (expected %d)\n", visited, totalAssignments)
	fmt.Printf("exact one-sided P(random rank contrast >= observed) = %.4f\n", float64(geOne)/float64(visited))
	fmt.Printf("exact two-sided P(|random rank contrast| >= |observed|) = %.4f\n", float64(geTwo)/float64(visited))
}

func rankFixedEra(year int) string {
	switch {
	case year >= 2017 && year <= 2019:
		return "2017-2019"
	case year >= 2020 && year <= 2022:
		return "2020-2022"
	case year >= 2023 && year <= 2025:
		return "2023-2025"
	case year == 2026:
		return "2026 PARTIAL"
	default:
		return "OTHER"
	}
}

func rankMidranks[T interface{ ~struct{ netR float64; inside bool } }](trades []T) []float64 {
	return nil
}
