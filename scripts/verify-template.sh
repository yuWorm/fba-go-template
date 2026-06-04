#!/usr/bin/env bash
set -euo pipefail

template="${1:-admin}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
template_dir="${root}/${template}"
generated_parent="${FBAGO_VERIFY_OUT:-}"
backup_dir=""
restore_template_module=0

if [[ -n "${FBA_GO_ROOT:-}" ]]; then
	core_candidate="${FBA_GO_ROOT}"
elif [[ -d "${root}/../fba-go/cmd/fbago" ]]; then
	core_candidate="${root}/../fba-go"
else
	# The template repo is commonly checked out as fba-go/templates/fba-go-template.
	# In that submodule layout, the core checkout is two levels above the template root.
	core_candidate="${root}/../.."
fi

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
	if [[ "${restore_template_module}" == "1" ]]; then
		cp "${backup_dir}/go.mod" "${template_dir}/go.mod"
		if [[ -f "${backup_dir}/go.sum" ]]; then
			cp "${backup_dir}/go.sum" "${template_dir}/go.sum"
		else
			rm -f "${template_dir}/go.sum"
		fi
	fi
	if [[ -n "${backup_dir:-}" ]]; then
		rm -rf "${backup_dir}"
	fi
	if [[ -n "${generated_parent:-}" && "${keep_generated:-}" != "1" ]]; then
		rm -rf "${generated_parent}"
	fi
}

backup_template_module() {
	backup_dir="$(mktemp -d)"
	cp "${template_dir}/go.mod" "${backup_dir}/go.mod"
	if [[ -f "${template_dir}/go.sum" ]]; then
		cp "${template_dir}/go.sum" "${backup_dir}/go.sum"
	fi
	restore_template_module=1
}

if [[ -z "${generated_parent}" ]]; then
	generated_parent="$(mktemp -d)"
else
	mkdir -p "${generated_parent}"
fi
trap cleanup EXIT

generated_dir="${generated_parent}/${template}-generated"
rm -rf "${generated_dir}"

echo "==> verifying template ${template}"
backup_template_module
(
	cd "${template_dir}"
	go mod edit -replace=github.com/yuWorm/fba-go="${core_root}"
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
	make tidy
	make test
	make build
	make clean
)

echo "==> template ${template} verified"
