# Measuring resource usage

`make measure` sizes a hosting plan before deploying. It builds both
production Dockerfiles, runs API + web + Postgres in an isolated Docker
network (random local ports, nothing shared with `make up`), seeds a year of
data, and samples `docker stats` while idle and under load.

```bash
make measure                                  # 512 MB / 0.5 CPU per app
CPU_LIMIT=0.2 make measure                    # 0.2 CPU limit
SKIP_BUILD=1 LOAD_SECONDS=60 CONCURRENCY=8 make measure
```

| Variable | Default | Meaning |
| --- | --- | --- |
| `MEM_LIMIT` | `512m` | Memory limit per app container |
| `CPU_LIMIT` | `0.5` | CPU limit per app container |
| `IDLE_SECONDS` / `LOAD_SECONDS` | `30` | Sampling windows |
| `CONCURRENCY` | `4` | Parallel simulated clients during the load phase |
| `TRANSACTIONS` | `500` | Transactions seeded before measuring |
| `SKIP_BUILD` | — | `1` reuses the images from the previous run |

Requires Docker, curl, awk and GNU `date` (Linux or WSL).

## Reading the results

- **Idle** is what a two-user app does almost all month; Railway bills it per
  second (~$10 per GB of RAM, ~$20 per vCPU, $0.15 per GB of volume).
- **Load** (one page every few hundred ms per client) is an upper bound.
- **Peak MiB** must stay under the plan's memory limit or the container is
  killed.
- **Web p50/p95** shows how a CPU limit affects page renders; the Next.js
  server is the CPU-bound part, the Go API barely registers.

## Sample run (October 2026, WSL2)

| | 0.5 CPU | 0.2 CPU (Seenode Basic) |
| --- | --- | --- |
| API idle / peak | 22 / 17 MiB | 19 / 26 MiB |
| Web idle / peak | 54 / 76 MiB | 53 / 74 MiB |
| Postgres idle / peak | 64 / 84 MiB | 66 / 71 MiB |
| Web page p50 / p95 under load | 203 / 488 ms | 492 / 1396 ms |
| Web ready after start | 1.3 s | 2.9 s |
| Database size (500 transactions) | 8.7 MiB | 8.7 MiB |
| Railway estimate | ~$1.50 usage → $5 plan minimum | same |
