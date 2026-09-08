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

// init appends one bounded recency-coverage diagnostic for the fixed 10-18 UTC
// frozen-trade cohort. It measures how recently the historical cohort has
// produced observations relative to the report end date. This is descriptive
// only: detector, confirmation, entry, stop, target, hold, costs and session
// definition remain unchanged, and no date filter is created or optimized.
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
	var trades []time.Time
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
		if err != nil || ts.Hour() < 10 || ts.Hour() >= 18 {
			continue
		}
		trades = append(trades, ts)
	}
	if len(trades) == 0 || reportEnd.IsZero() {
		return
	}

	sort.Slice(trades, func(i, j int) bool { return trades[i].Before(trades[j]) })
	latest := trades[len(trades)-1]
	ageDays := reportEnd.Sub(latest).Hours() / 24
	if ageDays < 0 {
		return
	}

	fmt.Println()
	fmt.Println("BTC 15M UTC 10-18 recency-coverage diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Exact fixed 10-18 UTC trades measured against the report end date using predeclared trailing 1Y/2Y/4Y windows. This tests historical recency only; it does not create or optimize a date filter.")
	fmt.Printf("cohort n=%d | first %s | latest %s | report end %s | latest-observation age %.0f days (%.2f years)\n",
		len(trades), trades[0].Format("2006-01-02"), latest.Format("2006-01-02"), reportEnd.Format("2006-01-02"), ageDays, ageDays/365.25)

	for _, years := range []int{1, 2, 4} {
		cutoff := reportEnd.AddDate(-years, 0, 0)
		count := 0
		for _, ts := range trades {
			if !ts.Before(cutoff) && !ts.After(reportEnd) {
				count++
			}
		}
		fmt.Printf("trailing %dY since %s | %d/%d cohort trades\n", years, cutoff.Format("2006-01-02"), count, len(trades))
	}
}
