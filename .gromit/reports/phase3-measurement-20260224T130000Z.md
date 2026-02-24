# Phase-3 Measurement Report

## Median Comparison

| Metric | Baseline | Optimized |
| --- | ---: | ---: |
| median_input_tokens | 2400 | 1725 |
| median_cost_usd | 1.58 | 1.12 |
| median_success_rate | 0.80 | 1.00 |

## Cache Hit Rates By Prompt Class

| Prompt Class | Hit Rate |
| --- | ---: |
| render_static_build | 0.67 |
| utility_summarization | 1.00 |

## Kill-Switch Rollback Assessment

- kill_switch_recommended: false
- triggers: none
