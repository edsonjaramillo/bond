#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 vX.Y.Z"
  echo "Example: $0 v0.2.0"
}

if [ "${1:-}" = "" ]; then
  usage
  exit 1
fi

tag="$1"
if ! [[ "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: tag must match vX.Y.Z (got: ${tag})"
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Error: working tree is not clean. Commit or stash changes first."
  exit 1
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
if [ "${branch}" != "main" ]; then
  echo "Error: releases must be tagged from main (current: ${branch})"
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
  echo "Error: tag already exists locally: ${tag}"
  exit 1
fi

if git ls-remote --exit-code --tags origin "${tag}" >/dev/null 2>&1; then
  echo "Error: tag already exists on origin: ${tag}"
  exit 1
fi

echo "Creating annotated tag ${tag}..."
git tag -a "${tag}" -m "Release ${tag}"

echo "Pushing tag ${tag} to origin..."
git push origin "${tag}"

echo "Done. GitHub Actions will build and publish the release."
