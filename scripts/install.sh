#!/usr/bin/env bash
set -euo pipefail

REPO="${BOND_REPO:-edsonjaramillo/bond}"
VERSION="${BOND_VERSION:-latest}"
INSTALL_DIR="${BOND_INSTALL_DIR:-$HOME/.local/bin}"
VERIFY_CHECKSUM=1

usage() {
	cat <<'EOF'
Install bond from GitHub Releases.

Usage:
  install.sh [--version vX.Y.Z] [--install-dir PATH] [--no-verify]

Environment overrides:
  BOND_REPO         GitHub repo in owner/name format (default: edsonjaramillo/bond)
  BOND_VERSION      Version tag like v0.2.0 or "latest" (default: latest)
  BOND_INSTALL_DIR  Install directory (default: ~/.local/bin)
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--version)
		VERSION="${2:-}"
		shift 2
		;;
	--install-dir)
		INSTALL_DIR="${2:-}"
		shift 2
		;;
	--no-verify)
		VERIFY_CHECKSUM=0
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "Unknown argument: $1" >&2
		usage >&2
		exit 1
		;;
	esac
done

if [[ -z "${VERSION}" ]]; then
	echo "--version requires a value" >&2
	exit 1
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "${os}" in
linux | darwin) ;;
*)
	echo "Unsupported OS: ${os}. Supported: linux, darwin." >&2
	exit 1
	;;
esac

case "${arch}" in
x86_64 | amd64) arch="amd64" ;;
arm64 | aarch64) arch="arm64" ;;
*)
	echo "Unsupported architecture: ${arch}. Supported: amd64, arm64." >&2
	exit 1
	;;
esac

have_cmd() {
	command -v "$1" >/dev/null 2>&1
}

download() {
	local url="$1"
	local output="$2"
	if have_cmd curl; then
		curl -fsSL "$url" -o "$output"
	elif have_cmd wget; then
		wget -qO "$output" "$url"
	else
		echo "Missing downloader. Install curl or wget." >&2
		exit 1
	fi
}

checksum_ok() {
	local checksums_file="$1"
	local file_name="$2"
	if have_cmd sha256sum; then
		(cd "$(dirname "${checksums_file}")" && sha256sum -c --ignore-missing "$(basename "${checksums_file}")" | grep -q "${file_name}: OK")
	elif have_cmd shasum; then
		local expected
		expected="$(grep " ${file_name}$" "${checksums_file}" | awk '{print $1}')"
		if [[ -z "${expected}" ]]; then
			return 1
		fi
		local actual
		actual="$(shasum -a 256 "$(dirname "${checksums_file}")/${file_name}" | awk '{print $1}')"
		[[ "${expected}" == "${actual}" ]]
	else
		echo "Missing checksum tool. Install sha256sum or shasum." >&2
		exit 1
	fi
}

tmp_dir="$(mktemp -d)"
cleanup() {
	rm -rf "${tmp_dir}"
}
trap cleanup EXIT

if [[ "${VERSION}" == "latest" ]]; then
	latest_url="https://api.github.com/repos/${REPO}/releases/latest"
	if have_cmd curl; then
		VERSION="$(curl -fsSL "${latest_url}" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
	elif have_cmd wget; then
		VERSION="$(wget -qO- "${latest_url}" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
	else
		echo "Missing downloader. Install curl or wget." >&2
		exit 1
	fi
fi

if [[ -z "${VERSION}" || "${VERSION}" != v* ]]; then
	echo "Failed to resolve version tag. Expected format: vX.Y.Z" >&2
	exit 1
fi

version_no_v="${VERSION#v}"
archive_name="bond_${version_no_v}_${os}_${arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${VERSION}"
archive_path="${tmp_dir}/${archive_name}"
checksums_path="${tmp_dir}/SHA256SUMS"

echo "Downloading ${archive_name} from ${REPO}@${VERSION}..."
download "${base_url}/${archive_name}" "${archive_path}"

if [[ "${VERIFY_CHECKSUM}" -eq 1 ]]; then
	echo "Downloading SHA256SUMS..."
	download "${base_url}/SHA256SUMS" "${checksums_path}"
	echo "Verifying checksum..."
	if ! checksum_ok "${checksums_path}" "${archive_name}"; then
		echo "Checksum verification failed for ${archive_name}" >&2
		exit 1
	fi
fi

mkdir -p "${tmp_dir}/extract"
tar -xzf "${archive_path}" -C "${tmp_dir}/extract"
binary_path="${tmp_dir}/extract/bond_${version_no_v}_${os}_${arch}/bond"

if [[ ! -f "${binary_path}" ]]; then
	echo "Expected binary not found in archive: ${binary_path}" >&2
	exit 1
fi

mkdir -p "${INSTALL_DIR}"
install -m 0755 "${binary_path}" "${INSTALL_DIR}/bond"

echo "bond installed to ${INSTALL_DIR}/bond"
echo "Run: ${INSTALL_DIR}/bond --help"
