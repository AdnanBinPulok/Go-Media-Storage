#!/bin/bash
# pm2_start.sh — build + pm2 start (requires Go 1.25.1+)
#
# System Go < 1.21 (e.g. Ubuntu apt go1.18) cannot parse this repo's go.mod.
# In that case we bootstrap the official Go 1.25.1 toolchain into ~/.cache.

REQUIRED_GO="1.25.1"
MIN_TOOLCHAIN_AUTO="1.21"
CACHE_ROOT="${HOME}/.cache/go-media-storage"
BOOTSTRAP_DIR="${CACHE_ROOT}/go${REQUIRED_GO}"

export GOTOOLCHAIN=auto

go_major_minor() {
    # "go version go1.18.1 linux/amd64" -> "1.18"
    go version 2>/dev/null | sed -n 's/.*go\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/p'
}

version_lt() {
    # true if $1 < $2 (e.g. 1.18 < 1.21)
    [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$1" ] && [ "$1" != "$2" ]
}

detect_go_os_arch() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)
            printf "\nError: unsupported architecture '%s'.\n" "$(uname -m)"
            printf "Install Go %s+ manually from https://go.dev/dl/\n\n" "${REQUIRED_GO}"
            exit 1
            ;;
    esac
    if [ "${os}" != "linux" ] && [ "${os}" != "darwin" ]; then
        printf "\nError: unsupported OS '%s' for automatic Go bootstrap.\n" "${os}"
        printf "Install Go %s+ manually from https://go.dev/dl/\n\n" "${REQUIRED_GO}"
        exit 1
    fi
    echo "${os}" "${arch}"
}

download_file() {
    local url="$1"
    local out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "${out}" "${url}"
        return $?
    fi
    if command -v wget >/dev/null 2>&1; then
        wget -q -O "${out}" "${url}"
        return $?
    fi
    printf "\nError: need curl or wget to download Go %s.\n\n" "${REQUIRED_GO}"
    return 1
}

bootstrap_go() {
    local os arch tarball url tmpdir
    read -r os arch <<EOF
$(detect_go_os_arch)
EOF

    if [ -x "${BOOTSTRAP_DIR}/bin/go" ]; then
        printf "Using cached Go %s at %s\n" "${REQUIRED_GO}" "${BOOTSTRAP_DIR}"
        export PATH="${BOOTSTRAP_DIR}/bin:${PATH}"
        return 0
    fi

    tarball="go${REQUIRED_GO}.${os}-${arch}.tar.gz"
    url="https://go.dev/dl/${tarball}"

    printf "\nSystem Go is missing or too old to parse go.mod (need >= %s for GOTOOLCHAIN=auto).\n" "${MIN_TOOLCHAIN_AUTO}"
    printf "Bootstrapping official Go %s from %s ...\n\n" "${REQUIRED_GO}" "${url}"

    mkdir -p "${CACHE_ROOT}" || {
        printf "\nError: cannot create cache dir %s\n\n" "${CACHE_ROOT}"
        exit 1
    }

    tmpdir="$(mktemp -d "${CACHE_ROOT}/go-bootstrap.XXXXXX")" || {
        printf "\nError: cannot create temp dir for Go bootstrap.\n\n"
        exit 1
    }

    if ! download_file "${url}" "${tmpdir}/${tarball}"; then
        printf "\nError: failed to download Go %s.\n" "${REQUIRED_GO}"
        printf "Air-gapped or blocked hosts must install Go %s+ manually: https://go.dev/dl/\n\n" "${REQUIRED_GO}"
        rm -rf "${tmpdir}"
        exit 1
    fi

    if ! tar -C "${tmpdir}" -xzf "${tmpdir}/${tarball}"; then
        printf "\nError: failed to extract Go tarball.\n\n"
        rm -rf "${tmpdir}"
        exit 1
    fi

    rm -rf "${BOOTSTRAP_DIR}"
    if ! mv "${tmpdir}/go" "${BOOTSTRAP_DIR}"; then
        printf "\nError: failed to install Go into %s\n\n" "${BOOTSTRAP_DIR}"
        rm -rf "${tmpdir}"
        exit 1
    fi
    rm -rf "${tmpdir}"

    if [ ! -x "${BOOTSTRAP_DIR}/bin/go" ]; then
        printf "\nError: bootstrap incomplete; %s/bin/go missing.\n\n" "${BOOTSTRAP_DIR}"
        exit 1
    fi

    export PATH="${BOOTSTRAP_DIR}/bin:${PATH}"
    printf "Go %s installed to %s\n" "${REQUIRED_GO}" "${BOOTSTRAP_DIR}"
}

# --- ensure a usable Go ---
need_bootstrap=0
if ! command -v go >/dev/null 2>&1; then
    need_bootstrap=1
else
    current_mm="$(go_major_minor)"
    if [ -z "${current_mm}" ] || version_lt "${current_mm}" "${MIN_TOOLCHAIN_AUTO}"; then
        need_bootstrap=1
    fi
fi

if [ "${need_bootstrap}" -eq 1 ]; then
    bootstrap_go
fi

printf "\nGo toolchain: %s\n" "$(go version)"
printf "Minimum required: Go %s (GOTOOLCHAIN=auto when system Go >= %s).\n\n" "${REQUIRED_GO}" "${MIN_TOOLCHAIN_AUTO}"

# Download Go modules
if [ -f "go.mod" ]; then
    go mod download
    if [ $? -ne 0 ]; then
        printf "\n\nGo module download failed. Exiting.\n\n"
        exit 1
    fi
    printf "\n\nGo modules downloaded successfully.\n\n"
fi

# Build the application
printf "\n\nBuilding the application...\n\n"
# Windows users might need .exe extension if running in Git Bash, but for standard linux/wsl usage:
go build -o go-media-storage main.go

if [ $? -ne 0 ]; then
    printf "\n\nBuild failed. Exiting.\n\n"
    printf "This project requires Go %s or newer. Air-gapped hosts must install that version locally.\n\n" "${REQUIRED_GO}"
    exit 1
fi
printf "\n\nBuild successful.\n\n"

printf "\n\nStarting the application using pm2...\n\n"
# Start with pm2
pm2 start ./go-media-storage --name="GO_MEDIA_STORAGE" & pm2 logs "GO_MEDIA_STORAGE" --lines 100
printf "\n\nApplication started successfully with pm2.\n\n"
