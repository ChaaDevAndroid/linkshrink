#!/usr/bin/env bash

# Project root: this script lives in scripts/, so go one folder up.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# First argument is the environment, second is the action (lowercased, like batch /I).
TARGET="$(printf '%s' "${1:-dev}" | tr '[:upper:]' '[:lower:]')"
ACTION="$(printf '%s' "${2:-up}" | tr '[:upper:]' '[:lower:]')"

usage() {
  echo "Usage: dev, prod or prod-debug, then an action"
  echo
  echo "Actions:"
  echo "  up        build and start. dev runs in the foreground with watch, prod in the background"
  echo "  down      stop and remove containers, keep the data"
  echo "  restart   restart the api container only"
  echo "  ps        show container status"
  echo "  logs      follow the api logs"
  echo "  config    print the merged compose configuration"
  echo "  reset     dev only: delete containers AND the database volume"
  echo "  close     prod-debug only: remove the published database port"
  echo "  help      show this text"
  exit 1
}

# Help must work before any file checks.
if [ "$TARGET" = "help" ] || [ "$ACTION" = "help" ]; then
  usage
fi

# Each target picks an env file. prod-debug reuses the prod one.
case "$TARGET" in
  dev)        ENVFILE=".env.dev" ;;
  prod)       ENVFILE=".env.prod" ;;
  prod-debug) ENVFILE=".env.prod" ;;
  *)          usage ;;
esac

# Stop early with a clear message if a required file is missing.
if [ ! -f "$ROOT/$ENVFILE" ]; then
  echo "Missing file: $ENVFILE"
  exit 1
fi
if [ "$TARGET" = "prod-debug" ] && [ ! -f "$ROOT/compose.prod-debug.yaml" ]; then
  echo "Missing file: compose.prod-debug.yaml"
  exit 1
fi

# Flags shared by every docker compose command. Later files override earlier ones.
BASE=(--project-directory "$ROOT" --env-file "$ROOT/$ENVFILE" -f "$ROOT/compose.yaml")
case "$TARGET" in
  dev)        BASE+=(-f "$ROOT/compose.dev.yaml") ;;
  prod)       BASE+=(-f "$ROOT/compose.prod.yaml") ;;
  prod-debug) BASE+=(-f "$ROOT/compose.prod.yaml" -f "$ROOT/compose.prod-debug.yaml") ;;
esac

# Run the action that matches.
case "$ACTION" in
  up)
    case "$TARGET" in
      dev)
        # Dev: foreground, rebuild the api when code files change.
        docker compose "${BASE[@]}" up --build --watch ;;
      prod)
        # Prod: detached, so the terminal is free.
        docker compose "${BASE[@]}" up -d --build ;;
      prod-debug)
        # Prod plus the override that publishes Postgres on 127.0.0.1 port 5433 only.
        docker compose "${BASE[@]}" up -d --build || exit 1
        echo
        echo "prod-debug is up. Postgres is published on 127.0.0.1:5433, on this machine only."
        echo "Close the port when you are done: ./prod-debug close" ;;
    esac
    ;;
  close)
    # Recreate prod without the debug override, which removes the published port.
    if [ "$TARGET" != "prod-debug" ]; then
      echo "close only applies to prod-debug."
      exit 1
    fi
    docker compose --project-directory "$ROOT" --env-file "$ROOT/.env.prod" \
      -f "$ROOT/compose.yaml" -f "$ROOT/compose.prod.yaml" up -d || exit 1
    echo "Port closed. Prod runs without a published database port."
    ;;
  down)
    # Remove the containers and network. The data volume is kept.
    docker compose "${BASE[@]}" down ;;
  restart)
    # Restart only the api container, not the database.
    docker compose "${BASE[@]}" restart api ;;
  ps)
    docker compose "${BASE[@]}" ps ;;
  logs)
    # Follow the api logs. Ctrl+C stops following, not the app.
    docker compose "${BASE[@]}" logs -f api ;;
  config)
    # Print the merged configuration without starting anything.
    docker compose "${BASE[@]}" config ;;
  reset)
    # Deleting the volume is only allowed on dev.
    if [ "$TARGET" = "dev" ]; then
      docker compose "${BASE[@]}" down -v
    else
      echo "reset is blocked on $TARGET, it would delete the database volume."
      exit 1
    fi
    ;;
  *)
    usage ;;
esac