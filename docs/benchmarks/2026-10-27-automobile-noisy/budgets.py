"""Summarize benchanalysis results.json files: spread, phases, budgets.

usage: python3 -I budgets.py out.csv results.json[:state,state] ...
Budgets: median x1.5 for wall and CPU, x1.25 for RSS, 1 s floor, rounded
up to two significant figures. Daemon states (warm-daemon, body-edit,
abi-edit) budget daemon CPU and RSS; cold and warm-disk budget the krit
process.
"""
import json, math, sys, csv

DAEMON_STATES = {"warm-daemon", "body-edit", "abi-edit"}
STATES = ["cold", "warm-disk", "warm-daemon", "body-edit", "abi-edit"]
WALL_FACTOR, CPU_FACTOR, RSS_FACTOR, FLOOR_MS = 1.5, 1.5, 1.25, 1000
PHASES = ["typeOracle", "typeOracle/jvmAnalyze", "firCheck", "dispatch", "crossFileAnalysis", "parse"]

def pct(v, p):
    # nearest-rank, same as the harness
    v = sorted(v)
    if not v:
        return None
    r = max(1, (p * len(v) + 99) // 100)
    return v[r - 1]

def load(specs):
    # spec: path[:state,state]  (state filter optional)
    runs, metas = [], []
    for spec in specs:
        path, _, only = spec.partition(":")
        d = json.load(open(path))
        metas.append(d["metadata"])
        src = path.rstrip("/").split("/")[-2]
        for r in d["runs"]:
            if only and r["state"] not in only.split(","):
                continue
            r["_src"] = src
            runs.append(r)
    return runs, metas

def nice(ms):
    # round a budget up to 2 significant figures
    if ms <= 0:
        return 0
    mag = 10 ** max(0, int(math.log10(ms)) - 1)
    return int(math.ceil(ms / mag) * mag)

def main():
    out_csv = sys.argv[1]
    runs, metas = load(sys.argv[2:])
    modes = []
    for r in runs:
        if r["mode"] not in modes:
            modes.append(r["mode"])
    rows = []
    for m in modes:
        for s in STATES:
            meas = [r for r in runs if r["mode"] == m and r["state"] == s and r["measured"]]
            acc = [r for r in meas if not r.get("rejected")]
            fills = [r for r in runs if r["mode"] == m and r["state"] == s and not r["measured"]]
            if not meas and not fills:
                continue
            wall = [r["wallMs"] for r in acc]
            cpu = [r["userCpuMs"] + r["sysCpuMs"] for r in acc]
            rss = [r["maxRssKb"] / 1024 for r in acc]
            dcpu = [r["daemon"]["cpuMs"] for r in acc if r.get("daemon") and r["daemon"].get("pids")]
            drss = [r["daemon"]["peakRssKb"] / 1024 for r in acc if r.get("daemon") and r["daemon"].get("pids")]
            loads = [float(r["loadAvg"].split()[0]) for r in acc if r.get("loadAvg")]
            row = dict(mode=m, state=s, source="+".join(sorted({r["_src"] for r in meas + fills})), accepted=len(acc), measured=len(meas),
                       rejected="; ".join(sorted({(r.get("rejected") or "")[:60] for r in meas + fills if r.get("rejected")})),
                       min=min(wall) if wall else None, p25=pct(wall, 25), median=pct(wall, 50),
                       p75=pct(wall, 75), p90=pct(wall, 90), max=max(wall) if wall else None,
                       cpu_median=pct(cpu, 50), rss_median_mb=round(pct(rss, 50)) if rss else None,
                       daemon_cpu_median=pct(dcpu, 50), daemon_rss_median_mb=round(pct(drss, 50)) if drss else None,
                       load1_min=min(loads) if loads else None, load1_max=max(loads) if loads else None,
                       oracle_skip=",".join(sorted({r.get("oracleSkip") for r in acc if r.get("oracleSkip")})),
                       findings=",".join(str(x) for x in sorted({r.get("findings") for r in acc})))
            for ph in PHASES:
                v = [r.get("phasesMs", {}).get(ph) for r in acc]
                v = [x for x in v if x is not None]
                row["phase:" + ph] = pct(v, 50)
            if wall:
                med = pct(wall, 50)
                row["budget_wall_ms"] = nice(max(med * WALL_FACTOR, FLOOR_MS))
                # Daemon states budget the daemons (the client is near-idle);
                # in-process states budget the krit process.
                if s in DAEMON_STATES:
                    row["budget_basis"] = "daemon"
                    row["budget_cpu_ms"] = nice(max(pct(dcpu, 50) * CPU_FACTOR, FLOOR_MS)) if dcpu else None
                    row["budget_rss_mb"] = nice(pct(drss, 50) * RSS_FACTOR) if drss else None
                else:
                    row["budget_basis"] = "process"
                    row["budget_cpu_ms"] = nice(max(pct(cpu, 50) * CPU_FACTOR, FLOOR_MS))
                    row["budget_rss_mb"] = nice(pct(rss, 50) * RSS_FACTOR)
            rows.append(row)
    keys = []
    for r in rows:
        for k in r:
            if k not in keys:
                keys.append(k)
    with open(out_csv, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=keys)
        w.writeheader()
        w.writerows(rows)
    for r in rows:
        print({k: v for k, v in r.items() if v not in (None, "")})

main()
