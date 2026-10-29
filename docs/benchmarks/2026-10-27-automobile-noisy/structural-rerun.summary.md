| mode | run kind | accepted/measured | median ms | p90 ms | min–max ms | median CPU ms | median max RSS MB | daemon CPU ms | daemon lifetime-peak RSS MB | oracle | verified | findings |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| structural | warm-disk | 5/5 | 3092 | 5220 | 2773–5220 | 4951 | 660 | n/a | n/a | n/a | 5/5 | 3890 |

Run kinds: cold = every cache and process reset before each run; warm-disk = on-disk caches kept, fresh processes; warm-daemon = caches and krit and JVM daemons kept; body-edit and abi-edit = warm-daemon after a comment-only or public-API edit.
CPU and max RSS cover the krit process and the helpers it waited for. Daemon CPU ms and daemon lifetime-peak RSS MB cover long-lived daemons (krit serve, krit-fir, krit-types, checker) sampled from /proc before and after each run; peak RSS is the daemons' lifetime VmHWM, and n/a means no sample (non-Linux). The oracle column says whether the oracle ran or why a warm run legitimately skipped it. Raw data: results.json.
