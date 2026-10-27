| mode | run kind | accepted/measured | median ms | p90 ms | min–max ms | median CPU ms | median max RSS MB | daemon CPU ms | daemon lifetime-peak RSS MB | oracle | verified | findings |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| structural | cold | 3/5 | 145329 | 312023 | 123050–312023 | 134621 | 1828 | n/a | n/a | n/a | n/a | 3890 |
| structural | warm-disk | 0/0 | 0 | 0 | 0–0 | 0 | 0 | n/a | n/a | n/a | n/a |  |
| structural | warm-daemon | 5/5 | 207 | 17191 | 108–17191 | 88 | 27 | 40 | 762 | n/a | 5/5 | 3890 |
| structural | body-edit | 5/5 | 333 | 474 | 258–474 | 88 | 27 | 250 | 2043 | n/a | 5/5 | 3890 |
| structural | abi-edit | 5/5 | 302 | 438 | 271–438 | 90 | 27 | 190 | 1983 | n/a | 5/5 | 3891 |

Rejected runs (excluded from timings):
- structural/cold run 1: exit code -1: 
- structural/cold run 5: exit code -1: 
- structural/warm-disk fill run (state skipped): exit code -1: 

Run kinds: cold = every cache and process reset before each run; warm-disk = on-disk caches kept, fresh processes; warm-daemon = caches and krit and JVM daemons kept; body-edit and abi-edit = warm-daemon after a comment-only or public-API edit.
CPU and max RSS cover the krit process and the helpers it waited for. Daemon CPU ms and daemon lifetime-peak RSS MB cover long-lived daemons (krit serve, krit-fir, krit-types, checker) sampled from /proc before and after each run; peak RSS is the daemons' lifetime VmHWM, and n/a means no sample (non-Linux). The oracle column says whether the oracle ran or why a warm run legitimately skipped it. Raw data: results.json.
