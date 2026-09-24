#!/usr/bin/env bash
#
# Print the version the next release gets, derived from the newest v* tag.
# Runs on Linux and macOS.

set -o errexit -o nounset -o pipefail

readonly SCRIPT_NAME="${0##*/}"

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} (--bump major|minor|patch | --explicit X.Y.Z)

Print the next version, without the leading v. The caller builds the tag.

Options:
  --bump TYPE        Raise the newest tag by this part.
  --explicit VERSION Use this version instead of raising the newest tag.
  --help             Print this help.
EOF
}

# Diagnostics go to stderr, so that the version is the only thing on stdout.
err() {
  echo "${SCRIPT_NAME}: $*" >&2
  exit 1
}

info() {
  echo "${SCRIPT_NAME}: $*" >&2
}

# The newest tag by version order, which git sorts itself; sort -V is not on
# every macOS. Empty output means the repository has no release yet.
latest_tag() {
  git tag --list 'v[0-9]*' --sort=-version:refname | head -n 1
}

raise() {
  local version="$1" part="$2"
  local major minor patch
  IFS=. read -r major minor patch <<<"${version}"

  case "${part}" in
    major) echo "$((major + 1)).0.0" ;;
    minor) echo "${major}.$((minor + 1)).0" ;;
    patch) echo "${major}.${minor}.$((patch + 1))" ;;
    *) err "unknown bump type ${part}, expected major, minor or patch" ;;
  esac
}

main() {
  local bump="" explicit=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --bump) bump="${2:-}"; shift 2 ;;
      --explicit) explicit="${2:-}"; shift 2 ;;
      --help) usage; exit 0 ;;
      *) err "unknown option $1" ;;
    esac
  done

  local version
  if [[ -n "${explicit}" ]]; then
    version="${explicit#v}"
  elif [[ -n "${bump}" ]]; then
    local tag
    tag="$(latest_tag)"
    if [[ -z "${tag}" ]]; then
      info "no v* tag yet, counting from v0.0.0"
      tag="v0.0.0"
    fi
    info "newest tag is ${tag}"
    version="$(raise "${tag#v}" "${bump}")"
  else
    err "either --bump or --explicit is required"
  fi

  [[ "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
    err "${version} is no X.Y.Z version"

  # A tag that exists would make the release overwrite a published one.
  if git rev-parse --verify --quiet "refs/tags/v${version}" >/dev/null; then
    err "tag v${version} exists already"
  fi

  echo "${version}"
}

main "$@"
