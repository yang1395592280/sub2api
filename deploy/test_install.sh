#!/bin/bash

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/install.sh"

assert_eq() {
    local expected="$1"
    local actual="$2"
    local message="$3"
    if [ "$expected" != "$actual" ]; then
        echo "ASSERT FAIL: ${message}" >&2
        echo "  expected: ${expected}" >&2
        echo "  actual:   ${actual}" >&2
        exit 1
    fi
}

test_default_repo() {
    assert_eq "yang1395592280/sub2api" "$GITHUB_REPO" "default repository should point to current fork"
}

test_override_repo() {
    GITHUB_REPO="custom-owner/custom-repo"
    local url
    url=$(download_url_for_version "v1.2.3" "linux" "amd64")
    assert_eq \
        "https://github.com/custom-owner/custom-repo/releases/download/v1.2.3/sub2api_1.2.3_linux_amd64.tar.gz" \
        "$url" \
        "download URL should respect GITHUB_REPO override"
}

test_checksum_url() {
    GITHUB_REPO="yang1395592280/sub2api"
    local url
    url=$(checksum_url_for_version "v9.9.9")
    assert_eq \
        "https://github.com/yang1395592280/sub2api/releases/download/v9.9.9/checksums.txt" \
        "$url" \
        "checksum URL should point to current fork"
}

test_sha256_file() {
    local tmp_file
    tmp_file=$(mktemp)
    printf 'sub2api-checksum-test' > "$tmp_file"

    local hash
    hash=$(sha256_file "$tmp_file")
    rm -f "$tmp_file"

    if [ -z "$hash" ]; then
        echo "ASSERT FAIL: sha256_file returned empty hash" >&2
        exit 1
    fi
}

main() {
    test_default_repo
    test_override_repo
    test_checksum_url
    test_sha256_file
    echo "install.sh tests passed"
}

main "$@"
