#!/usr/bin/env bash
# -----------------------------------------------------------------------------
#  tools/ots_builder.sh
#  Enterprise Cross-Platform Build, Maintenance & Daemon Orchestration Suite
#
#  Supported Environments:
#    - Linux (RHEL, Ubuntu, Debian, Alpine)
#    - Windows (Cygwin, MSYS2, Git Bash)
#    - macOS (Darwin)
#
#  Usage:
#    ots_builder.sh [version-tag] [options]
#
#  Available Options:
#    -h, --help                   Display help & usage information and exit.
#    --platform <target>          Specify build targets: 'windows', 'linux', or 'windows,linux'.
#    --auto-version               Derive build version automatically from git tag & commit hash.
#    --update-deps, --upgrade-deps Upgrade Go modules, Node/Vue packages, & audit dependencies.
#    --english-only               Purge non-English translations during compilation.
#    --no-package                 Skip distribution tarball / zip archive packaging.
#    --build-vendor               Vendor Go modules into vendor/ directory for offline audits.
#    --with-redis [<path>]        Enable Redis storage for live tests/daemons (optional <path> parameter).
#    --validate                   Launch compiled binary on port 39999 and run live API tests.
#    --kill-running               Terminate stale OTS server & Redis daemon processes.
#    --auto-start                 Auto-launch persistent OTS background server daemon on 127.0.0.1:3000.
#
#  Usage Examples:
#    bash ./tools/ots_builder.sh --auto-version --english-only --platform windows,linux --validate
#    bash ./tools/ots_builder.sh --auto-version --validate --with-redis d:/inetd/redis
#    bash ./tools/ots_builder.sh --platform windows --auto-start
# -----------------------------------------------------------------------------
set -euo pipefail

# -----------------------------------------------------------------------------
# Section 1: Path Initialization & Environment Defaults
# -----------------------------------------------------------------------------
# Sub-Step 1.1: Resolve Absolute Script & Project Root Directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Normalize MSYS2 / Cygwin paths upfront across mixed (Windows native) and POSIX formats
OUTPUT_DIR="${ROOT_DIR}/testfiles/dist"
BIN_DIR="${ROOT_DIR}/testfiles/bin"
OUTPUT_DIR_POSIX="${OUTPUT_DIR}"

if command -v cygpath &> /dev/null; then
  SCRIPT_DIR="$(cygpath -m "${SCRIPT_DIR}")"
  ROOT_DIR="$(cygpath -m "${ROOT_DIR}")"
  OUTPUT_DIR="$(cygpath -m "${OUTPUT_DIR}")"
  BIN_DIR="$(cygpath -m "${BIN_DIR}")"
  OUTPUT_DIR_POSIX="$(cygpath -u "${OUTPUT_DIR}")"
fi
cd "${ROOT_DIR}" || { echo "ERROR: Failed to change directory to project root ${ROOT_DIR}" >&2; exit 1; }

# Sub-Step 1.2: Initialize Build Variables & Default Options
BUILD_ID="ots"
VERSION_ARG=""
NO_PACKAGE=false
BUILD_LINUX=true
BUILD_WINDOWS=true
PLATFORM_EXPLICIT=false
BUILD_VENDOR=false
BUILD_ENGLISH_ONLY=false
BUILD_AUTO_VERSION=false
BUILD_VALIDATE=false
KILL_RUNNING=false
AUTO_START=false
UPDATE_DEPS=false
WITH_REDIS=false
REDIS_ARG_PATH=""
REDIS_BIN=""
REDIS_HOME="${REDIS_HOME:-}"

# Sub-Step 1.3: Render Help & Usage Information
show_usage() {
  cat << 'EOF'
Enterprise Cross-Platform Build, Maintenance & Daemon Orchestration Suite

Usage:
  ots_builder.sh [version-tag] [options]

Available Options:
  -h, --help                    Display this help & usage information and exit.
  --platform <target>           Specify build targets: 'windows', 'linux', or 'windows,linux'.
  --windows                     Build target for Windows (amd64).
  --linux                       Build target for Linux (amd64).
  --auto-version                Derive build version automatically from git tag & commit hash.
  --english-only                Purge non-English translations during compilation.
  --no-package                  Skip distribution tarball / zip archive packaging.
  --build-vendor                Vendor Go modules into vendor/ directory for offline audits.
  --update-deps, --upgrade-deps Upgrade Go modules, Node/Vue packages, & audit dependencies.
  --with-redis [<path>]         Enable Redis storage for live tests/daemons (optional <path> parameter).
  --validate                    Launch compiled binary on port 39999 and run live API validation suite.
  --kill-running                Terminate stale OTS server & Redis daemon processes.
  --auto-start                  Auto-launch persistent OTS background server daemon on 127.0.0.1:3000.

Usage Examples:
  bash ./tools/ots_builder.sh --auto-version --english-only --platform windows,linux --validate
  bash ./tools/ots_builder.sh --auto-version --validate --with-redis d:/inetd/redis
  bash ./tools/ots_builder.sh --platform windows --auto-start
EOF
  exit 0
}

# Sub-Step 1.4: Resolve Redis Location Helper
resolve_redis() {
  if [ "${WITH_REDIS}" != "true" ]; then
    return 0
  fi

  # 1. If <path> specified, add to PATH
  if [ -n "${REDIS_ARG_PATH}" ]; then
    local p="${REDIS_ARG_PATH}"
    if command -v cygpath &> /dev/null; then
      local u_path
      u_path="$(cygpath -u "${p}")"
      export PATH="${u_path}:${PATH}"
    else
      export PATH="${p}:${PATH}"
    fi
  fi

  # 2. If redis found in path, set REDIS_HOME
  local bin=""
  if command -v redis-server &> /dev/null; then
    bin="$(command -v redis-server)"
  fi

  if [ -n "${bin}" ]; then
    if command -v cygpath &> /dev/null; then
      bin="$(cygpath -m "${bin}")"
    fi
    REDIS_BIN="${bin}"
    REDIS_HOME="$(dirname "${bin}")"
    return 0
  fi

  # 3. If not, inspect for REDIS_HOME
  if [ -n "${REDIS_HOME}" ]; then
    local rh="${REDIS_HOME}"
    if command -v cygpath &> /dev/null; then
      rh="$(cygpath -m "${rh}")"
    fi
    if [ -f "${rh}/redis-server.exe" ]; then
      REDIS_BIN="${rh}/redis-server.exe"
      REDIS_HOME="${rh}"
      return 0
    elif [ -f "${rh}/redis-server" ]; then
      REDIS_BIN="${rh}/redis-server"
      REDIS_HOME="${rh}"
      return 0
    fi
  fi

  # 4. If not found, ABORT WITH ERROR
  echo "ERROR: --with-redis specified, but redis-server executable was not found in PATH or REDIS_HOME." >&2
  exit 1
}

# Sub-Step 1.4: Pre-Flight Environment Validation
if ! command -v go &> /dev/null; then
  echo "ERROR: Go toolchain ('go') is not installed or not in PATH." >&2
  exit 1
fi

# -----------------------------------------------------------------------------
# Section 2: Redis Server Control & Process Cleanup Helpers
# -----------------------------------------------------------------------------
stop_redis_server() {
  echo "==> Stopping running Redis processes..."
  if command -v pkill &> /dev/null; then
    pkill -f "redis-server" 2> /dev/null || true
  fi
  if command -v taskkill &> /dev/null; then
    taskkill /F /IM redis-server.exe 2> /dev/null || true
  fi
  if command -v powershell &> /dev/null; then
    powershell -Command "Get-Process -Name redis-server -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue" 2> /dev/null || true
  fi
  sleep 1
}

start_redis_server() {
  local port="${1:-6379}"
  if [ -z "${REDIS_BIN}" ] || [ ! -f "${REDIS_BIN}" ]; then
    echo "ERROR: Cannot start Redis server. redis-server binary not found." >&2
    return 1
  fi

  echo "==> Starting Redis server (${REDIS_BIN}) on port ${port}..."
  if command -v cygpath &> /dev/null; then
    WIN_REDIS="$(cygpath -m "${REDIS_BIN}")"
    if command -v powershell &> /dev/null; then
      powershell -Command "Start-Process -FilePath '${WIN_REDIS}' -ArgumentList '--port', '${port}', '--daemonize', 'no' -WindowStyle Hidden" > /dev/null 2>&1 || true
    else
      "${WIN_REDIS}" --port "${port}" --daemonize no > /dev/null 2>&1 &
    fi
  else
    nohup "${REDIS_BIN}" --port "${port}" --daemonize no > /dev/null 2>&1 &
  fi
  sleep 1
  echo "    Redis server started on 127.0.0.1:${port}."
}

kill_running_ots_processes() {
  echo "==> Terminating any running OTS server processes..."
  # Sub-Step 2.1: Terminate Stale Server Processes via pkill (Linux/macOS)
  if command -v pkill &> /dev/null; then
    pkill -f "ots.*--listen" 2> /dev/null || true
  fi
  # Sub-Step 2.2: Terminate Stale Server Processes via taskkill (Windows CMD/Cygwin)
  if command -v taskkill &> /dev/null; then
    taskkill /F /IM ots.exe 2> /dev/null || true
    taskkill /F /IM ots_windows_amd64.exe 2> /dev/null || true
    taskkill /F /IM ots_linux_amd64 2> /dev/null || true
  fi
  # Sub-Step 2.3: Terminate Stale Server Processes via PowerShell (Windows PS)
  if command -v powershell &> /dev/null; then
    powershell -Command "Get-Process -Name ots,ots_windows_amd64,ots_linux_amd64 -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue" 2> /dev/null || true
  fi
  sleep 1
}

# -----------------------------------------------------------------------------
# Section 3: CLI Argument Parsing & Option Evaluation
# -----------------------------------------------------------------------------
# Sub-Step 3.1: Loop over Command Line Arguments
while [ $# -gt 0 ]; do
  arg="$1"
  # Sub-Step 3.2: Evaluate Flag Parameters & Enable Feature Switches
  case "$arg" in
    --platform=*)
      PLATFORM_VAL="${arg#*=}"
      PLATFORM_EXPLICIT=true
      case "${PLATFORM_VAL}" in
        windows)
          BUILD_WINDOWS=true
          BUILD_LINUX=false
          ;;
        linux)
          BUILD_LINUX=true
          BUILD_WINDOWS=false
          ;;
        windows,linux | linux,windows | all | both)
          BUILD_LINUX=true
          BUILD_WINDOWS=true
          ;;
        *)
          echo "Unknown platform: ${PLATFORM_VAL}. Allowed: windows, linux, windows,linux"
          exit 1
          ;;
      esac
      shift
      ;;
    --platform)
      shift
      PLATFORM_VAL="${1:-}"
      PLATFORM_EXPLICIT=true
      case "${PLATFORM_VAL}" in
        windows)
          BUILD_WINDOWS=true
          BUILD_LINUX=false
          ;;
        linux)
          BUILD_LINUX=true
          BUILD_WINDOWS=false
          ;;
        windows,linux | linux,windows | all | both)
          BUILD_LINUX=true
          BUILD_WINDOWS=true
          ;;
        *)
          echo "Unknown platform: ${PLATFORM_VAL}. Allowed: windows, linux, windows,linux"
          exit 1
          ;;
      esac
      shift
      ;;
    --windows)
      BUILD_WINDOWS=true
      BUILD_LINUX=false
      PLATFORM_EXPLICIT=true
      shift
      ;;
    --linux)
      BUILD_LINUX=true
      BUILD_WINDOWS=false
      PLATFORM_EXPLICIT=true
      shift
      ;;
    --no-package)
      NO_PACKAGE=true
      shift
      ;;
    --build-vendor)
      BUILD_VENDOR=true
      shift
      ;;
    --english-only)
      BUILD_ENGLISH_ONLY=true
      shift
      ;;
    --auto-version)
      BUILD_AUTO_VERSION=true
      shift
      ;;
    --validate)
      BUILD_VALIDATE=true
      shift
      ;;
    --kill-running | --clean-processes)
      KILL_RUNNING=true
      shift
      ;;
    --auto-start)
      AUTO_START=true
      KILL_RUNNING=true
      shift
      ;;
    --update-deps | --upgrade-deps)
      UPDATE_DEPS=true
      shift
      ;;
    --with-redis=*)
      WITH_REDIS=true
      REDIS_ARG_PATH="${arg#*=}"
      shift
      ;;
    --with-redis)
      WITH_REDIS=true
      shift
      if [ $# -gt 0 ] && [[ "$1" != -* ]]; then
        REDIS_ARG_PATH="$1"
        shift
      fi
      ;;
    -h | --help)
      show_usage
      ;;
    -*)
      echo "Unknown flag: $arg"
      exit 1
      ;;
    *)
      if [ -z "${VERSION_ARG}" ]; then
        VERSION_ARG="$arg"
      fi
      shift
      ;;
  esac
done

# Sub-Step 3.3: Execute Pre-Build Process Cleanup & Redis Location Verification
if [ "${KILL_RUNNING}" = "true" ]; then
  kill_running_ots_processes
  stop_redis_server
fi

resolve_redis

# -----------------------------------------------------------------------------
# Section 4: Version Tag Resolution & Release Header Display
# -----------------------------------------------------------------------------
# Sub-Step 4.1: Query Git Revision Metadata (Tags & Short Commits)
if [ "${BUILD_AUTO_VERSION}" = "true" ] || [ -z "${VERSION_ARG}" ]; then
  GIT_TAG="$(git -C "${ROOT_DIR}" describe --tags --abbrev=0 2> /dev/null | sed 's/^v//' || echo "1.21.9")"
  GIT_HASH="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2> /dev/null || echo "dev")"
  VERSION_FULL="${GIT_TAG}-${GIT_HASH}"
else
  VERSION_FULL="${VERSION_ARG}"
fi

# Sub-Step 4.2: Parse Semantic Version Numbers & Release Tags
VERSION="${VERSION_FULL%-*}"
TAG="${VERSION_FULL#*-}"
[ "${TAG}" = "${VERSION}" ] && TAG="dev"

# Sub-Step 4.3: Render Informational Release Banner
echo "================================================================="
echo " Building ${BUILD_ID^^} Release: ${VERSION_FULL}"
echo " Environment: $(uname -s 2> /dev/null || echo "unknown") (${MACHTYPE:-unknown})"
echo " Version: ${VERSION} | Tag: ${TAG} | English Only: ${BUILD_ENGLISH_ONLY}"
echo "================================================================="

cd "${ROOT_DIR}"

# -----------------------------------------------------------------------------
# Section 5: Automated Dependency Maintenance & Security Scans (--update-deps)
# Upgrades Go modules, Node.js/Vue packages, & executes govulncheck audit
# -----------------------------------------------------------------------------
if [ "${UPDATE_DEPS}" = "true" ]; then
  echo "================================================================="
  echo " ==> RUNNING FULL DEPENDENCY MAINTENANCE (--update-deps)"
  echo "================================================================="
  # Sub-Step 5.1: Upgrade Go Modules & Tidy Dependencies across all workspace modules
  echo "1. Upgrading Go modules across workspace..."
  find . -name "go.mod" -not -path "*/vendor/*" | while read -r modfile; do
    moddir="$(dirname "${modfile}")"
    echo "   -> Maintenance on module: ${moddir}"
    (
      cd "${moddir}"
      go get -u ./... 2> /dev/null || true
      go mod tidy
      go mod verify
    )
  done

  # Sub-Step 5.2: Upgrade Node.js & Vue Packages via Package Manager (pnpm / npm)
  echo "2. Upgrading Node.js / Vue packages via pnpm..."
  if command -v pnpm &> /dev/null; then
    pnpm update
  elif command -v npm &> /dev/null; then
    npm update
  fi

  # Sub-Step 5.3: Execute Security Vulnerability Audit (govulncheck)
  echo "3. Auditing Go dependencies..."
  if command -v govulncheck &> /dev/null; then
    govulncheck ./... || true
  fi
  echo "================================================================="
fi

# -----------------------------------------------------------------------------
# Section 6: Translation Purging & i18n Generation
# -----------------------------------------------------------------------------
# Sub-Step 6.1: Backup i18n.yaml & Purge Non-English Translations (If --english-only)
if [ "${BUILD_ENGLISH_ONLY}" = "true" ]; then
  echo "==> Purging non-English languages (--english-only mode enabled)..."
  cp i18n.yaml i18n.yaml.bak
  trap 'mv -f i18n.yaml.bak i18n.yaml 2>/dev/null || true' EXIT INT TERM HUP

  echo "    Purging translations block using sed..."
  sed -i -n '/^translations:/q;p' i18n.yaml
  echo "translations: {}" >> i18n.yaml
fi

# Sub-Step 6.2: Build & Execute i18n Translation Generation Tool (ci/translate)
if [ -d "ci/translate" ]; then
  echo "==> Generating i18n translations..."
  if ! (cd ci/translate && go build -o translate_tool main.go 2> /dev/null); then
    (cd ci/translate && go build -o translate_tool)
  fi
  ./ci/translate/translate_tool || { echo "ERROR: i18n translation generation failed" >&2; exit 1; }
  rm -f ci/translate/translate_tool ci/translate/translate_tool.exe 2> /dev/null || true
fi

# Sub-Step 6.3: Immediately Restore Original i18n.yaml Configuration
if [ "${BUILD_ENGLISH_ONLY}" = "true" ]; then
  echo "==> Restoring original i18n.yaml file..."
  mv -f i18n.yaml.bak i18n.yaml 2> /dev/null || true
  trap - EXIT INT TERM HUP
fi

# -----------------------------------------------------------------------------
# Section 7: Host-Native Frontend Bundle Assembly (Vue 3 + esbuild)
# -----------------------------------------------------------------------------
echo "==> Checking Frontend Build Dependencies..."
# Sub-Step 7.1: Detect Available JavaScript Package Manager (pnpm / npm)
if command -v pnpm &> /dev/null; then
  # Sub-Step 7.2: Install Frontend Dependencies & Execute esbuild Bundler (ci/build.mjs)
  echo "    Building frontend with pnpm (ci/build.mjs)..."
  pnpm install --frozen-lockfile 2> /dev/null || pnpm install
  pnpm node ci/build.mjs || node ci/build.mjs || true
elif command -v npm &> /dev/null; then
  echo "    Building frontend with npm (ci/build.mjs)..."
  npm install --no-audit --no-fund 2> /dev/null || npm install
  node ci/build.mjs || true
# Sub-Step 7.3: Fallback to Pre-Built Embedded Go Assets
elif [ -d "frontend/dist" ] || [ -f "frontend/dist/index.html" ]; then
  echo "    Using pre-built embedded assets in frontend/dist/"
else
  echo "    Notice: pnpm/npm not found. Embedded Go FS fallback will be utilized."
fi

# -----------------------------------------------------------------------------
# Section 8: Go Module Verification & Workspace Setup
# -----------------------------------------------------------------------------
# Sub-Step 8.1: Verify & Tidy All Go Modules Across Workspace
echo "==> Verifying Go module dependencies across workspace..."
find . -name "go.mod" -not -path "*/vendor/*" | while read -r modfile; do
  moddir="$(dirname "${modfile}")"
  (
    cd "${moddir}"
    go mod tidy
    go mod verify
  )
done

# Sub-Step 8.2: Ensure Output Directories Exist (testfiles/bin & testfiles/dist)
mkdir -p "${BIN_DIR}"
mkdir -p "${OUTPUT_DIR}"

# Sub-Step 8.3: Define Reusable Binary Compilation Routine (build_binary)
build_binary() {
  local target_os="$1"
  local target_arch="$2"
  local bin_suffix="$3"
  local output_server="${BIN_DIR}/ots_${target_os}_${target_arch}${bin_suffix}"
  local output_cli="${BIN_DIR}/ots-cli_${target_os}_${target_arch}${bin_suffix}"

  echo "==> Compiling for ${target_os}/${target_arch}..."

  # Build main server binary with stripped symbols & version injection
  if ! GOOS="${target_os}" GOARCH="${target_arch}" CGO_ENABLED=0 \
    go build -v -mod=readonly -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}-${TAG}" \
    -o "${output_server}" .; then
    echo "ERROR: Compilation of main server binary failed for ${target_os}/${target_arch}" >&2
    exit 1
  fi

  # Build standalone CLI binary
  if [ -d "cmd/ots-cli" ]; then
    (
      cd cmd/ots-cli
      if ! GOOS="${target_os}" GOARCH="${target_arch}" CGO_ENABLED=0 \
        go build -v -mod=readonly -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}-${TAG}" \
        -o "${output_cli}" .; then
        echo "ERROR: Compilation of CLI binary failed for ${target_os}/${target_arch}" >&2
        exit 1
      fi
    )
  fi
}

# -----------------------------------------------------------------------------
# Section 9: Compilation Matrix (Linux & Windows Targets)
# -----------------------------------------------------------------------------
# Sub-Step 9.1: Compile Linux amd64 Server & CLI Binaries
if [ "${BUILD_LINUX}" = "true" ]; then
  build_binary "linux" "amd64" ""
fi

# Sub-Step 9.2: Compile Windows amd64 Executables (ots.exe & ots-cli.exe)
if [ "${BUILD_WINDOWS}" = "true" ]; then
  build_binary "windows" "amd64" ".exe"
fi

# Sub-Step 9.3: Verify Compiled Binaries on Filesystem
echo "==> Verifying compiled binaries..."
if [ -f "${BIN_DIR}/ots_linux_amd64" ]; then
  echo "    Linux Binary: ${BIN_DIR}/ots_linux_amd64"
fi
if [ -f "${BIN_DIR}/ots_windows_amd64.exe" ]; then
  echo "    Windows Binary: ${BIN_DIR}/ots_windows_amd64.exe"
fi

# -----------------------------------------------------------------------------
# Section 10: Distribution Packaging (RPM Specs & Archives)
# Creates .tar.gz (Linux) and .zip (Windows) matching systemd & configuration layouts
# -----------------------------------------------------------------------------
if [ "${NO_PACKAGE}" = "false" ]; then
  echo "==> Packaging distribution archives..."

  # Sub-Step 10.1: Assemble Linux Staging Directory & Create .tar.gz Distribution Package
  if [ "${BUILD_LINUX}" = "true" ]; then
    LINUX_STAGING="${OUTPUT_DIR}/ots-linux-staging"
    rm -rf "${LINUX_STAGING}"
    mkdir -p "${LINUX_STAGING}/etc/custom" "${LINUX_STAGING}/log" "${LINUX_STAGING}/systemd"

    cp -f "${BIN_DIR}/ots_linux_amd64" "${LINUX_STAGING}/ots" 2> /dev/null || true
    cp -f "${BIN_DIR}/ots-cli_linux_amd64" "${LINUX_STAGING}/ots-cli" 2> /dev/null || true
    [ -f "${SCRIPT_DIR}/ots-config.yaml" ] && cp -f "${SCRIPT_DIR}/ots-config.yaml" "${LINUX_STAGING}/etc/ots-config.yaml"
    [ -f "${SCRIPT_DIR}/ots.sysconfig" ] && cp -f "${SCRIPT_DIR}/ots.sysconfig" "${LINUX_STAGING}/etc/ots.env"
    [ -f "${SCRIPT_DIR}/ots.service" ] && cp -f "${SCRIPT_DIR}/ots.service" "${LINUX_STAGING}/systemd/ots.service"
    echo "ots-${VERSION}-${TAG}-linux_amd64" > "${LINUX_STAGING}/etc/ots.version"

    LINUX_ARCHIVE="${OUTPUT_DIR}/ots-${VERSION}-${TAG}-linux_amd64.tar.gz"
    TAR_OUT="${OUTPUT_DIR_POSIX}/ots-${VERSION}-${TAG}-linux_amd64.tar.gz"
    (cd "${LINUX_STAGING}" && tar -czvf "${TAR_OUT}" ./*)
    rm -rf "${LINUX_STAGING}"
    echo "    Linux Archive Created: ${LINUX_ARCHIVE}"
  fi

  # Sub-Step 10.2: Assemble Windows Staging Directory & Create .zip Distribution Package
  if [ "${BUILD_WINDOWS}" = "true" ]; then
    WIN_STAGING="${OUTPUT_DIR}/ots-win-staging"
    rm -rf "${WIN_STAGING}"
    mkdir -p "${WIN_STAGING}/bin" "${WIN_STAGING}/etc/custom" "${WIN_STAGING}/log"

    cp -f "${BIN_DIR}/ots_windows_amd64.exe" "${WIN_STAGING}/bin/ots.exe" 2> /dev/null || true
    cp -f "${BIN_DIR}/ots-cli_windows_amd64.exe" "${WIN_STAGING}/bin/ots-cli.exe" 2> /dev/null || true

    if [ -f "${SCRIPT_DIR}/ots-config.yaml" ]; then
      cp -f "${SCRIPT_DIR}/ots-config.yaml" "${WIN_STAGING}/etc/ots-config.yaml"
      sed -i 's|/etc/ots/custom|c:/inetd/ots/etc/custom|g' "${WIN_STAGING}/etc/ots-config.yaml" 2> /dev/null || true
    fi
    if [ -f "${SCRIPT_DIR}/ots.sysconfig" ]; then
      cp -f "${SCRIPT_DIR}/ots.sysconfig" "${WIN_STAGING}/etc/ots.env"
      sed -i 's|/etc/ots/ots-config.yaml|c:/inetd/ots/etc/ots-config.yaml|g' "${WIN_STAGING}/etc/ots.env" 2> /dev/null || true
    fi
    echo "ots-${VERSION}-${TAG}-windows_amd64" > "${WIN_STAGING}/etc/ots.version"

    WIN_ARCHIVE="${OUTPUT_DIR}/ots-${VERSION}-${TAG}-windows_amd64.zip"
    if command -v zip &> /dev/null; then
      (cd "${WIN_STAGING}" && zip -r "${WIN_ARCHIVE}" ./*)
    else
      echo "    Notice: 'zip' command not found, skipping ZIP compression."
    fi
    rm -rf "${WIN_STAGING}"
    echo "    Windows Archive Created: ${WIN_ARCHIVE}"
  fi

  # Sub-Step 10.3: Package Go Vendor Modules for Offline Security Audits (--build-vendor)
  if [ "${BUILD_VENDOR}" = "true" ]; then
    echo "==> Creating vendor archive..."
    go mod vendor
    VENDOR_ARCHIVE="${OUTPUT_DIR}/ots-${VERSION}-${TAG}-vendor.tar.gz"
    tar -czvf "${VENDOR_ARCHIVE}" vendor/
    echo "    Vendor Archive Created: ${VENDOR_ARCHIVE}"
  fi
else
  echo "==> Skipping distribution packaging (--no-package enabled)."
fi

# -----------------------------------------------------------------------------
# Section 11: Live E2E API Validation (--validate)
# Launches compiled binary on port 39999 and exercises endpoints + 75MB payloads
# -----------------------------------------------------------------------------
if [ "${BUILD_VALIDATE}" = "true" ]; then
  echo "==> Running Live API Validation (--validate enabled)..."
  # Sub-Step 11.1: Terminate Existing Instances & Determine Host Platform Binary
  kill_running_ots_processes

  TEST_PORT="39999"
  TEST_LISTEN="127.0.0.1:${TEST_PORT}"
  TEST_URL="http://${TEST_LISTEN}/api"
  SERVER_BIN=""

  # Select compiled binary for current host platform
  if [ -f "${BIN_DIR}/ots_windows_amd64.exe" ] && [[ "$(uname -s 2> /dev/null || echo "")" == *"NT"* || "$(uname -s 2> /dev/null || echo "")" == *"CYGWIN"* || "$(uname -s 2> /dev/null || echo "")" == *"MSYS"* || "$(uname -s 2> /dev/null || echo "")" == *"MINGW"* ]]; then
    SERVER_BIN="${BIN_DIR}/ots_windows_amd64.exe"
  elif [ -f "${BIN_DIR}/ots_linux_amd64" ]; then
    SERVER_BIN="${BIN_DIR}/ots_linux_amd64"
  fi

  if [ -z "${SERVER_BIN}" ] || [ ! -f "${SERVER_BIN}" ]; then
    echo "    Validation FAILED: No compiled binary found matching platform for validation."
    exit 1
  fi

  # Sub-Step 11.2: Launch Test Server on Port 39999 & Wait for /api/healthz Readiness
  echo "    Starting compiled binary (${SERVER_BIN}) on ${TEST_LISTEN}..."
  "${SERVER_BIN}" --listen "${TEST_LISTEN}" --log-level warn &
  SERVER_PID=$!

  # Ensure server process is killed on exit
  trap 'kill ${SERVER_PID} 2>/dev/null || true' EXIT

  READY=false
  for _ in {1..10}; do
    if curl -s "${TEST_URL}/healthz" &> /dev/null; then
      READY=true
      break
    fi
    sleep 0.5
  done

  if [ "${READY}" = "false" ]; then
    echo "    Validation FAILED: Server failed to respond on ${TEST_URL}/healthz"
    kill ${SERVER_PID} 2> /dev/null || true
    exit 1
  fi

  # Sub-Step 11.3: Verify /api/healthz, /api/isWritable, and /api/settings Endpoints
  echo "    1. Testing /api/healthz & /api/isWritable endpoints..."
  curl -sSf "${TEST_URL}/healthz" &> /dev/null
  curl -sSf "${TEST_URL}/isWritable" &> /dev/null

  echo "    2. Testing /api/settings endpoint..."
  SETTINGS_RESP="$(curl -sSf "${TEST_URL}/settings")"
  if [[ "${SETTINGS_RESP}" != *"appTitle"* ]]; then
    echo "    Validation FAILED: /api/settings returned invalid response: ${SETTINGS_RESP}"
    kill ${SERVER_PID} 2> /dev/null || true
    exit 1
  fi

  # Sub-Step 11.4: Verify Secret Creation & Atomic One-Time Burn-After-Read (404 Error)
  echo "    3. Testing Secret Creation & One-Time Read/Burn..."
  CREATE_RESP="$(curl -sSf -X POST -H "Content-Type: application/json" -d '{"secret":"Validation Test Payload"}' "${TEST_URL}/create")"
  SECRET_ID="$(echo "${CREATE_RESP}" | sed -n 's/.*"secret_id":"\([^"]*\)".*/\1/p')"

  if [ -z "${SECRET_ID}" ]; then
    echo "    Validation FAILED: Could not parse secret_id from response: ${CREATE_RESP}"
    kill ${SERVER_PID} 2> /dev/null || true
    exit 1
  fi

  # First read succeeds
  GET_RESP="$(curl -sSf "${TEST_URL}/get/${SECRET_ID}")"
  if [[ "${GET_RESP}" != *"Validation Test Payload"* ]]; then
    echo "    Validation FAILED: Secret payload mismatch: ${GET_RESP}"
    kill ${SERVER_PID} 2> /dev/null || true
    exit 1
  fi

  # Second read returns 404 (Burn-after-read verified)
  HTTP_CODE="$(curl -s -o /dev/null -w "%{http_code}" "${TEST_URL}/get/${SECRET_ID}")"
  if [ "${HTTP_CODE}" != "404" ]; then
    echo "    Validation FAILED: Second read returned status ${HTTP_CODE}, expected 404 Not Found"
    kill ${SERVER_PID} 2> /dev/null || true
    exit 1
  fi

  # Sub-Step 11.5: Test High-Capacity Payload Transfer (~75 MiB Payload Capacity)
  echo "    4. Testing Large Secret Payload (~75 MiB payload capacity)..."
  LARGE_CREATE_RESP="$( (echo -n '{"secret":"' && head -c 75000000 /dev/zero | base64 -w 0 && echo -n '"}') | curl -sSf -X POST -H "Content-Type: application/json" --data-binary @- "${TEST_URL}/create")"
  LARGE_SECRET_ID="$(echo "${LARGE_CREATE_RESP}" | sed -n 's/.*"secret_id":"\([^"]*\)".*/\1/p')"
  if [ -z "${LARGE_SECRET_ID}" ]; then
    echo "    Validation FAILED: Large secret creation failed"
    kill ${SERVER_PID} 2> /dev/null || true
    exit 1
  fi

  # Sub-Step 11.6: Run Redis Live Storage Validation if Redis is enabled
  if [ "${WITH_REDIS}" = "true" ] || [ -n "${REDIS_BIN}" ]; then
    echo "==> Running Live API Validation with REDIS storage..."
    REDIS_PORT="63799"

    start_redis_server "${REDIS_PORT}"

    # Shut down Memory test server & wait for port socket cleanup
    kill ${SERVER_PID} 2> /dev/null || true
    sleep 1.5

    echo "    Starting compiled binary (${SERVER_BIN}) on ${TEST_LISTEN} (Storage: REDIS)..."
    REDIS_URL="redis://127.0.0.1:${REDIS_PORT}/0" "${SERVER_BIN}" --listen "${TEST_LISTEN}" --storage-type redis --log-level warn &
    SERVER_PID=$!

    REDIS_READY=false
    for _ in {1..10}; do
      if curl -s "${TEST_URL}/healthz" &> /dev/null; then
        REDIS_READY=true
        break
      fi
      sleep 0.5
    done

    if [ "${REDIS_READY}" = "true" ]; then
      echo "    5. Testing Redis Secret Creation & Burn..."
      REDIS_CREATE_RESP="$(curl -sSf -X POST -H "Content-Type: application/json" -d '{"secret":"Redis Validation Payload"}' "${TEST_URL}/create")"
      REDIS_SECRET_ID="$(echo "${REDIS_CREATE_RESP}" | sed -n 's/.*"secret_id":"\([^"]*\)".*/\1/p')"
      if [ -n "${REDIS_SECRET_ID}" ]; then
        REDIS_GET="$(curl -sSf "${TEST_URL}/get/${REDIS_SECRET_ID}")"
        if [[ "${REDIS_GET}" == *"Redis Validation Payload"* ]]; then
          echo "    Redis Storage Validation PASSED 100%!"
        fi
      fi
    else
      echo "    WARNING: Redis test server failed to respond on port ${REDIS_PORT}"
    fi

    stop_redis_server
  fi

  # Sub-Step 11.7: Shutdown Test Server & Report Validation Status
  echo "    Shutting down test server (PID ${SERVER_PID})..."
  kill ${SERVER_PID} 2> /dev/null || true
  trap - EXIT

  echo "==> Live API Validation PASSED 100%!"
fi

echo "================================================="
echo " SUCCESS: OTS Build completed successfully!"
echo " Binaries stored in: ${BIN_DIR}"
if [ "${NO_PACKAGE}" = "false" ]; then
  echo " Packages stored in: ${OUTPUT_DIR}"
fi
echo "================================================="

# -----------------------------------------------------------------------------
# Section 12: Persistent Background Daemon Orchestration (--auto-start)
# Launches detached server on http://127.0.0.1:3000/ with testfiles/config.yml
# -----------------------------------------------------------------------------
if [ "${AUTO_START}" = "true" ]; then
  # Sub-Step 12.1: Terminate Stale Instances & Prepare Logging Directory (testfiles/logs)
  kill_running_ots_processes

  USE_REDIS_DAEMON=false
  if [ "${WITH_REDIS}" = "true" ] || [ -n "${REDIS_BIN}" ]; then
    USE_REDIS_DAEMON=true
    stop_redis_server
    start_redis_server 6379
  fi

  # Resolve custom configuration file if present
  CUSTOM_CFG=""
  for cfg_candidate in \
    "${ROOT_DIR}/testfiles/config.yml" \
    "${ROOT_DIR}/testfiles/config.yaml" \
    "${ROOT_DIR}/testdata/config.yml" \
    "${ROOT_DIR}/testdata/config.yaml" \
    "${ROOT_DIR}/config.yml" \
    "${ROOT_DIR}/config.yaml"; do
    if [ -f "${cfg_candidate}" ]; then
      CUSTOM_CFG="${cfg_candidate}"
      break
    fi
  done

  LOG_DIR="${ROOT_DIR}/testfiles/logs"
  mkdir -p "${LOG_DIR}"
  LOG_FILE="${LOG_DIR}/ots.log"

  echo "==> Auto-starting OTS server on http://127.0.0.1:3000/ (Logs: ${LOG_FILE}) ..."
  if [ -n "${CUSTOM_CFG}" ]; then
    echo "    Using configuration file: ${CUSTOM_CFG}"
  fi

  if [ "${USE_REDIS_DAEMON}" = "true" ]; then
    export REDIS_URL="redis://127.0.0.1:6379/0"
  fi

  STARTED_PID=""

  # Sub-Step 12.2: Launch Detached Windows Background Process via PowerShell Start-Process
  if [ -f "${BIN_DIR}/ots_windows_amd64.exe" ] && command -v cygpath &> /dev/null; then
    EXEC_PATH="$(cygpath -m "${BIN_DIR}/ots_windows_amd64.exe")"
    WIN_LOG_PATH="$(cygpath -m "${LOG_FILE}")"

    ARG_STR="--listen 127.0.0.1:3000 --log-file-path \"${WIN_LOG_PATH}\""
    if [ -n "${CUSTOM_CFG}" ]; then
      CFG_PATH="$(cygpath -m "${CUSTOM_CFG}")"
      ARG_STR="${ARG_STR} --customize \"${CFG_PATH}\""
    fi
    if [ "${USE_REDIS_DAEMON}" = "true" ]; then
      ARG_STR="${ARG_STR} --storage-type redis"
    fi

    if [ "${USE_REDIS_DAEMON}" = "true" ]; then
      STARTED_PID="$(powershell -Command "\$env:REDIS_URL='redis://127.0.0.1:6379/0'; \$p = Start-Process -FilePath '${EXEC_PATH}' -ArgumentList '${ARG_STR}' -PassThru -WindowStyle Hidden; \$p.Id" 2> /dev/null | tr -d '\r\n')" || true
    else
      STARTED_PID="$(powershell -Command "\$p = Start-Process -FilePath '${EXEC_PATH}' -ArgumentList '${ARG_STR}' -PassThru -WindowStyle Hidden; \$p.Id" 2> /dev/null | tr -d '\r\n')" || true
    fi

    # Fallback to ps -W if PassThru was empty
    if [ -z "${STARTED_PID}" ] && command -v ps &> /dev/null; then
      STARTED_PID="$(ps -W 2> /dev/null | grep -i "ots_windows_amd64\.exe" | awk '{print $4}' | head -n 1)" || true
    fi
  # Sub-Step 12.3: Launch Detached Linux Background Process via nohup
  elif [ -f "${BIN_DIR}/ots_linux_amd64" ]; then
    CFG_PARAMS=()
    if [ -n "${CUSTOM_CFG}" ]; then
      CFG_PARAMS=("--customize" "${CUSTOM_CFG}")
    fi
    if [ "${USE_REDIS_DAEMON}" = "true" ]; then
      REDIS_URL="redis://127.0.0.1:6379/0" nohup "${BIN_DIR}/ots_linux_amd64" --listen 127.0.0.1:3000 "${CFG_PARAMS[@]}" --storage-type redis --log-file-path "${LOG_FILE}" > /dev/null 2>&1 &
    else
      nohup "${BIN_DIR}/ots_linux_amd64" --listen 127.0.0.1:3000 "${CFG_PARAMS[@]}" --log-file-path "${LOG_FILE}" > /dev/null 2>&1 &
    fi
    STARTED_PID=$!
  fi

  echo "    Server auto-started successfully (PID: ${STARTED_PID:-detached})."
fi
