# BTCUSDT / 15M autonomous research loop policy

Scope: BTCUSDT, 15-minute candles, Wyckoff accumulation research only.

## Frozen baseline

- V3 structure detector remains unchanged unless a separate robustness study justifies a redesign.
- Entry confirmation: midpoint + prospective higher low within 8 candles.
- Execution: next 15-minute candle open.
- Stop: post-Test structural stop.
- Target benchmark: 3R.
- Maximum hold: 64 x 15-minute candles (16 hours).
- Cost assumption: 10 bps fee per side + 5 bps slippage per side unless explicitly changed for sensitivity research.

## Autonomous-loop rules

1. Work only on branch `wyckoff-v2`; never modify `main`.
2. Run at most ONE new diagnostic/research experiment per assistant cycle.
3. Prefer descriptive diagnostics, walk-forward checks, perturbation tests and bounded comparisons over parameter optimization.
4. Do not search dense parameter grids or repeatedly tune thresholds until backtests improve.
5. Do not promote a research variant to the frozen baseline solely because it wins on the same historical sample that suggested it.
6. Keep causal next-open execution and conservative same-candle stop priority.
7. Do not enable live trading or automatic order execution. Paper alerts may be considered only after robustness work.
8. If tests fail, data is incomplete, repository state is ambiguous, or evidence conflicts, stop the loop and request human review rather than forcing a result.
9. The Mac poller may run every 20 minutes, but it should execute the expensive study only when new research code appears on `wyckoff-v2`.
10. The assistant should consume a newly pushed `research/btc15m/latest.txt` report before proposing the next experiment.

## Alternating handshake

- Assistant pushes exactly one new research-code commit.
- Mac poller detects the new commit, pulls, runs tests and the master report, then pushes `research: refresh BTCUSDT 15M master report`.
- Assistant consumes that report and may push the next single research-code commit.

This handshake prevents both sides from repeatedly acting on stale results.
