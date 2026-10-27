| mode | run kind | accepted/measured | median ms | p90 ms | min–max ms | median CPU ms | median max RSS MB | daemon CPU ms | daemon lifetime-peak RSS MB | oracle | verified | findings |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| fir-oracle | cold | 3/3 | 131673 | 132300 | 130380–132300 | 378572 | 2931 | n/a | n/a | ran | n/a | 4047 |
| fir-oracle | warm-disk | 3/3 | 7477 | 7694 | 6755–7694 | 12485 | 1447 | n/a | n/a | skipped: findings-bundle-hit (3) | 3/3 | 4047 |
| fir-oracle | warm-daemon | 3/3 | 3139 | 3239 | 2929–3239 | 85 | 27 | 4270 | 5402 | skipped: findings-bundle-hit (3) | 3/3 | 4047 |
| fir-oracle | body-edit | 3/3 | 59862 | 74451 | 56326–74451 | 91 | 27 | 110780 | 6179 | ran | 3/3 | 4047 |
| fir-oracle | abi-edit | 3/3 | 70178 | 71290 | 69150–71290 | 80 | 27 | 152790 | 6134 | ran | 3/3 | 4048 |

Run kinds: cold = every cache and process reset before each run; warm-disk = on-disk caches kept, fresh processes; warm-daemon = caches and krit and JVM daemons kept; body-edit and abi-edit = warm-daemon after a comment-only or public-API edit.
CPU and max RSS cover the krit process and the helpers it waited for. Daemon CPU ms and daemon lifetime-peak RSS MB cover long-lived daemons (krit serve, krit-fir, krit-types, checker) sampled from /proc before and after each run; peak RSS is the daemons' lifetime VmHWM, and n/a means no sample (non-Linux). The oracle column says whether the oracle ran or why a warm run legitimately skipped it. Raw data: results.json.
