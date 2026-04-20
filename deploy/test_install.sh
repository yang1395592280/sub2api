#!/bin/bash

set -euo pipefail

SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/install.sh"

assert_contains() {
    local pattern="$1"
    local description="$2"
    if ! grep -Fq "$pattern" "$SCRIPT_PATH"; then
        echo "ASSERT FAIL: ${description}" >&2
        echo "  missing pattern: ${pattern}" >&2
        exit 1
    fi
}

main() {
    assert_contains 'GITHUB_REPO="${GITHUB_REPO:-yang1395592280/sub2api}"' \
        "default repository should point to current fork"
    assert_contains 'print_error "This installer currently supports Linux servers with systemd only."' \
        "installer should reject non-Linux systems explicitly"
    assert_contains 'detect_sha256_command()' \
        "installer should provide sha256 command detection"
    assert_contains 'sha256_file()' \
        "installer should use a sha256 helper"
    assert_contains 'if ! command -v systemctl &> /dev/null; then' \
        "installer should require systemctl"
    assert_contains 'if ! command -v useradd &> /dev/null; then' \
        "installer should require useradd"
    assert_contains 'curl -fsSL "$download_url" -o "$TEMP_DIR/$archive_name"' \
        "installer should fail fast when release download fails"
    assert_contains 'print_warning "$(msg '\''checksum_not_found'\'')"' \
        "installer should only warn when checksum file or entry is missing"
    assert_contains 'if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then' \
        "installer should be source-safe for future tests"
    echo "install.sh text checks passed"
}

main "$@"
