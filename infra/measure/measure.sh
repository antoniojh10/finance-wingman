#!/usr/bin/env bash
# Measures the resources the production images use, to size a hosting plan
# before deploying. It builds both Dockerfiles, runs API + web + Postgres in
# an isolated Docker network with the memory/CPU limits of a small plan,
# seeds realistic data, then samples `docker stats` while idle and under
# load, and prints the result with a monthly cost estimate.
#
# Usage: make measure  (or infra/measure/measure.sh)
#
# Tunables (environment variables):
#   MEM_LIMIT      memory limit per app container   (default 512m)
#   CPU_LIMIT      CPU limit per app container      (default 0.5)
#   IDLE_SECONDS   idle sampling window             (default 30)
#   LOAD_SECONDS   load sampling window             (default 30)
#   CONCURRENCY    parallel simulated clients       (default 4)
#   TRANSACTIONS   transactions to seed             (default 500)
#   SKIP_BUILD=1   reuse previously built images
set -euo pipefail

MEM_LIMIT=${MEM_LIMIT:-512m}
CPU_LIMIT=${CPU_LIMIT:-0.5}
IDLE_SECONDS=${IDLE_SECONDS:-30}
LOAD_SECONDS=${LOAD_SECONDS:-30}
CONCURRENCY=${CONCURRENCY:-4}
TRANSACTIONS=${TRANSACTIONS:-500}

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
PREFIX=fw-measure
NET=$PREFIX-net
PG=$PREFIX-postgres
API=$PREFIX-api
WEB=$PREFIX-web
API_IMAGE=$PREFIX-api:latest
WEB_IMAGE=$PREFIX-web:latest
EMAIL=measure@example.com
WORK=$(mktemp -d)

log() { printf '\033[1m==> %s\033[0m\n' "$*" >&2; }

remove_containers() {
	docker rm -f "$API" "$WEB" "$PG" >/dev/null 2>&1 || true
	docker network rm "$NET" >/dev/null 2>&1 || true
}
cleanup() {
	jobs -p | xargs -r kill 2>/dev/null || true
	remove_containers
	rm -rf "$WORK"
}
trap cleanup EXIT

for cmd in docker curl awk; do
	command -v "$cmd" >/dev/null || { echo "missing required command: $cmd" >&2; exit 1; }
done

# Containers publish random host ports so the script never clashes with
# `make up`, worktrees or a running dev server.
host_port() { docker port "$1" "$2" | head -n1 | sed 's/.*://'; }

wait_for() { # url, timeout seconds
	local deadline=$((SECONDS + $2))
	until curl -fsS -o /dev/null "$1" 2>/dev/null; do
		((SECONDS < deadline)) || { echo "timed out waiting for $1" >&2; return 1; }
		sleep 0.2
	done
}

if [[ ${SKIP_BUILD:-} != 1 ]]; then
	log "Building images"
	docker build -q -t "$API_IMAGE" "$ROOT/apps/api" >/dev/null
	docker build -q -t "$WEB_IMAGE" "$ROOT/apps/web" >/dev/null
fi

remove_containers # leftovers from an interrupted run
docker network create "$NET" >/dev/null

log "Starting Postgres"
docker run -d --name "$PG" --network "$NET" \
	-e POSTGRES_USER=finance -e POSTGRES_PASSWORD=finance -e POSTGRES_DB=finance \
	postgres:18-alpine >/dev/null
until docker exec "$PG" pg_isready -U finance -d finance -q 2>/dev/null; do sleep 0.5; done
sleep 1 # pg_isready answers during the init restart

log "Starting API (limits: ${MEM_LIMIT} RAM, ${CPU_LIMIT} CPU)"
api_started=$(date +%s%N)
docker run -d --name "$API" --network "$NET" -p 127.0.0.1::8080 \
	--memory "$MEM_LIMIT" --cpus "$CPU_LIMIT" \
	-e DATABASE_URL="postgres://finance:finance@$PG:5432/finance?sslmode=disable" \
	-e INITIAL_USERS="$EMAIL:Measure" -e EMAIL_PROVIDER=log \
	-e LOGIN_EMAILS_PER_HOUR=100 -e APP_TIMEZONE=America/Mexico_City \
	"$API_IMAGE" >/dev/null
API_URL="http://127.0.0.1:$(host_port "$API" 8080)"
wait_for "$API_URL/healthz" 60
api_ready_ms=$((($(date +%s%N) - api_started) / 1000000))

log "Starting web"
web_started=$(date +%s%N)
docker run -d --name "$WEB" --network "$NET" -p 127.0.0.1::3000 \
	--memory "$MEM_LIMIT" --cpus "$CPU_LIMIT" \
	-e API_URL="http://$API:8080" -e API_PUBLIC_URL="$API_URL" \
	-e APP_TIMEZONE=America/Mexico_City -e DEFAULT_CURRENCY=MXN \
	"$WEB_IMAGE" >/dev/null
WEB_URL="http://127.0.0.1:$(host_port "$WEB" 3000)"
wait_for "$WEB_URL/login" 60
web_ready_ms=$((($(date +%s%N) - web_started) / 1000000))

log "Signing in and seeding $TRANSACTIONS transactions"
api() { # method path [json]
	curl -fsS -X "$1" "$API_URL$2" -H "Authorization: Bearer ${TOKEN:-}" \
		-H 'Content-Type: application/json' ${3:+-d "$3"}
}
json_field() { sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p"; }

api POST /api/v1/auth/login "{\"email\":\"$EMAIL\"}" >/dev/null
sleep 0.5
magic=$(docker logs "$API" 2>&1 | grep -o 'token=[A-Za-z0-9_-]*' | tail -n1 | cut -d= -f2)
TOKEN=$(api POST /api/v1/auth/verify "{\"token\":\"$magic\"}" | json_field token)
[[ -n $TOKEN ]] || { echo "could not sign in" >&2; exit 1; }

checking=$(api POST /api/v1/accounts '{"name":"Checking","type":"checking","currency":"MXN","initial_balance":2500000}' | json_field id)
card=$(api POST /api/v1/accounts '{"name":"Card","type":"credit_card","currency":"MXN"}' | json_field id)
api POST /api/v1/accounts '{"name":"Savings","type":"savings","currency":"USD","initial_balance":100000}' >/dev/null
categories=()
for name in Groceries Restaurants Transport Home Health Fun; do
	categories+=("$(api POST /api/v1/categories "{\"name\":\"$name\",\"kind\":\"expense\"}" | json_field id)")
done
salary=$(api POST /api/v1/categories '{"name":"Salary","kind":"income"}' | json_field id)

# Spread transactions over the last year, in batches of 100.
seeded=0
while ((seeded < TRANSACTIONS)); do
	items=()
	for ((i = 0; i < 100 && seeded < TRANSACTIONS; i++, seeded++)); do
		day=$(date -d "-$((seeded * 365 / TRANSACTIONS)) days" +%F)
		if ((seeded % 15 == 0)); then
			items+=("{\"type\":\"income\",\"account_id\":\"$checking\",\"category_id\":\"$salary\",\"amount\":3500000,\"occurred_on\":\"$day\",\"description\":\"Salary\"}")
		else
			account=$checking
			((seeded % 3 == 0)) && account=$card
			category=${categories[$((seeded % ${#categories[@]}))]}
			items+=("{\"type\":\"expense\",\"account_id\":\"$account\",\"category_id\":\"$category\",\"amount\":$((5000 + (seeded * 7919) % 200000)),\"occurred_on\":\"$day\",\"description\":\"Expense $seeded\"}")
		fi
	done
	(IFS=,; api POST /api/v1/transactions/batch "{\"items\":[${items[*]}]}" >/dev/null)
done

# Warm up every page once so lazy compilation and caches don't skew the
# idle sample.
pages=(/ /transactions /accounts /categories /subscriptions /settings)
endpoints=(/api/v1/summary /api/v1/accounts /api/v1/transactions?limit=50 /api/v1/recurring/upcoming?days=7)
first_page_ms=$(curl -fsS -o /dev/null -w '%{time_total}' -b "fw_session=$TOKEN" "$WEB_URL/" | awk '{printf "%d", $1 * 1000}')
for p in "${pages[@]}"; do curl -fsS -o /dev/null -b "fw_session=$TOKEN" "$WEB_URL$p"; done

sample() { # phase seconds
	local deadline=$((SECONDS + $2))
	while ((SECONDS < deadline)); do
		docker stats --no-stream --format "$1,{{.Name}},{{.CPUPerc}},{{.MemUsage}}" "$API" "$WEB" "$PG" >>"$WORK/stats.csv"
	done
}

client() { # deadline
	local i=0
	while ((SECONDS < $1)); do
		if ((i % 3 == 0)); then
			curl -sS -o /dev/null -w 'api %{http_code} %{time_total}\n' -H "Authorization: Bearer $TOKEN" \
				"$API_URL${endpoints[$((i / 3 % ${#endpoints[@]}))]}"
		else
			curl -sS -o /dev/null -w 'web %{http_code} %{time_total}\n' -b "fw_session=$TOKEN" \
				"$WEB_URL${pages[$((i % ${#pages[@]}))]}"
		fi
		i=$((i + 1))
	done >>"$WORK/requests.txt"
}

log "Sampling idle usage for ${IDLE_SECONDS}s"
sample idle "$IDLE_SECONDS"

log "Sampling under load for ${LOAD_SECONDS}s (${CONCURRENCY} clients)"
load_deadline=$((SECONDS + LOAD_SECONDS))
for ((c = 0; c < CONCURRENCY; c++)); do client "$load_deadline" & done
sample load "$LOAD_SECONDS"
wait

db_bytes=$(docker exec "$PG" psql -U finance -d finance -tAc "SELECT pg_database_size('finance')")
api_image_mb=$(docker image inspect -f '{{.Size}}' "$API_IMAGE" | awk '{printf "%d", $1 / 1048576}')
web_image_mb=$(docker image inspect -f '{{.Size}}' "$WEB_IMAGE" | awk '{printf "%d", $1 / 1048576}')

log "Results"
awk -F, \
	-v api="$API" -v web="$WEB" -v pg="$PG" \
	-v mem_limit="$MEM_LIMIT" -v cpu_limit="$CPU_LIMIT" \
	-v load_seconds="$LOAD_SECONDS" -v concurrency="$CONCURRENCY" \
	-v db_bytes="$db_bytes" -v api_image_mb="$api_image_mb" -v web_image_mb="$web_image_mb" \
	-v api_ready_ms="$api_ready_ms" -v web_ready_ms="$web_ready_ms" -v first_page_ms="$first_page_ms" \
	-v requests="$WORK/requests.txt" '
function mib(v,   n, unit) {
	n = v + 0; unit = v; sub(/^[0-9.]+/, "", unit)
	if (unit == "B") return n / 1048576
	if (unit == "KiB" || unit == "kB") return n / 1024
	if (unit == "GiB" || unit == "GB") return n * 1024
	return n
}
function pct(sorted, n, p) { return n ? sorted[int((n - 1) * p) + 1] : 0 }
{
	split($4, m, " / "); mem = mib(m[1]); cpu = $3 + 0
	k = $1 SUBSEP $2
	cnt[k]++; cpu_sum[k] += cpu; mem_sum[k] += mem
	if (mem > mem_max[k]) mem_max[k] = mem
	if (cpu > cpu_max[k]) cpu_max[k] = cpu
}
END {
	name[api] = "api (Go)"; name[web] = "web (Next.js)"; name[pg] = "postgres"
	order[1] = api; order[2] = web; order[3] = pg

	printf "\nLimits per app container: %s RAM, %s CPU\n\n", mem_limit, cpu_limit
	printf "%-15s %10s %10s %10s %10s %10s\n", "service", "idle MiB", "idle CPU%", "load MiB", "peak MiB", "load CPU%"
	for (i = 1; i <= 3; i++) {
		c = order[i]; ki = "idle" SUBSEP c; kl = "load" SUBSEP c
		idle_mem[c] = cnt[ki] ? mem_sum[ki] / cnt[ki] : 0
		idle_cpu[c] = cnt[ki] ? cpu_sum[ki] / cnt[ki] : 0
		load_cpu[c] = cnt[kl] ? cpu_sum[kl] / cnt[kl] : 0
		printf "%-15s %10.1f %10.2f %10.1f %10.1f %10.1f\n", name[c], idle_mem[c], idle_cpu[c],
			cnt[kl] ? mem_sum[kl] / cnt[kl] : 0, mem_max[kl], load_cpu[c]
		total_idle_mem += idle_mem[c]; total_idle_cpu += idle_cpu[c]
		total_load_mem += cnt[kl] ? mem_sum[kl] / cnt[kl] : 0; total_load_cpu += load_cpu[c]
	}

	while ((getline line < requests) > 0) {
		split(line, r, " "); kind = r[1]
		total[kind]++; if (r[2] !~ /^2/) errors[kind]++
		lat[kind, total[kind]] = r[3] * 1000
	}
	printf "\nLoad: %d clients for %ds\n", concurrency, load_seconds
	for (kind in total) {
		n = total[kind]
		for (i = 1; i <= n; i++) s[i] = lat[kind, i]
		# insertion sort is fine for a few thousand samples
		for (i = 2; i <= n; i++) { v = s[i]; j = i - 1; while (j > 0 && s[j] > v) { s[j + 1] = s[j]; j-- } s[j + 1] = v }
		printf "  %-4s %6d requests  %6.1f req/s  p50 %5.0f ms  p95 %5.0f ms  errors %d\n",
			kind, n, n / load_seconds, pct(s, n, 0.5), pct(s, n, 0.95), errors[kind] + 0
	}

	printf "\nStartup: API ready in %d ms, web ready in %d ms, first dashboard render %d ms\n", api_ready_ms, web_ready_ms, first_page_ms
	printf "Database size: %.1f MiB   Images: api %d MiB, web %d MiB\n", db_bytes / 1048576, api_image_mb, web_image_mb

	# Railway bills per second: ~$10 per GB of RAM and ~$20 per vCPU per
	# month, $0.15 per GB of volume. A two-user app is idle nearly all the
	# time, so the idle averages are the realistic month; the load figures
	# are an upper bound for a month of constant traffic.
	vol_gb = db_bytes / 1073741824 + 0.25
	idle_cost = total_idle_mem / 1024 * 10 + total_idle_cpu / 100 * 20 + vol_gb * 0.15
	busy_cost = total_load_mem / 1024 * 10 + total_load_cpu / 100 * 20 + vol_gb * 0.15
	printf "\nEstimated monthly cost\n"
	printf "  Railway Hobby:  usage ~$%.2f (mostly idle) to ~$%.2f (constant load); plan is $5 incl. $5 usage -> $%.2f\n",
		idle_cost, busy_cost, (idle_cost > 5 ? idle_cost : 5)
	fits = (mem_max["load" SUBSEP api] < 512 && mem_max["load" SUBSEP web] < 512 && db_bytes < 1073741824)
	printf "  Seenode Basic:  $12 fixed (2 x 512 MB apps + 1 GB Postgres) -> %s\n",
		fits ? "fits (peak memory under 512 MiB, DB under 1 GB)" : "DOES NOT FIT the Basic tier"
}' "$WORK/stats.csv"
