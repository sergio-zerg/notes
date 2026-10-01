#!/usr/bin/env bash
#
# Brings up the whole notes stack in Docker: PostgreSQL, the Go API (REST and
# gRPC) and the Flutter web client.
#
#   ./up.sh              build if needed and start everything
#   ./up.sh rebuild      rebuild images from scratch and restart
#   ./up.sh down         stop and remove containers, keep the database
#   ./up.sh down --volumes   also delete the database — this loses every note
#   ./up.sh logs [svc]   follow logs (all services, or one: web, notes-app, db)
#   ./up.sh status       show what is running and the published ports
#
# Environment overrides, all optional:
#   WEB_PORT=8081                host port for the web client
#   WEB_API_BASE_URL=http://host:8080   API address compiled into the web bundle
#   API_KEYS=...                 keys the API accepts (see backend/.env.example)
#   STACK_PROJECT=backend        compose project name; changing it starts a
#                                second database and collides on port 5432

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$ROOT/docker-compose.yml"

die() { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }
say() { printf '\033[36m==>\033[0m %s\n' "$*"; }

# --- preflight --------------------------------------------------------------

command -v docker >/dev/null 2>&1 || die "docker is not installed"

docker info >/dev/null 2>&1 || die "docker is not running — start Docker Desktop and retry"

# Used below to tell "started but not answering yet" from "started and fine".
# Without it a machine without curl waits out the full timeout and then reports
# the wrong problem.
command -v curl >/dev/null 2>&1 || die "curl is not installed (macOS ships it; apt install curl)"

# The backend stack reads backend/.env for DB credentials and API keys. Compose
# would otherwise look for .env next to this script, where there is none, and
# silently fall back to the defaults in docker-compose.yml.
#
# The project name is pinned because compose would otherwise derive it from this
# directory ("notes") and start a *second* stack: the existing database container
# would still hold port 5432, so the new one fails to bind, and even if it did not,
# it would come up against its own empty volume rather than the notes already
# written. Naming the project "backend" makes this one deployment of the same
# services, reusing the volume that is already there.
COMPOSE_ARGS=(-p "${STACK_PROJECT:-backend}" -f "$COMPOSE_FILE")
if [[ -f "$ROOT/backend/.env" ]]; then
    COMPOSE_ARGS+=(--env-file "$ROOT/backend/.env")
fi

compose() { docker compose "${COMPOSE_ARGS[@]}" "$@"; }

# --- commands ---------------------------------------------------------------

cmd_up() {
    local rebuild="${1:-}"

    if [[ "$rebuild" == "--rebuild" ]]; then
        say "rebuilding images (the Flutter SDK layer is cached, this is not slow the first time)"
    else
        say "starting db, api and web"
    fi

    # --build is unconditional on a fresh up so a first run needs no extra flag;
    # compose skips the work when the image and context are both unchanged.
    compose up -d $rebuild

    # Compose returned before either service was actually serving. The API is
    # checked first because it is the one that can still be running migrations
    # when its container reports started, and a web client answering on its port
    # while the API is unready is exactly the case worth waiting out.
    wait_for_url "http://localhost:8080/healthz" "API"
    wait_for_url "http://localhost:${WEB_PORT:-8081}/" "web client"

    echo
    say "stack is up"
    printf '    web      http://localhost:%s\n' "${WEB_PORT:-8081}"
    printf '    api      http://localhost:8080      (gRPC :50051)\n'
    printf '    metrics  http://localhost:8080/metrics\n'
    echo
    printf '    API keys come from %s — sign in with one of them.\n' \
        "$([[ -f "$ROOT/backend/.env" ]] && echo 'backend/.env (API_KEYS)' || echo 'the compose default (dev-secret-key-change-me)')"
}

wait_for_url() {
    local url="$1" what="$2" attempts=30
    while (( attempts-- > 0 )); do
        if curl -fsS -o /dev/null --max-time 2 "$url" 2>/dev/null; then
            return 0
        fi
        sleep 1
    done
    say "warning: $what did not answer at $url — check './up.sh logs'"
}

cmd_down() {
    if [[ "${1:-}" == "--volumes" || "${1:-}" == "-v" ]]; then
        printf 'This deletes the PostgreSQL volume: every note is lost.\n'
        read -r -p 'Type "delete" to continue: ' answer || die "aborted: no answer on stdin"
        [[ "$answer" == "delete" ]] || die "aborted"
        say "removing containers and the database volume"
        compose down -v
    else
        say "removing containers (the database volume is kept)"
        compose down
    fi
    # backend/docker-compose.yml does not know about the web service, so on its
    # own it leaves the container behind. This is the one command that takes the
    # whole project down.
    printf '\nIf anything is left over, `docker ps -a --filter name=backend-` lists it.\n'
}

cmd_logs() {
    compose logs -f "$@"
}

case "${1:-up}" in
    up)      cmd_up "" ;;
    rebuild) cmd_up "--build" ;;
    down)    shift; cmd_down "${1:-}" ;;
    logs)    shift; cmd_logs "$@" ;;
    status)  compose ps ;;
    *)       die "unknown command '$1' — try: up, rebuild, down, logs, status" ;;
esac