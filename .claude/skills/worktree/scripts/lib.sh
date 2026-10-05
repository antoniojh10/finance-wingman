# Shared helpers for the worktree scripts. Source it; do not execute it.

# Path of the main checkout (the first entry of `git worktree list`).
main_checkout() {
  git worktree list --porcelain | awk '/^worktree /{print substr($0, 10); exit}'
}

# Paths of every linked worktree (excluding the main checkout).
linked_worktrees() {
  git worktree list --porcelain | awk '/^worktree /{print substr($0, 10)}' | tail -n +2
}

# Turns a branch or directory name into a lowercase [a-z0-9_] slug.
slugify() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/_/g; s/^_+|_+$//g' | cut -c1-40
}

# set_env FILE KEY VALUE: replaces KEY=... in FILE, or appends it.
set_env() {
  local file=$1 key=$2 value=$3
  if grep -qE "^${key}=" "$file"; then
    sed -i -E "s|^${key}=.*|${key}=${value}|" "$file"
  else
    printf '%s=%s\n' "$key" "$value" >>"$file"
  fi
}

# get_env FILE KEY: prints the value of KEY in FILE (empty if missing).
get_env() {
  [ -f "$1" ] && sed -nE "s/^$2=(.*)$/\1/p" "$1" | tail -n 1
}

# Runs psql inside the shared docker-compose Postgres of the main checkout.
shared_psql() {
  local main
  main=$(main_checkout)
  docker compose --project-directory "$main" -f "$main/docker-compose.yml" \
    exec -T postgres psql -U finance -d finance -v ON_ERROR_STOP=1 "$@"
}
