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

// wilson95SessionBreak returns a 95% Wilson score interval for a binomial proportion.
// The distinct name avoids colliding with the existing package-level wilson95 helper.
func wilson95SessionBreak(successes, total int) (float64, float64) {
	if total <= 0 {
		return 0, 0
	}
	const z = 1.96
	n := float64(total)
	p := float64(successes) / n
	z2 := z * z
	den := 1 + z2/n
	center := (p + z2/(2*n)) / den
	half := z * math.Sqrt((p*(1-p)+z2/(4*n))/n) / den
	return center - half, center + half
}

// init adds one bounded uncertainty diagnostic for the already-observed 10-18 UTC
// incidence break. The split is fixed at the report's predeclared 2023 era boundary.
// It quantifies how much uncertainty remains around the apparent post-2022 drought;
// it does not change the detector, confirmation, execution, stop, target, hold,
// costs, session boundary, or frozen baseline.
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
	preTotal, preInside := 0, 0
	postTotal, postInside := 0, 0

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
			preTotal++
			if inside {
				preInside++
			}
		} else {
			postTotal++
			if inside {
				postInside++
			}
		}
	}
	if preTotal == 0 || postTotal == 0 {
		return
	}

	preLo, preHi := wilson95SessionBreak(preInside, preTotal)
	postLo, postHi := wilson95SessionBreak(postInside, postTotal)
	preShare := float64(preInside) / float64(preTotal)
	postShare := float64(postInside) / float64(postTotal)
	zeroRecentProb := math.Pow(1-preShare, float64(postTotal))

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 incidence-break uncertainty (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Exact frozen trades split once at the predeclared 2023 era boundary. Wilson 95% intervals and the historical-share zero-count probability quantify uncertainty around the apparent post-2022 session drought; no session/date filter is created.")
	fmt.Printf("through 2022 | 10-18 %d/%d = %.1f%% | Wilson95 %.1f%%..%.1f%%\n",
		preInside, preTotal, 100*preShare, 100*preLo, 100*preHi)
	fmt.Printf("2023+        | 10-18 %d/%d = %.1f%% | Wilson95 %.1f%%..%.1f%%\n",
		postInside, postTotal, 100*postShare, 100*postLo, 100*postHi)
	fmt.Printf("If the through-2022 10-18 share %.1f%% persisted, P(0 of %d recent frozen trades in 10-18) = %.1f%%\n",
		100*preShare, postTotal, 100*zeroRecentProb)
}
