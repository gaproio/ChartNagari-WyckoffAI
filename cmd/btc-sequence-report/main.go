package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Ju571nK/Chatter/internal/wyckoff"
)

var tradeR = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) .*\|.* gross ([+-]?[0-9]+(?:\.[0-9]+)?)R net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)
var yearHeaderR = regexp.MustCompile(`^(\d{4}):$`)
var cohortR = regexp.MustCompile(`^\s*(B_ACCEPTED|B_REJECTED)\s+n=\s*(\d+)\s+\|\s+16h win\s+([0-9]+(?:\.[0-9]+)?)% avg\s+([+-]?[0-9]+(?:\.[0-9]+)?)%\s+\|\s+MFE\s+([+-]?[0-9]+(?:\.[0-9]+)?)% MAE\s+([+-]?[0-9]+(?:\.[0-9]+)?)%`)

type cohortAggregate struct {
	N       int
	Wins    float64
	RetSum  float64
	MFESum  float64
	MAESum  float64
}

type eraCohort struct {
	Name     string
	Accepted cohortAggregate
	Rejected cohortAggregate
}

func main() {
	path := flag.String("file", "research/btc15m/latest.txt", "BTC master text report")
	flag.Parse()

	f, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open report:", err)
		os.Exit(1)
	}
	defer f.Close()

	report := wyckoff.BTCMasterReport{}
	eras := map[string]*eraCohort{
		"2017-2019":    {Name: "2017-2019"},
		"2020-2022":    {Name: "2020-2022"},
		"2023-2025":    {Name: "2023-2025"},
		"2026 PARTIAL": {Name: "2026 PARTIAL"},
	}
	inBYearSection := false
	currentYear := 0

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()

		if m := tradeR.FindStringSubmatch(line); len(m) == 4 {
			entryTime, err0 := time.ParseInLocation("2006-01-02 15:04", m[1], time.UTC)
			grossR, err1 := strconv.ParseFloat(m[2], 64)
			netR, err2 := strconv.ParseFloat(m[3], 64)
			if err0 == nil && err1 == nil && err2 == nil {
				report.Trades = append(report.Trades, wyckoff.BTCMasterTrade{EntryTime: entryTime.Unix(), GrossR: grossR, NetR: netR})
			}
		}

		if line == "B decision selection by year (same common V3 next-open anchor):" {
			inBYearSection = true
			currentYear = 0
			continue
		}
		if inBYearSection && strings.HasPrefix(line, "B confirmation latency study") {
			inBYearSection = false
			currentYear = 0
			continue
		}
		if !inBYearSection {
			continue
		}
		if m := yearHeaderR.FindStringSubmatch(line); len(m) == 2 {
			currentYear, _ = strconv.Atoi(m[1])
			continue
		}
		if currentYear == 0 {
			continue
		}
		m := cohortR.FindStringSubmatch(line)
		if len(m) != 7 {
			continue
		}
		n, err0 := strconv.Atoi(m[2])
		winRate, err1 := strconv.ParseFloat(m[3], 64)
		avgRet, err2 := strconv.ParseFloat(m[4], 64)
		avgMFE, err3 := strconv.ParseFloat(m[5], 64)
		avgMAE, err4 := strconv.ParseFloat(m[6], 64)
		if err0 != nil || err1 != nil || err2 != nil || err3 != nil || err4 != nil || n <= 0 {
			continue
		}
		era := eraForYear(currentYear)
		if era == "" {
			continue
		}
		e := eras[era]
		var a *cohortAggregate
		if m[1] == "B_ACCEPTED" {
			a = &e.Accepted
		} else {
			a = &e.Rejected
		}
		a.N += n
		a.Wins += float64(n) * winRate / 100
		a.RetSum += float64(n) * avgRet
		a.MFESum += float64(n) * avgMFE
		a.MAESum += float64(n) * avgMAE
	}
	if err := s.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read report:", err)
		os.Exit(1)
	}

	seq := wyckoff.ValidateBTCSequenceDiagnostic(report)
	fmt.Println()
	fmt.Println("BTC 15M sequence/tail-dependence diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Chronological NetR path of the exact frozen trades; measures concentration and drawdown, not a new filter.")
	fmt.Printf("trades %d | total net %+.3fR | median %+.3fR | PF %.2f | max DD %.3fR | max consecutive losses %d\n",
		seq.Trades, seq.TotalNetR, seq.MedianNetR, seq.ProfitFactor, seq.MaxDrawdownR, seq.MaxConsecutiveLosses)
	fmt.Printf("positive/negative/flat %d/%d/%d | largest winner %+.3fR = %.1f%% of total net | top 3 winners %+.3fR = %.1f%% of total net\n",
		seq.PositiveTrades, seq.NegativeTrades, seq.FlatTrades, seq.LargestWinnerR, seq.LargestWinnerSharePct, seq.Top3WinnersR, seq.Top3WinnersSharePct)

	fmt.Println()
	fmt.Println("BTC 15M cost-sensitivity diagnostic (ROBUSTNESS; frozen rules unchanged):")
	fmt.Println("Rescales only the existing research cost assumption: 0x, 0.5x, 1x baseline, and 2x stress. This is not an exchange fee claim.")
	for _, r := range wyckoff.ValidateBTCCostSensitivity(report) {
		fmt.Printf("%-16s n=%2d | net-win %.1f%% | total %+.3fR avg %+.3fR median %+.3fR | PF %.2f\n",
			r.Name, r.Trades, r.NetWinRate, r.TotalNetR, r.AvgNetR, r.MedianNetR, r.ProfitFactor)
	}

	fmt.Println()
	fmt.Println("BTC 15M temporal robustness diagnostic (DESCRIPTIVE; no era filter):")
	fmt.Println("Fixed calendar blocks: 2017-2019, 2020-2022, 2023-2025, and 2026 partial. No period was chosen from performance.")
	for _, r := range wyckoff.ValidateBTCTemporalRobustness(report) {
		if r.Trades == 0 {
			fmt.Printf("%-12s n=0\n", r.Name)
			continue
		}
		fmt.Printf("%-12s n=%2d | net-win %.1f%% | total %+.3fR avg %+.3fR median %+.3fR | PF %.2f | max DD %.3fR | max losses %d\n",
			r.Name, r.Trades, r.NetWinRate, r.TotalNetR, r.AvgNetR, r.MedianNetR, r.ProfitFactor, r.MaxDrawdownR, r.MaxConsecutiveLosses)
	}

	fmt.Println()
	fmt.Println("BTC 15M B-selection by fixed era (DESCRIPTIVE; no era/B filter):")
	fmt.Println("Aggregates the existing common-anchor V3 cohorts. ACCEPTED/REJECTED classification can look up to 8 bars ahead and is descriptive only.")
	for _, name := range []string{"2017-2019", "2020-2022", "2023-2025", "2026 PARTIAL"} {
		e := eras[name]
		total := e.Accepted.N + e.Rejected.N
		acceptRate := 0.0
		if total > 0 {
			acceptRate = float64(e.Accepted.N) / float64(total) * 100
		}
		fmt.Printf("%-12s V3 cohorts %3d | B accepted %2d (%.1f%%) | rejected %2d\n", name, total, e.Accepted.N, acceptRate, e.Rejected.N)
		printCohortAggregate("  ACCEPTED", e.Accepted)
		printCohortAggregate("  REJECTED", e.Rejected)
	}
}

func eraForYear(year int) string {
	switch {
	case year >= 2017 && year <= 2019:
		return "2017-2019"
	case year >= 2020 && year <= 2022:
		return "2020-2022"
	case year >= 2023 && year <= 2025:
		return "2023-2025"
	case year == 2026:
		return "2026 PARTIAL"
	default:
		return ""
	}
}

func printCohortAggregate(label string, a cohortAggregate) {
	if a.N == 0 {
		fmt.Printf("%-12s n=0\n", label)
		return
	}
	n := float64(a.N)
	fmt.Printf("%-12s n=%2d | 16h win %.1f%% avg %+.3f%% | MFE %+.2f%% MAE %+.2f%%\n",
		label, a.N, a.Wins/n*100, a.RetSum/n, a.MFESum/n, a.MAESum/n)
}
