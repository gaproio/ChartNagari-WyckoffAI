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

var bIncidenceEraR = regexp.MustCompile(`^(2017-2019|2020-2022|2023-2025|2026 PARTIAL)\s+n=\s*(\d+)\s+\|.*\| BOTH/B\s+(\d+)\s+\(([0-9]+(?:\.[0-9]+)?)%\)`)

type bRateObservation struct {
	Name string
	N    int
	B    int
}

// init appends one bounded uncertainty diagnostic to btc-sequence-report output.
// It reads only the already-generated B-component incidence counts and changes
// no detector, confirmation, execution, or trading rule.
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

	order := []string{"2017-2019", "2020-2022", "2023-2025", "2026 PARTIAL"}
	obs := map[string]bRateObservation{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		m := bIncidenceEraR.FindStringSubmatch(s.Text())
		if len(m) != 5 {
			continue
		}
		n, errN := strconv.Atoi(m[2])
		b, errB := strconv.Atoi(m[3])
		if errN != nil || errB != nil || n <= 0 || b < 0 || b > n {
			continue
		}
		obs[m[1]] = bRateObservation{Name: m[1], N: n, B: b}
	}
	if len(obs) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M B-rate sampling-uncertainty diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Wilson 95% intervals for fixed-era BOTH/B incidence. This checks whether apparent era shifts are precise enough to interpret; it is not a filter or significance gate.")
	for _, name := range order {
		o, ok := obs[name]
		if !ok {
			continue
		}
		lo, hi := wilson95(o.B, o.N)
		p := float64(o.B) / float64(o.N) * 100
		fmt.Printf("%-12s %2d/%2d = %5.1f%% | Wilson 95%% %.1f%% to %.1f%%\n", name, o.B, o.N, p, lo*100, hi*100)
	}

	a, okA := obs["2017-2019"]
	b, okB := obs["2020-2022"]
	if okA && okB {
		aLo, aHi := wilson95(a.B, a.N)
		bLo, bHi := wilson95(b.B, b.N)
		diff := float64(b.B)/float64(b.N) - float64(a.B)/float64(a.N)
		// Newcombe-style interval from independent Wilson score bounds.
		diffLo := bLo - aHi
		diffHi := bHi - aLo
		fmt.Printf("2020-2022 minus 2017-2019 BOTH/B: %+.1f pp | conservative Wilson-bound interval %+.1f to %+.1f pp\n", diff*100, diffLo*100, diffHi*100)
	}
}

func wilson95(successes, n int) (float64, float64) {
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
