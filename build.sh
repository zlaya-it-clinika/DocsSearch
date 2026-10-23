#!/usr/bin/env bash
# Собираем артефакты в build/ в корне репозитория.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
output="$root/build"
mkdir -p "$output"

(
	cd "$root/source/core"
	go build -o "$output/api" ./cmd/api
)
cp "$root/source/core/.env.example" "$output/.env.example"

echo "Собрано: $output/api"
