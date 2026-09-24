#!/usr/bin/env bash
#
# Download, verify and extract the adocfmt release archive for one platform.
# Runs on Linux, macOS and Windows (Git Bash), the three GitHub-hosted runner
# families the action supports.

set -o errexit -o nounset -o pipefail

readonly SCRIPT_NAME="${0##*/}"
readonly REPOSITORY="ypfaff/adocfmt"

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} --version X.Y.Z --os OS --arch ARCH --dest DIR

Download the adocfmt release archive for one platform, verify it against the
release's checksums.txt, and extract the binary into DIR.

Options:
  --version VERSION  Release to install, without the leading v.
  --os OS            runner.os as GitHub Actions reports it (Linux, Windows, macOS).
  --arch ARCH        runner.arch as GitHub Actions reports it (X64, ARM64).
  --dest DIR         Directory to extract the binary into.
  --help             Print this help.
EOF
}

err() {
  echo "${SCRIPT_NAME}: $*" >&2
  exit 1
}

# GoReleaser's build target names, not GitHub's; see .goreleaser.yml.
goreleaser_os() {
  case "$1" in
    Linux) echo linux ;;
    Windows) echo windows ;;
    macOS) echo darwin ;;
    *) err "unsupported runner.os $1" ;;
  esac
}

goreleaser_arch() {
  case "$1" in
    X64) echo amd64 ;;
    ARM64) echo arm64 ;;
    *) err "unsupported runner.arch $1" ;;
  esac
}

# macOS carries no sha256sum by default, so this falls back to the shasum that
# ships with it instead of depending on GNU coreutils.
sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

main() {
  local version="" os="" arch="" dest=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version) version="${2:-}"; shift 2 ;;
      --os) os="${2:-}"; shift 2 ;;
      --arch) arch="${2:-}"; shift 2 ;;
      --dest) dest="${2:-}"; shift 2 ;;
      --help) usage; exit 0 ;;
      *) err "unknown option $1" ;;
    esac
  done

  [[ -n "${version}" ]] || err "--version is required"
  [[ -n "${os}" ]] || err "--os is required"
  [[ -n "${arch}" ]] || err "--arch is required"
  [[ -n "${dest}" ]] || err "--dest is required"

  local goos goarch archive base_url
  goos="$(goreleaser_os "${os}")"
  goarch="$(goreleaser_arch "${arch}")"
  archive="adocfmt_${version}_${goos}_${goarch}.tar.gz"
  base_url="https://github.com/${REPOSITORY}/releases/download/v${version}"

  # Not local: the EXIT trap runs after main has returned, where set -u would
  # find a local unset.
  workdir="$(mktemp -d)"
  trap 'rm -rf "${workdir}"' EXIT

  curl --fail --silent --show-error --location --output "${workdir}/${archive}" \
    "${base_url}/${archive}"
  curl --fail --silent --show-error --location --output "${workdir}/checksums.txt" \
    "${base_url}/checksums.txt"

  local want got
  want="$(awk -v want="${archive}" '$2 == want { print $1 }' "${workdir}/checksums.txt")"
  [[ -n "${want}" ]] || err "checksums.txt holds no entry for ${archive}"
  got="$(sha256 "${workdir}/${archive}")"
  [[ "${want}" == "${got}" ]] || err "checksum mismatch for ${archive}: expected ${want}, got ${got}"

  mkdir -p "${dest}"
  local binary="adocfmt"
  [[ "${goos}" == windows ]] && binary="adocfmt.exe"
  tar --extract --gzip --file "${workdir}/${archive}" --directory "${dest}" "${binary}"

  echo "${SCRIPT_NAME}: installed adocfmt ${version} (${goos}/${goarch}) into ${dest}" >&2
}

main "$@"
