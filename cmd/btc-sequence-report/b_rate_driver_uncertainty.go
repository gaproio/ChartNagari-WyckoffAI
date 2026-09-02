package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var bDriverEraR = regexp.MustCompile(`^(2017-2019|2020-2022|2023-2025|2026 PARTIAL)\s+n=\s*(\d+)\s+\| midpoint\s+(\d+)\s+\([0-9]+(?:\.[0-9]+)?%\)\s+\| prospective-HL\s+(\d+)\s+\([0-9]+(?:\.[0-9]+)?%\)\s+\| BOTH/B\s+(\d+)\s+\([0-9]+(?:\.[0-9]+)?%\)`)

type bDriverObservation struct {
	Name     string
	N        int
	Midpoint int
	Both     int
}

// init appends one bounded uncertainty check for the two components in the
// existing B-rate decomposition. It reads only fixed-era incidence counts from
// the already-generated report and changes no detector or execution rule.
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

	obs := map[string]bDriverObservation{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		m := bDriverEraR.FindStringSubmatch(s.Text())
		if len(m) != 6 {
			continue
		}
		n, errN := strconv.Atoi(m[2])
		mid, errMid := strconv.Atoi(m[3])
		both, errBoth := strconv.Atoi(m[5])
		if errN != nil || errMid != nil || errBoth != nil || n <= 0 || mid < 0 || mid > n || both < 0 || both > mid {
			continue
		}
		obs[m[1]] = bDriverObservation{Name: m[1], N: n, Midpoint: mid, Both: both}
	}

	a, okA := obs["2017-2019"]
	b, okB := obs["2020-2022"]
	if !okA || !okB || a.Midpoint == 0 || b.Midpoint == 0 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M B-rate driver uncertainty diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Wilson 95% intervals for the two factors in the fixed-era B-rate decomposition: midpoint incidence and BOTH conditional on midpoint. No filter or significance gate.")
	printBDriverInterval(a)
	printBDriverInterval(b)

	aMidLo, aMidHi := wilson95Driver(a.Midpoint, a.N)
	bMidLo, bMidHi := wilson95Driver(b.Midpoint, b.N)
	aConvLo, aConvHi := wilson95Driver(a.Both, a.Midpoint)
	bConvLo, bConvHi := wilson95Driver(b.Both, b.Midpoint)
	midDiff := float64(b.Midpoint)/float64(b.N) - float64(a.Midpoint)/float64(a.N)
	convDiff := float64(b.Both)/float64(b.Midpoint) - float64(a.Both)/float64(a.Midpoint)
	fmt.Printf("2020-2022 minus 2017-2019 midpoint incidence: %+.1f pp | conservative Wilson-bound interval %+.1f to %+.1f pp\n",
		midDiff*100, (bMidLo-aMidHi)*100, (bMidHi-aMidLo)*100)
	fmt.Printf("2020-2022 minus 2017-2019 BOTH|midpoint conversion: %+.1f pp | conservative Wilson-bound interval %+.1f to %+.1f pp\n",
		convDiff*100, (bConvLo-aConvHi)*100, (bConvHi-aConvLo)*100)
}

func printBDriverInterval(o bDriverObservation) {
	midLo, midHi := wilson95Driver(o.Midpoint, o.N)
	convLo, convHi := wilson95Driver(o.Both, o.Midpoint)
	midP := float64(o.Midpoint) / float64(o.N) * 100
	convP := float64(o.Both) / float64(o.Midpoint) * 100
	fmt.Printf("%-12s midpoint %2d/%2d = %5.1f%% | Wilson %.1f%% to %.1f%% ; BOTH|midpoint %2d/%2d = %5.1f%% | Wilson %.1f%% to %.1f%%\n",
		o.Name, o.Midpoint, o.N, midP, midLo*100, midHi*100, o.Both, o.Midpoint, convP, convLo*100, convHi*100)
}

func wilson95Driver(successes, n int) (float64, float64) {
	if n <= 0 {
		return 0, 0
	}
	z := 1.959963984540054
	z2 := z * z
	nf := float64(n)
	p := float64(successes) / nf
	denom := 1 + z2/nf
	center := (p + z2/(2*nf)) / denom
	half := z * math.Sqrt((p*(1-p)+z2/(4*nf))/nf) / denom
	lo := center - half
	hi := center + half
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}
