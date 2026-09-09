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

func logChooseSessionBreak(n, k int) float64 {
	if k < 0 || k > n {
		return math.Inf(-1)
	}
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

func hypergeomSessionBreak(x, row1, col1, total int) float64 {
	return math.Exp(logChooseSessionBreak(col1, x) + logChooseSessionBreak(total-col1, row1-x) - logChooseSessionBreak(total, row1))
}

// fisherTwoSidedSessionBreak returns the ordinary two-sided Fisher exact p-value
// by summing all fixed-margin tables no more probable than the observed table.
func fisherTwoSidedSessionBreak(a, b, c, d int) float64 {
	row1 := a + b
	col1 := a + c
	total := row1 + c + d
	lo := 0
	if row1-(total-col1) > lo {
		lo = row1 - (total - col1)
	}
	hi := row1
	if col1 < hi {
		hi = col1
	}
	observed := hypergeomSessionBreak(a, row1, col1, total)
	p := 0.0
	for x := lo; x <= hi; x++ {
		px := hypergeomSessionBreak(x, row1, col1, total)
		if px <= observed+1e-12 {
			p += px
		}
	}
	if p > 1 {
		return 1
	}
	return p
}

// init adds one bounded exact-test diagnostic for the already-observed 10-18 UTC
// incidence break. It uses the same fixed 2023 boundary and exact frozen trades as
// the uncertainty diagnostic. This quantifies evidence only; it creates no filter
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

	p := fisherTwoSidedSessionBreak(preInside, preOutside, postInside, postOutside)
	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 fixed-boundary Fisher exact diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Same exact frozen trades and predeclared 2023 boundary as the incidence-break uncertainty check. Two-sided Fisher exact tests whether session incidence differs across the boundary without relying on asymptotic approximations; no session/date filter is created.")
	fmt.Printf("through 2022: 10-18 %d | outside %d ; 2023+: 10-18 %d | outside %d | Fisher two-sided p=%.4f\n",
		preInside, preOutside, postInside, postOutside, p)
}
