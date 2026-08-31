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
	symbol := flag.String("symbol","BTCUSDT","Binance symbol")
	days := flag.Int("days",90,"history length in days")
	endDate := flag.String("end","","history end date in UTC, YYYY-MM-DD (default: now)")
	flag.Parse()
	if *days < 7 { fmt.Fprintln(os.Stderr,"days must be at least 7"); os.Exit(2) }

	end := time.Now().UTC()
	if strings.TrimSpace(*endDate) != "" {
		t,err := time.Parse("2006-01-02",strings.TrimSpace(*endDate))
		if err != nil { fmt.Fprintln(os.Stderr,"invalid -end date; use YYYY-MM-DD:",err); os.Exit(2) }
		end = t.Add(24*time.Hour-time.Millisecond)
	}

	bars,err := fetch15M(strings.ToUpper(*symbol),*days,end)
	if err != nil { fmt.Fprintln(os.Stderr,"fetch failed:",err); os.Exit(1) }

	s := wyckoff.ValidateV3(strings.ToUpper(*symbol),bars)
	fmt.Printf("\nWyckoff V3 validation — %s 15M\n",s.Symbol)
	fmt.Printf("Window end: %s | Bars: %d | Unique qualifying ranges: %d | Study triggers: %d\n",end.UTC().Format("2006-01-02"),s.Bars,s.UniqueRanges,s.Overall.Triggers)
	if s.Overall.Triggers==0 { fmt.Println("No qualifying V3 Spring+Test study triggers found in this period."); return }

	fmt.Println("\nOverall:")
	printBucket(s.Overall)
	fmt.Println("\nRisk/reward simulation (Test-close entry, stop = Spring low - 0.25 ATR, max hold 16h):")
	printRisk(s.Overall)
	fmt.Println("\nBy V3 trade-score threshold:")
	for _,b:=range s.ByScore { printBucket(b); printRisk(b) }

	exec := wyckoff.ValidateV3Execution(bars,s)
	fmt.Println("\nExecution comparison — same V3 signals, detector unchanged:")
	fmt.Println("Overall:")
	for _,x:=range exec.Overall { printExecution(x) }
	fmt.Println("\nTradeScore >= 0.65:")
	for _,x:=range exec.Score65 { printExecution(x) }

	conf := wyckoff.ValidateV3Confirmation(bars,s)
	fmt.Println("\nConfirmation research — measured only, NOT used as a signal filter:")
	fmt.Println("Overall:")
	printConfirmation(conf.Overall)
	fmt.Println("\nBy number of confirmation features present within next 8 candles:")
	for _,b:=range conf.ByFeature { printConfirmation(b) }
	fmt.Println("\nIndividual confirmation features:")
	for _,b:=range conf.Features { printConfirmation(b) }

	v4 := wyckoff.ValidateV4Candidates(bars,s)
	fmt.Println("\nV4 candidate rules — research comparison only, V3 detector unchanged:")
	for _,r:=range v4.Candidates { printV4Candidate(r) }

	causal := wyckoff.ValidateV4Causal(bars,s)
	fmt.Println("\nV4 causal study — MIDPOINT RECLAIM + confirmed HIGHER LOW:")
	printV4Causal(causal)

	variants := wyckoff.ValidateV4EntryVariants(bars,s)
	fmt.Println("\nV4 causal entry/stop variants — detector unchanged:")
	fmt.Println("SPRING stop = Spring low - 0.75 ATR | POSTTEST stop = lowest Test-to-entry low - 0.25 ATR")
	for _,r:=range variants.Variants { printV4Variant(r) }

	frozen := wyckoff.ValidateV4FrozenFinalists(bars,s)
	fmt.Println("\nV4 frozen finalists — NEXT-BAR OPEN, POSTTEST stop:")
	fmt.Println("Decision is made on candle close; execution is the following 15M candle open.")
	for _,r:=range frozen.Variants { printV4Variant(r) }

	fmt.Println("\nRecent V3 triggers:")
	start:=0; if len(s.Events)>12 { start=len(s.Events)-12 }
	for _,e:=range s.Events[start:] {
		fmt.Printf("%s | score %.3f | risk %.2f%% | R1 %+.2fR R2 %+.2fR R3 %+.2fR | 4h %+.2f%% | 8h %+.2f%% | 16h %+.2f%%\n",
			time.Unix(e.Time,0).UTC().Format("2006-01-02 15:04 UTC"),e.TradeScore,e.RiskPct,e.R1,e.R2,e.R3,e.Return4H,e.Return8H,e.Return16H)
	}
}

func printBucket(b wyckoff.V3ValidationBucket) {
	if b.Triggers==0 { fmt.Printf("%-12s n=%3d\n",b.Name,b.Triggers); return }
	fmt.Printf("%-12s n=%3d | score %.3f | 4h %.1f%% %+.3f%% | 8h %.1f%% %+.3f%% | 16h %.1f%% %+.3f%% | MFE %+.3f%% MAE %+.3f%%\n",
		b.Name,b.Triggers,b.AvgScore,b.WinRate4H,b.AvgReturn4H,b.WinRate8H,b.AvgReturn8H,b.WinRate16H,b.AvgReturn16H,b.AvgMFE16H,b.AvgMAE16H)
}

func printRisk(b wyckoff.V3ValidationBucket) {
	if b.Triggers==0 { return }
	fmt.Printf("             risk %.2f%% | 1R win %.1f%% avg %+.3fR | 2R win %.1f%% avg %+.3fR | 3R win %.1f%% avg %+.3fR\n",
		b.AvgRiskPct,b.R1WinRate,b.AvgR1,b.R2WinRate,b.AvgR2,b.R3WinRate,b.AvgR3)
}

func printExecution(x wyckoff.V3ExecutionSummary) {
	if x.Trades==0 { fmt.Printf("%-20s n=%3d\n",x.Name,x.Trades); return }
	fmt.Printf("%-20s n=%3d | risk %.2f%% | 1R %.1f%% %+.3fR | 2R %.1f%% %+.3fR | 3R %.1f%% %+.3fR\n",
		x.Name,x.Trades,x.AvgRiskPct,x.R1WinRate,x.AvgR1,x.R2WinRate,x.AvgR2,x.R3WinRate,x.AvgR3)
}

func printConfirmation(b wyckoff.V3ConfirmationBucket) {
	if b.Signals==0 { fmt.Printf("%-20s n=%3d\n",b.Name,b.Signals); return }
	fmt.Printf("%-20s n=%3d | feat %.2f/6 | 16h win %.1f%% avg %+.3f%% | confirm %.1f%% | confirmed 3R avg %+.3fR\n",
		b.Name,b.Signals,b.AvgFeatures,b.WinRate16H,b.AvgReturn16H,b.ConfirmRate,b.AvgConfirmedR3)
}

func printV4Candidate(r wyckoff.V4CandidateResult) {
	if r.Signals==0 { fmt.Printf("%-24s n=%3d\n",r.Name,r.Signals); return }
	fmt.Printf("%-24s n=%3d | 16h win %.1f%% avg %+.3f%% | confirm %3d (%.1f%%) | confirmed 3R avg %+.3fR\n",
		r.Name,r.Signals,r.WinRate16H,r.AvgReturn16H,r.ConfirmedTrades,r.ConfirmRate,r.AvgConfirmedR3)
}

func printV4Causal(r wyckoff.V4CausalSummary) {
	if r.V4Entries==0 {
		fmt.Printf("V3 signals %d | V4 entries 0\n",r.V3Signals)
		return
	}
	fmt.Printf("V3 signals %d | V4 entries %d (%.1f%%) | delay %.2f bars | risk %.2f%%\n",
		r.V3Signals,r.V4Entries,r.EntryRate,r.AvgDelayBars,r.AvgRiskPct)
	fmt.Printf("16h win %.1f%% avg %+.3f%% | 1R %.1f%% %+.3fR | 2R %.1f%% %+.3fR | 3R %.1f%% %+.3fR\n",
		r.WinRate16H,r.AvgReturn16H,r.R1WinRate,r.AvgR1,r.R2WinRate,r.AvgR2,r.R3WinRate,r.AvgR3)
}

func printV4Variant(r wyckoff.V4VariantResult) {
	if r.Entries==0 {
		fmt.Printf("%-42s n=%3d | entry %.1f%%\n",r.Name,r.Entries,r.EntryRate)
		return
	}
	fmt.Printf("%-42s n=%3d | entry %.1f%% | delay %.2f | risk %.2f%% | 16h %.1f%% %+.3f%% | 1R %+.3fR | 2R %+.3fR | 3R %+.3fR\n",
		r.Name,r.Entries,r.EntryRate,r.AvgDelay,r.AvgRiskPct,r.WinRate16H,r.AvgReturn16H,r.AvgR1,r.AvgR2,r.AvgR3)
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
			if bar.Close>0 { bars=append(bars,bar) }; if openMS>lastOpen { lastOpen=openMS }
		}
		if lastOpen==0 { break }; next:=lastOpen+int64(15*time.Minute/time.Millisecond); if next<=startMS { break }; startMS=next
		if len(raw)<1000 { break }; time.Sleep(120*time.Millisecond)
	}
	return bars,nil
}

func rawFloat(raw json.RawMessage) float64 {
	var s string; if err:=json.Unmarshal(raw,&s); err==nil { v,_:=strconv.ParseFloat(s,64); return v }
	var v float64; _=json.Unmarshal(raw,&v); return v
}
