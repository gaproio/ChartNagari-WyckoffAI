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

var tradeNetR = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} .*\|.* net ([+-]?[0-9]+(?:\.[0-9]+)?)R`)

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
		m := tradeNetR.FindStringSubmatch(s.Text())
		if len(m) != 2 { continue }
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil { continue }
		report.Trades = append(report.Trades, wyckoff.BTCMasterTrade{NetR: v})
	}
	if err := s.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read report:", err)
		os.Exit(1)
	}

	r := wyckoff.ValidateBTCSequenceDiagnostic(report)
	fmt.Println()
	fmt.Println("BTC 15M sequence/tail-dependence diagnostic (DESCRIPTIVE; frozen rules unchanged):")
	fmt.Println("Chronological NetR path of the exact frozen trades; measures concentration and drawdown, not a new filter.")
	fmt.Printf("trades %d | total net %+.3fR | median %+.3fR | PF %.2f | max DD %.3fR | max consecutive losses %d\n",
		r.Trades, r.TotalNetR, r.MedianNetR, r.ProfitFactor, r.MaxDrawdownR, r.MaxConsecutiveLosses)
	fmt.Printf("positive/negative/flat %d/%d/%d | largest winner %+.3fR = %.1f%% of total net | top 3 winners %+.3fR = %.1f%% of total net\n",
		r.PositiveTrades, r.NegativeTrades, r.FlatTrades, r.LargestWinnerR, r.LargestWinnerSharePct, r.Top3WinnersR, r.Top3WinnersSharePct)
}
