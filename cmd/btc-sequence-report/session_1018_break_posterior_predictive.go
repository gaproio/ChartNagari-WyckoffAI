package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// init adds one bounded posterior-predictive diagnostic for the already-observed
// 10-18 UTC incidence drought. It learns the historical session-incidence rate
// only from through-2022 frozen trades under a symmetric Beta(1,1) reference prior,
// then asks how probable the observed post-2022 zero-count is after integrating
// historical rate uncertainty. This is descriptive only: it creates no filter and
// changes no detector, confirmation, execution, stop, target, hold, or costs.
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

	postN := postInside + postOutside
	if preInside+preOutside == 0 || postN == 0 {
		return
	}

	// Historical posterior under Beta(1,1): alpha = inside+1, beta = outside+1.
	// Beta-binomial posterior predictive probability of zero inside-session trades
	// in postN future observations is B(alpha,beta+postN)/B(alpha,beta), evaluated
	// as a stable finite product because postN is small and fixed by observed data.
	alpha := float64(preInside + 1)
	beta := float64(preOutside + 1)
	pZero := 1.0
	for i := 0; i < postN; i++ {
		pZero *= (beta + float64(i)) / (alpha + beta + float64(i))
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 historical-posterior predictive drought diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Uses only through-2022 frozen trades to form a Beta(1,1)-updated historical session-incidence posterior, then integrates that uncertainty when predicting the observed 2023+ sample size. No session/date filter is created.")
	fmt.Printf("through 2022: 10-18 %d | outside %d | posterior Beta(%d,%d) ; 2023+: 10-18 %d/%d | posterior-predictive P(0 of %d)=%.1f%%\n",
		preInside, preOutside, preInside+1, preOutside+1, postInside, postN, postN, 100*pZero)
}
