| mode | run kind | accepted/measured | median ms | p90 ms | min–max ms | median CPU ms | median max RSS MB | daemon CPU ms | daemon lifetime-peak RSS MB | oracle | verified | findings |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| kaa-oracle | cold | 5/5 | 149361 | 154700 | 144405–154700 | 535859 | 3825 | n/a | n/a | ran | n/a | 3845 |
| kaa-oracle | warm-disk | 5/5 | 4648 | 5414 | 4582–5414 | 7639 | 1037 | n/a | n/a | skipped: findings-bundle-hit (5) | 5/5 | 3845 |
| kaa-oracle | warm-daemon | 0/5 | 0 | 0 | 0–0 | 0 | 0 | n/a | n/a | - | 0/5 |  |
| kaa-oracle | body-edit | 0/5 | 0 | 0 | 0–0 | 0 | 0 | n/a | n/a | - | 0/5 |  |
| kaa-oracle | abi-edit | 0/5 | 0 | 0 | 0–0 | 0 | 0 | n/a | n/a | - | 0/5 |  |

Rejected runs (excluded from timings):
- kaa-oracle/warm-daemon run 1: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/warm-daemon run 2: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/warm-daemon run 3: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/warm-daemon run 4: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/warm-daemon run 5: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/body-edit run 1: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/body-edit run 2: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/body-edit run 3: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/body-edit run 4: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/body-edit run 5: findings differ from a clean scan (3846 vs 3845 findings)
- kaa-oracle/abi-edit run 1: findings differ from a clean scan (3847 vs 3846 findings)
- kaa-oracle/abi-edit run 2: findings differ from a clean scan (3847 vs 3846 findings)
- kaa-oracle/abi-edit run 3: findings differ from a clean scan (3847 vs 3846 findings)
- kaa-oracle/abi-edit run 4: findings differ from a clean scan (3847 vs 3846 findings)
- kaa-oracle/abi-edit run 5: findings differ from a clean scan (3847 vs 3846 findings)

Run kinds: cold = every cache and process reset before each run; warm-disk = on-disk caches kept, fresh processes; warm-daemon = caches and krit and JVM daemons kept; body-edit and abi-edit = warm-daemon after a comment-only or public-API edit.
CPU and max RSS cover the krit process and the helpers it waited for. Daemon CPU ms and daemon lifetime-peak RSS MB cover long-lived daemons (krit serve, krit-fir, krit-types, checker) sampled from /proc before and after each run; peak RSS is the daemons' lifetime VmHWM, and n/a means no sample (non-Linux). The oracle column says whether the oracle ran or why a warm run legitimately skipped it. Raw data: results.json.
