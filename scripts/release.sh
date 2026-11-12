#!/usr/bin/env bash
# Tagging, building and publishing a release of Samling.
#
# Usage:
#   scripts/release.sh tag vX.Y.Z      create an annotated tag on the current commit
#   scripts/release.sh build [vX.Y.Z]  build the release archives into dist/
#   scripts/release.sh publish vX.Y.Z  push the tag; GitHub Actions then tests,
#                                      builds and publishes the release
#
# "build" without a version uses the tag of the current commit, or a
# development version derived from "git describe".
#
# Environment: REMOTE (default "origin"), RELEASE_BRANCH (default "rom"; tags
# are only created on a commit that is part of this branch on the remote),
# PLATFORMS (default "linux/amd64 linux/arm64").
set -euo pipefail

cd "$(dirname "$0")/.."
remote="${REMOTE:-origin}"
branch="${RELEASE_BRANCH:-rom}"
platforms="${PLATFORMS:-linux/amd64 linux/arm64}"

die() { echo "release: $*" >&2; exit 1; }

check_version() {
	[[ "${1:-}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] ||
		die "the version must look like v1.2.3 or v1.2.3-rc.1 (got '${1:-}')"
}

cmd_tag() {
	local v="${1:-}"
	check_version "$v"
	[ -z "$(git status --porcelain)" ] || die "the working tree has uncommitted changes"
	git rev-parse -q --verify "refs/tags/$v" >/dev/null && die "tag $v already exists"
	git fetch --quiet "$remote" "$branch" || die "cannot fetch $remote/$branch"
	git merge-base --is-ancestor HEAD "$remote/$branch" ||
		die "the current commit is not part of $remote/$branch; releases are tagged from $branch"
	git tag -a "$v" -m "Samling $v"
	echo "created tag $v on $(git rev-parse --short HEAD)"
	echo "next: make publish VERSION=$v"
}

cmd_build() {
	local v="${1:-}"
	if [ -z "$v" ]; then
		v="$(git describe --tags --exact-match 2>/dev/null || true)"
		[ -n "$v" ] || v="v0.0.0-dev.$(git rev-parse --short HEAD)"
	fi
	check_version "$v"
	rm -rf dist
	mkdir -p dist
	local p os arch name
	for p in $platforms; do
		os="${p%/*}"
		arch="${p#*/}"
		name="samling-$v-$os-$arch"
		mkdir -p "dist/$name"
		CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
			-ldflags "-s -w -X main.version=${v#v}" -o "dist/$name/samling" ./cmd/samling
		cp -r README.md samling.example.yaml scripts deploy docs "dist/$name/"
		rm -rf "dist/$name/docs/manual/img" "dist/$name/scripts/manual-screenshots.js" "dist/$name/scripts/release.sh"
		[ ! -f LICENSE ] || cp LICENSE "dist/$name/"
		tar -C dist --owner=0 --group=0 --numeric-owner -czf "dist/$name.tar.gz" "$name"
		rm -rf "dist/$name"
		echo "built dist/$name.tar.gz"
	done
	(cd dist && sha256sum -- *.tar.gz >SHA256SUMS)
	echo "checksums in dist/SHA256SUMS"
}

cmd_publish() {
	local v="${1:-}"
	check_version "$v"
	git rev-parse -q --verify "refs/tags/$v" >/dev/null || die "tag $v does not exist; run: make tag VERSION=$v"
	git push "$remote" "refs/tags/$v"
	echo "pushed $v. The release workflow now tests, builds and publishes it:"
	echo "  $(git remote get-url "$remote" | sed -e 's#^git@github.com:#https://github.com/#' -e 's#\.git$##')/actions/workflows/release.yml"
}

case "${1:-}" in
tag) cmd_tag "${2:-}" ;;
build) cmd_build "${2:-}" ;;
publish) cmd_publish "${2:-}" ;;
*) sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'; exit 2 ;;
esac
