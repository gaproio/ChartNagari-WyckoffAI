package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"time"
)

func logBetaSessionBreak(a, b float64) float64 {
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lab, _ := math.Lgamma(a + b)
	return la + lb - lab
}

// init adds one bounded model-evidence diagnostic for the already-observed
// 10-18 UTC incidence break. It compares a single stable session-incidence rate
// with separate pre/post-2023 rates under the same symmetric Beta(1,1) reference
// prior. This is descriptive prior-sensitive evidence only: it creates no filter
// and changes no detector, confirmation, execution, stop, target, hold, or costs.
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

	tradeLine := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) `)
	preInside, preOutside := 0, 0
	postInside, postOutside := 0, 0

	s := bufio.NewScanner(f)
	for s.Scan() {
		m := tradeLine.FindStringSubmatch(s.Text())
		if len(m) != 2 {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
		if err != nil {
			continue
		}
		inside := ts.Hour() >= 10 && ts.Hour() < 18
		if ts.Year() <= 2022 {
			if inside {
				preInside++
			} else {
				preOutside++
			}
		} else if inside {
			postInside++
		} else {
			postOutside++
		}
	}
	if preInside+preOutside == 0 || postInside+postOutside == 0 {
		return
	}

	// Beta-binomial marginal likelihoods with Beta(1,1) reference priors.
	// Stable model: one incidence rate across both eras.
	allInside := preInside + postInside
	allOutside := preOutside + postOutside
	logStable := logBetaSessionBreak(float64(allInside+1), float64(allOutside+1))

	// Break model: independent incidence rates before and after the fixed boundary.
	logBreak := logBetaSessionBreak(float64(preInside+1), float64(preOutside+1)) +
		logBetaSessionBreak(float64(postInside+1), float64(postOutside+1))
	bfBreakVsStable := math.Exp(logBreak - logStable)

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 fixed-boundary Bayes-factor diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Same exact frozen trades and predeclared 2023 boundary. A Beta-binomial comparison uses symmetric Beta(1,1) reference priors to compare separate pre/post session-incidence rates against one stable rate. This is prior-sensitive evidence only; no session/date filter is created.")
	fmt.Printf("through 2022: 10-18 %d | outside %d ; 2023+: 10-18 %d | outside %d | BF(break/stable)=%.3f\n",
		preInside, preOutside, postInside, postOutside, bfBreakVsStable)
}
