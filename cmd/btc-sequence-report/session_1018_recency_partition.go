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
}
