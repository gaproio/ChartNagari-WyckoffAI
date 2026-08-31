package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/Ju571nK/Chatter/internal/wyckoff"
)

var tradeR = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} .*\|.* gross ([+-]?[0-9]+(?:\.[0-9]+)?)R net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)

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
	s := bufio.NewScanner(f)
	for s.Scan() {
		m := tradeR.FindStringSubmatch(s.Text())
		if len(m) != 3 { continue }
		grossR, err1 := strconv.ParseFloat(m[1], 64)
		netR, err2 := strconv.ParseFloat(m[2], 64)
		if err1 != nil || err2 != nil { continue }
		report.Trades = append(report.Trades, wyckoff.BTCMasterTrade{GrossR: grossR, NetR: netR})
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
}
