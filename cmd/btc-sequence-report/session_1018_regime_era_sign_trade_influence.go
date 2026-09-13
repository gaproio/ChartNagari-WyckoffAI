package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// init appends one bounded descriptive robustness check for the existing
// regime+era matched 10-18 UTC sign contrast. It removes each common-support
// trade once, keeps the original five-stratum estimand only when both session
// sides remain represented in every stratum, and reports how often one deletion
// destroys common support plus the range of the surviving sign contrasts. This
// changes no detector, threshold, session, execution, cost, or trading rule.
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
	type obs struct {
		stamp    string
		stratum  string
		positive bool
		inside   bool
	}
	var all []obs
	byStratum := map[string][]int{}

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
		idx := len(all)
		all = append(all, obs{
			stamp:    m[1],
			stratum:  key,
			positive: netR > 0,
			inside:   ts.Hour() >= 10 && ts.Hour() < 18,
		})
		byStratum[key] = append(byStratum[key], idx)
	}

	keys := make([]string, 0, len(byStratum))
	for key, idxs := range byStratum {
		inN := 0
		for _, idx := range idxs {
			if all[idx].inside {
				inN++
			}
		}
		if inN > 0 && inN < len(idxs) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 regime+era sign single-trade influence stress (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Removes each trade from the existing common-support regime+era sign comparison once. The original stratum set is retained only when both 10-18 and outside-session observations remain in every stratum; otherwise the deletion is flagged as support-breaking. No filter or trading rule changes.")
	if len(keys) == 0 {
		fmt.Println("sign single-trade influence stress unavailable: no common-support strata")
		return
	}

	keySet := map[string]bool{}
	for _, key := range keys {
		keySet[key] = true
	}

	valid := 0
	broken := 0
	minContrast := 0.0
	maxContrast := 0.0
	firstValid := true
	for omit, candidate := range all {
		if !keySet[candidate.stratum] {
			continue
		}

		sumDiff := 0.0
		supportOK := true
		for _, key := range keys {
			inN, outN := 0, 0
			inPos, outPos := 0, 0
			for _, idx := range byStratum[key] {
				if idx == omit {
					continue
				}
				tr := all[idx]
				if tr.inside {
					inN++
					if tr.positive {
						inPos++
					}
				} else {
					outN++
					if tr.positive {
						outPos++
					}
				}
			}
			if inN == 0 || outN == 0 {
				supportOK = false
				break
			}
			sumDiff += float64(inPos)/float64(inN) - float64(outPos)/float64(outN)
		}

		where := "outside"
		if candidate.inside {
			where = "10-18"
		}
		sign := "non-positive"
		if candidate.positive {
			sign = "positive"
		}
		if !supportOK {
			broken++
			fmt.Printf("omit %s %-24s %-7s %-12s | common support LOST\n", candidate.stamp, candidate.stratum, where, sign)
			continue
		}

		contrast := sumDiff / float64(len(keys))
		valid++
		if firstValid || contrast < minContrast {
			minContrast = contrast
		}
		if firstValid || contrast > maxContrast {
			maxContrast = contrast
		}
		firstValid = false
		fmt.Printf("omit %s %-24s %-7s %-12s | equal-weight sign contrast %+.3f\n", candidate.stamp, candidate.stratum, where, sign, contrast)
	}

	fmt.Printf("single-trade deletions: %d preserve all %d common-support strata, %d break support\n", valid, len(keys), broken)
	if valid > 0 {
		fmt.Printf("support-preserving leave-one-trade-out sign-contrast range: %+.3f to %+.3f\n", minContrast, maxContrast)
	}
}
