| mode | run kind | accepted/measured | median ms | p90 ms | min–max ms | median CPU ms | median max RSS MB | daemon CPU ms | daemon lifetime-peak RSS MB | oracle | verified | findings |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| fir-checkers | cold | 3/3 | 139619 | 141872 | 138918–141872 | 393582 | 3065 | n/a | n/a | ran | n/a | 4010 |
| fir-checkers | warm-disk | 3/3 | 8835 | 9112 | 8421–9112 | 18584 | 1317 | n/a | n/a | skipped: findings-bundle-hit (3) | 3/3 | 4010 |
| fir-checkers | warm-daemon | 3/3 | 69289 | 70630 | 68087–70630 | 157 | 37 | 146500 | 6874 | ran | 3/3 | 4010 |
| fir-checkers | body-edit | 3/3 | 76420 | 79327 | 71856–79327 | 137 | 37 | 151810 | 6066 | ran | 3/3 | 4010 |
| fir-checkers | abi-edit | 3/3 | 84605 | 87201 | 80162–87201 | 151 | 37 | 170770 | 6257 | ran | 3/3 | 4011 |

Run kinds: cold = every cache and process reset before each run; warm-disk = on-disk caches kept, fresh processes; warm-daemon = caches and krit and JVM daemons kept; body-edit and abi-edit = warm-daemon after a comment-only or public-API edit.
CPU and max RSS cover the krit process and the helpers it waited for. Daemon CPU ms and daemon lifetime-peak RSS MB cover long-lived daemons (krit serve, krit-fir, krit-types, checker) sampled from /proc before and after each run; peak RSS is the daemons' lifetime VmHWM, and n/a means no sample (non-Linux). The oracle column says whether the oracle ran or why a warm run legitimately skipped it. Raw data: results.json.
