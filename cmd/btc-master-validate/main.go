package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ju571nK/Chatter/internal/wyckoff"
	"github.com/Ju571nK/Chatter/pkg/models"
)

const binanceKlinesURL = "https://api.binance.com/api/v3/klines"

func main() {
	days := flag.Int("days",2190,"history length in days")
	endDate := flag.String("end","","history end date in UTC, YYYY-MM-DD (default: now)")
	feeBps := flag.Float64("fee-bps",10,"fee per side in basis points")
	slipBps := flag.Float64("slippage-bps",5,"slippage per side in basis points")
	flag.Parse()
	if *days < 60 { fmt.Fprintln(os.Stderr,"days must be at least 60"); os.Exit(2) }
	if *feeBps < 0 || *slipBps < 0 { fmt.Fprintln(os.Stderr,"cost assumptions cannot be negative"); os.Exit(2) }

	end := time.Now().UTC()
	if strings.TrimSpace(*endDate) != "" {
		t,err := time.Parse("2006-01-02",strings.TrimSpace(*endDate))
		if err != nil { fmt.Fprintln(os.Stderr,"invalid -end date; use YYYY-MM-DD:",err); os.Exit(2) }
		end = t.Add(24*time.Hour-time.Millisecond)
	}

	bars,err := fetch15M("BTCUSDT",*days,end)
	if err != nil { fmt.Fprintln(os.Stderr,"fetch failed:",err); os.Exit(1) }
	v3 := wyckoff.ValidateV3("BTCUSDT",bars)
	cfg := wyckoff.DefaultBTCMasterConfig()
	cfg.FeeBpsPerSide = *feeBps
	cfg.SlippageBpsPerSide = *slipBps
	report := wyckoff.ValidateBTCMaster(bars,v3,cfg)
	latency := wyckoff.ValidateBTCMasterLatency(bars,v3)
	timingVariants := wyckoff.ValidateBTCTimingVariants(bars,v3,cfg)
	holdVariants := wyckoff.ValidateBTCHoldVariants(bars,v3,cfg)
	targetVariants := wyckoff.ValidateBTCTargetVariants(bars,v3,cfg)

	fmt.Println("\nBTC MASTER REPORT — frozen B profile")
	fmt.Printf("Window end: %s | days %d | bars %d | V3 structures %d\n",end.Format("2006-01-02"),*days,len(bars),len(v3.Events))
	fmt.Println("Entry: MIDPOINT + prospective HL decision -> NEXT 15M OPEN")
	fmt.Println("Stop: lowest Test-to-decision low - 0.25 Spring ATR | Target benchmark: 3R | Max hold: 16h")
	fmt.Printf("Costs: fee %.1f bps/side + slippage %.1f bps/side | regime: 30D return >+10%% BULL, <-10%% BEAR, else SIDEWAYS\n",*feeBps,*slipBps)

	f := report.Funnel
	fmt.Println("\nSignal funnel (descriptive only; rules unchanged):")
	fmt.Printf("V3 %d -> foundation %d -> Test %d -> midpoint %d -> B decision<=8 bars %d -> next-open history %d -> valid entry/stop %d -> trades %d\n",
		f.V3Structures,f.FoundationRecovered,f.TestRecovered,f.MidpointValid,f.BDecisionFound,f.NextOpenAvailable,f.ValidEntryStop,f.Trades)
	fmt.Printf("No B decision within 8 bars: %d of %d midpoint-valid structures\n",f.MidpointValid-f.BDecisionFound,f.MidpointValid)

	fmt.Println("\nB decision selection check (descriptive; common V3 next-open anchor):")
	fmt.Println("Accepted/rejected label may use the next 8 candles; this section is NOT a tradable V3-time filter.")
	for _,b := range report.ByBDecision { printStructureCohort(b) }

	fmt.Println("\nB decision selection by year (same common V3 next-open anchor):")
	lastYear := 0
	for _,b := range report.ByBDecisionYear {
		if b.Year != lastYear {
			fmt.Printf("%d:\n",b.Year)
			lastYear = b.Year
		}
		fmt.Print("  ")
		printStructureCohort(b)
	}

	fmt.Println("\nB confirmation latency study (descriptive only; frozen rule remains <=8 bars):")
	fmt.Println("Groups show when the existing B condition first appears after the V3 signal. Outcomes use the same V3 next-open anchor.")
	for _,b := range latency { printLatencyBucket(b) }

	fmt.Println("\nBTC B timing execution comparison (CAUSAL next-open; <=16 is research only):")
	fmt.Println("Same B condition, post-Test stop, 3R target, 16h max hold and costs. The frozen <=8 rule is unchanged.")
	for _,r := range timingVariants { printTimingVariant(r) }

	fmt.Println("\nBTC 15M holding-horizon study (RESEARCH; frozen baseline remains 16H):")
	fmt.Println("Frozen <=8 B confirmation, next-open entry, post-Test stop, 3R target and same costs. Only max holding time changes.")
	for _,r := range holdVariants { printHoldVariant(r) }

	fmt.Println("\nBTC 15M target-distance study (RESEARCH; frozen baseline remains 3R):")
	fmt.Println("Frozen <=8 B confirmation, next-open entry, post-Test stop, 16H max hold and same costs. Only target distance changes.")
	for _,r := range targetVariants { printTargetVariant(r) }

	fmt.Println("\nOverall:")
	printBucket(report.Overall)
	fmt.Println("\nBy year:")
	for _,b := range report.ByYear { printBucket(b) }
	fmt.Println("\nBy 30D regime (descriptive only; NOT a filter):")
	for _,b := range report.ByRegime { printBucket(b) }

	fmt.Println("\nTrades:")
	for _,t := range report.Trades {
		fmt.Printf("%s | %-11s 30D %+.1f%% | risk %.2f%% | %-6s exit %2d bars | gross %+.3fR net %+.3fR | 16h %+.2f%% | MFE %+.2f%% MAE %+.2f%%\n",
			time.Unix(t.EntryTime,0).UTC().Format("2006-01-02 15:04"),t.Regime,t.RegimeReturn30,t.RiskPct,t.Outcome,t.ExitBars,t.GrossR,t.NetR,t.Return16H,t.MFE16H,t.MAE16H)
	}
}

func printStructureCohort(b wyckoff.BTCMasterStructureCohort) {
	if b.Structures == 0 { fmt.Printf("%-12s n=0\n",b.Name); return }
	fmt.Printf("%-12s n=%3d | 16h win %.1f%% avg %+.3f%% | MFE %+.2f%% MAE %+.2f%%\n",
		b.Name,b.Structures,b.WinRate16H,b.AvgReturn16H,b.AvgMFE16H,b.AvgMAE16H)
}

func printLatencyBucket(b wyckoff.BTCMasterLatencyBucket) {
	if b.Structures == 0 { fmt.Printf("%-12s n=0\n",b.Name); return }
	fmt.Printf("%-12s n=%3d | 16h win %.1f%% avg %+.3f%% | MFE %+.2f%% MAE %+.2f%%\n",
		b.Name,b.Structures,b.WinRate16H,b.AvgReturn16H,b.AvgMFE16H,b.AvgMAE16H)
}

func printTimingVariant(r wyckoff.BTCTimingVariantResult) {
	if r.Entries == 0 { fmt.Printf("%-18s n=0\n",r.Name); return }
	fmt.Printf("%-18s n=%3d | T/S/X %d/%d/%d | net-win %.1f%% | delay %.2f | risk %.2f%% | gross %+.3fR net %+.3fR\n",
		r.Name,r.Entries,r.TargetHits,r.StopHits,r.TimeExits,r.NetWinRate,r.AvgDelayBars,r.AvgRiskPct,r.AvgGrossR,r.AvgNetR)
}

func printHoldVariant(r wyckoff.BTCHoldVariantResult) {
	if r.Entries == 0 { fmt.Printf("%-24s n=0\n",r.Name); return }
	fmt.Printf("%-24s n=%3d | T/S/X %d/%d/%d | net-win %.1f%% | gross %+.3fR net %+.3fR\n",
		r.Name,r.Entries,r.TargetHits,r.StopHits,r.TimeExits,r.NetWinRate,r.AvgGrossR,r.AvgNetR)
}

func printTargetVariant(r wyckoff.BTCTargetVariantResult) {
	if r.Entries == 0 { fmt.Printf("%-12s n=0\n",r.Name); return }
	fmt.Printf("%-12s n=%3d | T/S/X %d/%d/%d | net-win %.1f%% | gross %+.3fR net %+.3fR\n",
		r.Name,r.Entries,r.TargetHits,r.StopHits,r.TimeExits,r.NetWinRate,r.AvgGrossR,r.AvgNetR)
}

func printBucket(b wyckoff.BTCMasterBucket) {
	if b.Trades == 0 { fmt.Printf("%-12s n=0\n",b.Name); return }
	fmt.Printf("%-12s n=%3d | T/S/X %d/%d/%d | net-win %.1f%% | gross %+.3fR net %+.3fR | 16h %+.3f%% | MFE %+.2f%% MAE %+.2f%% | risk %.2f%% | exit %.1f bars\n",
		b.Name,b.Trades,b.TargetHits,b.StopHits,b.TimeExits,b.NetWinRate,b.AvgGrossR,b.AvgNetR,b.AvgReturn16H,b.AvgMFE16H,b.AvgMAE16H,b.AvgRiskPct,b.AvgExitBars)
}

func fetch15M(symbol string, days int, end time.Time) ([]models.OHLCV,error) {
	client:=&http.Client{Timeout:20*time.Second}
	end=end.UTC(); start:=end.Add(-time.Duration(days)*24*time.Hour)
	startMS,endMS:=start.UnixMilli(),end.UnixMilli()
	bars:=make([]models.OHLCV,0,days*96)
	for startMS<endMS {
		url:=fmt.Sprintf("%s?symbol=%s&interval=15m&limit=1000&startTime=%d&endTime=%d",binanceKlinesURL,symbol,startMS,endMS)
		resp,err:=client.Get(url); if err!=nil { return nil,err }
		body,err:=io.ReadAll(resp.Body); resp.Body.Close(); if err!=nil { return nil,err }
		if resp.StatusCode!=http.StatusOK { return nil,fmt.Errorf("Binance HTTP %d: %s",resp.StatusCode,strings.TrimSpace(string(body))) }
		var raw [][]json.RawMessage; if err:=json.Unmarshal(body,&raw); err!=nil { return nil,err }
		if len(raw)==0 { break }
		lastOpen:=int64(0)
		for _,row:=range raw {
			if len(row)<6 { continue }
			var openMS int64; if err:=json.Unmarshal(row[0],&openMS); err!=nil { continue }
			bar:=models.OHLCV{Symbol:symbol,Timeframe:"15M",OpenTime:time.UnixMilli(openMS).UTC(),Open:rawFloat(row[1]),High:rawFloat(row[2]),Low:rawFloat(row[3]),Close:rawFloat(row[4]),Volume:rawFloat(row[5])}
			if bar.Close>0 { bars=append(bars,bar) }
			if openMS>lastOpen { lastOpen=openMS }
		}
		if lastOpen==0 { break }
		next:=lastOpen+int64(15*time.Minute/time.Millisecond)
		if next<=startMS { break }
		startMS=next
		if len(raw)<1000 { break }
		time.Sleep(120*time.Millisecond)
	}
	return bars,nil
}

func rawFloat(raw json.RawMessage) float64 {
	var s string
	if err:=json.Unmarshal(raw,&s); err==nil { v,_:=strconv.ParseFloat(s,64); return v }
	var v float64
	_ = json.Unmarshal(raw,&v)
	return v
}
