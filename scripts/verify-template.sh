#!/usr/bin/env bash
set -euo pipefail

template="${1:-admin}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
template_dir="${root}/${template}"
core_candidate="${FBA_GO_ROOT:-${root}/../fba-go}"
generated_parent="${FBAGO_VERIFY_OUT:-}"

if [[ ! -d "${template_dir}" ]]; then
	echo "template ${template} does not exist under ${root}" >&2
	exit 1
fi

if ! core_root="$(cd "${core_candidate}" 2>/dev/null && pwd)"; then
	echo "FBA_GO_ROOT must point to a fba-go checkout; got ${core_candidate}" >&2
	exit 1
fi

if [[ ! -d "${core_root}/cmd/fbago" ]]; then
	echo "FBA_GO_ROOT must point to a fba-go checkout; got ${core_root}" >&2
	exit 1
fi

if ! command -v go >/dev/null 2>&1; then
	echo "go is required" >&2
	exit 1
fi

if ! command -v make >/dev/null 2>&1; then
	echo "make is required" >&2
	exit 1
fi

export GOCACHE="${GOCACHE:-${root}/.cache/go-build}"

cleanup() {
	if [[ -n "${generated_parent:-}" && "${keep_generated:-}" != "1" ]]; then
		rm -rf "${generated_parent}"
	fi
}

if [[ -z "${generated_parent}" ]]; then
	generated_parent="$(mktemp -d)"
	trap cleanup EXIT
else
	mkdir -p "${generated_parent}"
fi

generated_dir="${generated_parent}/${template}-generated"
rm -rf "${generated_dir}"

echo "==> verifying template ${template}"
(
	cd "${template_dir}"
	make clean
	make test
	make build
	make clean
)

echo "==> generating project from ${template}"
(
	cd "${core_root}"
	go run ./cmd/fbago init "github.com/acme/fbago-${template}-generated" --template "${template_dir}" --dir "${generated_dir}"
)

echo "==> verifying generated project"
(
	cd "${generated_dir}"
	# The template may target the next unreleased fba-go commit. Local verification
	# pins the generated project to the checkout under test instead of the module proxy.
	go mod edit -replace=github.com/yuWorm/fba-go="${core_root}"
	make tidy
	make test
	make build
	make clean
)

echo "==> template ${template} verified"
