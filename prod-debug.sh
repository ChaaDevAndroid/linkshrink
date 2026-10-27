#!/usr/bin/env bash
# Run the engine with prod-debug as the first argument. "$@" forwards the action you typed.
exec "$(dirname "$0")/scripts/run.sh" prod-debug "$@"