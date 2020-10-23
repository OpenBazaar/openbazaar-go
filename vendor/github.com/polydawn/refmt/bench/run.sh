#!/bin/bash
set -euo pipefail

echo "--- regular bench ---"
go test -bench=. "$@"
echo
echo "--- NOGC bench ---"
GOGC=off go test -bench=. "$@"
echo
echo "--- bench mem ---"
go test -bench=. -benchmem "$@"
