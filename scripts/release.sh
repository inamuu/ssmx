#!/usr/bin/env bash
# Release ssmx and replace the installed binary.
#
#   scripts/release.sh patch|minor|major|vX.Y.Z
#       test -> tag -> push -> wait for the GitHub Actions release
#       -> brew upgrade inamuu/tap/ssmx
#
#   scripts/release.sh --local
#       test -> build -> replace the ssmx binary found in PATH
#       (old binary goes to ~/.Trash; $(go env GOPATH)/bin if not in PATH).
#       Set SSMX_INSTALL_PATH to override.
set -euo pipefail

cd "$(dirname "$0")/.."

FORMULA="inamuu/tap/ssmx"
WORKFLOW="release.yml"

usage() {
  sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'
  exit 1
}

log() { printf '\n==> %s\n' "$*"; }

run_tests() {
  log "go vet / go test"
  go vet ./...
  go test ./...
}

latest_tag() {
  git tag --list 'v*' --sort=-v:refname | head -n 1
}

next_version() {
  local bump=$1 cur major minor patch
  cur=$(latest_tag)
  cur=${cur:-v0.0.0}
  IFS=. read -r major minor patch <<<"${cur#v}"
  case "$bump" in
    major) echo "v$((major + 1)).0.0" ;;
    minor) echo "v${major}.$((minor + 1)).0" ;;
    patch) echo "v${major}.${minor}.$((patch + 1))" ;;
    v[0-9]*.[0-9]*.[0-9]*) echo "$bump" ;;
    *) usage ;;
  esac
}

install_local() {
  run_tests

  local dest version tmp
  dest=${SSMX_INSTALL_PATH:-}
  if [ -z "$dest" ]; then
    dest=$(command -v ssmx || true)
    if [ -z "$dest" ]; then
      dest="$(go env GOPATH)/bin/ssmx"
      mkdir -p "$(dirname "$dest")"
      echo "ssmx not found in PATH. install to ${dest}"
    else
      # Follow symlinks (e.g. Homebrew's bin -> Cellar).
      dest=$(realpath "$dest")
    fi
  fi

  version="$(git describe --tags --always --dirty)-local"
  tmp=$(mktemp -d)
  log "build ssmx ${version}"
  go build -ldflags "-s -w -X github.com/inamuu/ssmx/cmd.version=${version}" -o "${tmp}/ssmx" .

  if [ -e "$dest" ]; then
    log "move old binary to ~/.Trash"
    mv "$dest" "$HOME/.Trash/ssmx.$(date +%Y%m%d%H%M%S)"
  fi
  mv "${tmp}/ssmx" "$dest"
  rmdir "$tmp"

  log "installed: $dest"
  ssmx --version
}

release() {
  local version=$1 branch run_id

  branch=$(git rev-parse --abbrev-ref HEAD)
  if [ "$branch" != "main" ]; then
    echo "release must be run on main (current: $branch)" >&2
    exit 1
  fi
  if [ -n "$(git status --porcelain)" ]; then
    echo "working tree is not clean. commit or stash first." >&2
    exit 1
  fi
  if git rev-parse -q --verify "refs/tags/${version}" >/dev/null; then
    echo "tag ${version} already exists" >&2
    exit 1
  fi
  command -v gh >/dev/null || { echo "gh command is required" >&2; exit 1; }

  run_tests

  printf '\nrelease %s (latest: %s) from %s. continue? [y/N] ' \
    "$version" "$(latest_tag)" "$(git rev-parse --short HEAD)"
  read -r answer
  [ "$answer" = "y" ] || [ "$answer" = "Y" ] || { echo "aborted"; exit 1; }

  log "push main and tag ${version}"
  git push origin main
  git tag -a "$version" -m "Release ${version}"
  git push origin "$version"

  log "wait for GitHub Actions (${WORKFLOW})"
  run_id=""
  for _ in $(seq 1 30); do
    run_id=$(gh run list --workflow "$WORKFLOW" --branch "$version" \
      --limit 1 --json databaseId --jq '.[0].databaseId' 2>/dev/null || true)
    [ -n "$run_id" ] && break
    sleep 5
  done
  if [ -z "$run_id" ]; then
    echo "release workflow run not found. check GitHub Actions." >&2
    exit 1
  fi
  gh run watch "$run_id" --exit-status

  log "brew upgrade ${FORMULA}"
  brew update
  if brew list --formula ssmx >/dev/null 2>&1; then
    brew upgrade "$FORMULA"
  else
    brew install "$FORMULA"
  fi

  log "installed"
  ssmx --version
}

[ $# -eq 1 ] || usage
case "$1" in
  --local) install_local ;;
  -h | --help) usage ;;
  *) release "$(next_version "$1")" ;;
esac
