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
// regime+era matched exact rank comparison. It repeats the ordinal fixed-margin
// test while omitting each common-support regime-by-era stratum once. This asks
// whether the rank-based session result is dominated by one stratum after NetR
// magnitudes have already been discarded. It is descriptive only and changes no
// frozen detector, execution, cost, session, regime, era, or trading rule.
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
		netR   float64
		inside bool
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
		key := m[2] + "|" + rankFixedEra(ts.Year())
		byStratum[key] = append(byStratum[key], trade{
			netR:   netR,
			inside: ts.Hour() >= 10 && ts.Hour() < 18,
		})
	}

	type stratum struct {
		name string
		ranks []float64
		inN  int
		obs  float64
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
		for _, tr := range trades {
			if tr.inside {
				inN++
			}
		}
		outN := len(trades) - inN
		if inN == 0 || outN == 0 {
			continue
		}

		order := make([]int, len(trades))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool {
			return trades[order[i]].netR < trades[order[j]].netR
		})
		ranks := make([]float64, len(trades))
		for start := 0; start < len(order); {
			end := start + 1
			for end < len(order) && trades[order[end]].netR == trades[order[start]].netR {
				end++
			}
			midrank := (float64(start+1) + float64(end)) / 2.0
			for pos := start; pos < end; pos++ {
				ranks[order[pos]] = midrank
			}
			start = end
		}

		inSum, outSum := 0.0, 0.0
		for i, tr := range trades {
			if tr.inside {
				inSum += ranks[i]
			} else {
				outSum += ranks[i]
			}
		}
		obs := inSum/float64(inN) - outSum/float64(outN)
		strata = append(strata, stratum{name: key, ranks: ranks, inN: inN, obs: obs})
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime+era exact rank leave-one-stratum-out stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Repeats the fixed-margin within-stratum rank randomization after omitting each common-support regime+era stratum once. This bounded ordinal concentration stress discards NetR magnitudes and changes no filter or trading rule.")
	if len(strata) < 3 {
		fmt.Printf("regime+era exact rank jackknife unavailable: need at least 3 common-support strata, found %d\n", len(strata))
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
			diffs := rankEnumerateFixedCountDiffs(st.ranks, st.inN)
			if len(diffs) == 0 || totalAssignments > maxAssignments/int64(len(diffs)) {
				bounded = true
				break
			}
			totalAssignments *= int64(len(diffs))
			perStratum = append(perStratum, diffs)
			observedSum += st.obs
			used++
		}

		if bounded || used == 0 {
			fmt.Printf("omit %-24s | exact rank enumeration unavailable within %d-assignment cap\n", strata[omit].name, maxAssignments)
			continue
		}

		observed := observedSum / float64(used)
		var geTwo, visited int64
		var combine func(int, float64)
		combine = func(idx int, sum float64) {
			if idx == len(perStratum) {
				stat := sum / float64(used)
				visited++
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

		fmt.Printf("omit %-24s | strata %d | observed rank %+.3f | assignments %d | exact two-sided p=%.4f\n",
			strata[omit].name, used, observed, visited, float64(geTwo)/float64(visited))
	}
}
