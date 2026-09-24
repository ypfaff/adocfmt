#!/usr/bin/env bash
#
# Rewrite the version input's default in action.yml to the version being
# released, so `uses: ypfaff/adocfmt@vX.Y.Z` installs that exact release
# without needing the `version` input. Runs on Linux and macOS.

set -o errexit -o nounset -o pipefail

readonly SCRIPT_NAME="${0##*/}"
readonly MARKER="# adocfmt-version-marker"

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} --version X.Y.Z --file action.yml

Rewrite the version input's default in an action.yml to the given version.

Options:
  --version VERSION  The version to write, without the leading v.
  --file FILE        The action.yml to rewrite.
  --help             Print this help.
EOF
}

err() {
  echo "${SCRIPT_NAME}: $*" >&2
  exit 1
}

main() {
  local version="" file=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version) version="${2:-}"; shift 2 ;;
      --file) file="${2:-}"; shift 2 ;;
      --help) usage; exit 0 ;;
      *) err "unknown option $1" ;;
    esac
  done

  [[ -n "${version}" ]] || err "--version is required"
  [[ -n "${file}" ]] || err "--file is required"
  [[ -w "${file}" ]] || err "cannot write ${file}"

  grep -q "${MARKER}" "${file}" || err "${file} carries no line with '${MARKER}'"

  # A suffix attached to -i is the one in-place form BSD sed (macOS) and GNU
  # sed (Linux) both accept.
  sed -i.bak -E "s/(default: ')[^']*(' ${MARKER})/\1${version}\2/" "${file}"
  rm -f "${file}.bak"

  grep -q "default: '${version}' ${MARKER}" "${file}" ||
    err "rewrite did not take effect in ${file}"

  echo "${SCRIPT_NAME}: set the action's version default to ${version}" >&2
}

main "$@"
