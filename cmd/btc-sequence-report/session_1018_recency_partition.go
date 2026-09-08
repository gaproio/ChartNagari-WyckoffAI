package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type recencyPartitionCohort struct {
	name   string
	trades []time.Time
}

// init adds one bounded recency-partition diagnostic around the fixed 10-18 UTC
// cohort. It compares recency inside that already-studied block with all frozen
// trades outside it. This is descriptive only: it does not change the detector,
// confirmation, execution, stop, target, hold, costs, or create a session/date
// filter. The purpose is only to distinguish a 10-18-specific observation drought
// from a broader frozen-signal drought.
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
	endLine := regexp.MustCompile(`^# end: (\d{4}-\d{2}-\d{2})$`)
	inside := recencyPartitionCohort{name: "10-18 UTC"}
	outside := recencyPartitionCohort{name: "outside 10-18 UTC"}
	var reportEnd time.Time

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if m := endLine.FindStringSubmatch(line); len(m) == 2 {
			if t, err := time.ParseInLocation("2006-01-02", m[1], time.UTC); err == nil {
				reportEnd = t
			}
		}
		m := tradeLine.FindStringSubmatch(line)
		if len(m) != 2 {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
		if err != nil {
			continue
		}
		if ts.Hour() >= 10 && ts.Hour() < 18 {
			inside.trades = append(inside.trades, ts)
		} else {
			outside.trades = append(outside.trades, ts)
		}
	}
	if reportEnd.IsZero() || len(inside.trades) == 0 || len(outside.trades) == 0 {
		return
	}

	cohorts := []*recencyPartitionCohort{&inside, &outside}
	for _, c := range cohorts {
		sort.Slice(c.trades, func(i, j int) bool { return c.trades[i].Before(c.trades[j]) })
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 recency partition diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Exact frozen trades partitioned once into the already-studied 10-18 UTC block versus all other UTC entry times. This tests whether the 10-18 observation drought is session-specific or system-wide; it does not create or optimize a session/date filter.")
	for _, c := range cohorts {
		latest := c.trades[len(c.trades)-1]
		ageDays := reportEnd.Sub(latest).Hours() / 24
		if ageDays < 0 {
			continue
		}
		counts := make([]int, 3)
		for i, years := range []int{1, 2, 4} {
			cutoff := reportEnd.AddDate(-years, 0, 0)
			for _, ts := range c.trades {
				if !ts.Before(cutoff) && !ts.After(reportEnd) {
					counts[i]++
				}
			}
		}
		fmt.Printf("%-18s | n=%d | first %s | latest %s | age %.0f days | trailing 1Y/2Y/4Y %d/%d/%d\n",
			c.name, len(c.trades), c.trades[0].Format("2006-01-02"), latest.Format("2006-01-02"), ageDays, counts[0], counts[1], counts[2])
	}

	// Fixed-era incidence is a bounded follow-up to the recency partition above.
	// It asks whether the 10-18 disappearance is abrupt/recent or part of a longer
	// temporal shift. Era boundaries are predeclared and already used elsewhere in
	// the master report; no session boundary, date cutoff, or trading rule is tuned.
	type era struct {
		name       string
		startYear  int
		endYear    int
	}
	eras := []era{
		{name: "2017-2019", startYear: 2017, endYear: 2019},
		{name: "2020-2022", startYear: 2020, endYear: 2022},
		{name: "2023-2025", startYear: 2023, endYear: 2025},
		{name: "2026 PARTIAL", startYear: 2026, endYear: 2026},
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 frozen-trade incidence by fixed era (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Exact frozen trades only, using the master report's predeclared eras. Counts and 10-18 share diagnose temporal/session concentration; they do not create a session or era filter.")
	for _, e := range eras {
		insideN, outsideN := 0, 0
		for _, ts := range inside.trades {
			if ts.Year() >= e.startYear && ts.Year() <= e.endYear {
				insideN++
			}
		}
		for _, ts := range outside.trades {
			if ts.Year() >= e.startYear && ts.Year() <= e.endYear {
				outsideN++
			}
		}
		total := insideN + outsideN
		if total == 0 {
			fmt.Printf("%-12s | all n=0\n", e.name)
			continue
		}
		fmt.Printf("%-12s | 10-18 n=%d | outside n=%d | all n=%d | 10-18 share %.1f%%\n",
			e.name, insideN, outsideN, total, 100*float64(insideN)/float64(total))
	}
}
