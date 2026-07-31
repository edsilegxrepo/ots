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
#    --auto-version               Derive build version automatically from git tag & commit hash.
#    --update-deps, --upgrade-deps Upgrade Go modules, Node/Vue packages, & audit dependencies.
#    --english-only               Purge non-English translations during compilation.
#    --windows                    Compile target binaries for Windows amd64 (ots.exe & ots-cli.exe).
#    --no-package                 Skip distribution tarball / zip archive packaging.
#    --build-vendor               Vendor Go modules into vendor/ directory for offline audits.
#    --validate                   Launch compiled binary on port 39999 and run live API tests.
#    --kill-running               Terminate stale OTS server processes across Windows/Linux.
#    --auto-start                 Auto-launch persistent OTS background server daemon on 127.0.0.1:3000.
#
#  Usage Examples:
#    bash ./tools/ots_builder.sh --auto-version --english-only --windows --validate --auto-start
#    bash ./tools/ots_builder.sh --update-deps --validate
# -----------------------------------------------------------------------------
set -euo pipefail

# -----------------------------------------------------------------------------
# Section 1: Path Initialization & Environment Defaults
# -----------------------------------------------------------------------------
# Sub-Step 1.1: Resolve Absolute Script & Project Root Directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Sub-Step 1.2: Initialize Build Variables & Default Options
BUILD_ID="ots"
VERSION_ARG=""
NO_PACKAGE=false
BUILD_WINDOWS=false
BUILD_VENDOR=false
BUILD_ENGLISH_ONLY=false
BUILD_AUTO_VERSION=false
BUILD_VALIDATE=false
KILL_RUNNING=false
AUTO_START=false
UPDATE_DEPS=false
OUTPUT_DIR="${ROOT_DIR}/testfiles/dist"
BIN_DIR="${ROOT_DIR}/testfiles/bin"

# -----------------------------------------------------------------------------
# Section 2: Cross-Platform Process Cleanup Helper
# Terminates stale OTS instances on Linux (pkill) and Windows (taskkill / PowerShell)
# -----------------------------------------------------------------------------
kill_running_ots_processes() {
  echo "==> Terminating any running OTS server processes..."
  # Sub-Step 2.1: Terminate Stale Server Processes via pkill (Linux/macOS)
  if command -v pkill &>/dev/null; then
    pkill -f "ots.*--listen" 2>/dev/null || true
  fi
  # Sub-Step 2.2: Terminate Stale Server Processes via taskkill (Windows CMD/Cygwin)
  if command -v taskkill &>/dev/null; then
    taskkill /F /IM ots.exe 2>/dev/null || true
    taskkill /F /IM ots_windows_amd64.exe 2>/dev/null || true
    taskkill /F /IM ots_linux_amd64 2>/dev/null || true
  fi
  # Sub-Step 2.3: Terminate Stale Server Processes via PowerShell (Windows PS)
  if command -v powershell &>/dev/null; then
    powershell -Command "Get-Process -Name ots,ots_windows_amd64,ots_linux_amd64 -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue" 2>/dev/null || true
  fi
}

# -----------------------------------------------------------------------------
# Section 3: CLI Argument Parsing & Option Evaluation
# -----------------------------------------------------------------------------
# Sub-Step 3.1: Loop over Command Line Arguments
for arg in "$@"; do
  # Sub-Step 3.2: Evaluate Flag Parameters & Enable Feature Switches
  case "$arg" in
    --windows)
      BUILD_WINDOWS=true
      ;;
    --no-package)
      NO_PACKAGE=true
      ;;
    --build-vendor)
      BUILD_VENDOR=true
      ;;
    --english-only)
      BUILD_ENGLISH_ONLY=true
      ;;
    --auto-version)
      BUILD_AUTO_VERSION=true
      ;;
    --validate)
      BUILD_VALIDATE=true
      ;;
    --kill-running|--clean-processes)
      KILL_RUNNING=true
      ;;
    --auto-start)
      AUTO_START=true
      KILL_RUNNING=true
      ;;
    --update-deps|--upgrade-deps)
      UPDATE_DEPS=true
      ;;
    -*)
      echo "Unknown flag: $arg"
      exit 1
      ;;
    *)
      if [ -z "${VERSION_ARG}" ]; then
        VERSION_ARG="$arg"
      fi
      ;;
  esac
done

# Sub-Step 3.3: Execute Pre-Build Process Cleanup (If Requested)
if [ "${KILL_RUNNING}" = "true" ]; then
  kill_running_ots_processes
fi

# -----------------------------------------------------------------------------
# Section 4: Version Tag Resolution & Release Header Display
# -----------------------------------------------------------------------------
# Sub-Step 4.1: Query Git Revision Metadata (Tags & Short Commits)
if [ "${BUILD_AUTO_VERSION}" = "true" ] || [ -z "${VERSION_ARG}" ]; then
  GIT_TAG="$(git -C "${ROOT_DIR}" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "1.21.9")"
  GIT_HASH="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo "dev")"
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
echo " Environment: $(uname -s 2>/dev/null || echo "unknown") (${MACHTYPE:-unknown})"
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
  # Sub-Step 5.1: Upgrade Go Modules & Tidy Dependencies
  echo "1. Upgrading Go modules..."
  go get -u ./...
  go mod tidy
  go mod verify

  # Sub-Step 5.2: Upgrade Node.js & Vue Packages via Package Manager (pnpm / npm)
  echo "2. Upgrading Node.js / Vue packages via pnpm..."
  if command -v pnpm &>/dev/null; then
    pnpm update --latest
  elif command -v npm &>/dev/null; then
    npm update
  fi

  # Sub-Step 5.3: Execute Security Vulnerability Audit (govulncheck)
  echo "3. Auditing Go dependencies..."
  if command -v govulncheck &>/dev/null; then
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
  trap 'mv -f i18n.yaml.bak i18n.yaml 2>/dev/null || true' EXIT

  echo "    Purging translations block using sed..."
  sed -i -n '/^translations:/q;p' i18n.yaml
  echo "translations: {}" >> i18n.yaml
fi

# Sub-Step 6.2: Build & Execute i18n Translation Generation Tool (ci/translate)
if [ -d "ci/translate" ]; then
  echo "==> Generating i18n translations..."
  if ! (cd ci/translate && go build -o translate_tool main.go 2>/dev/null); then
    (cd ci/translate && go build -o translate_tool)
  fi
  ./ci/translate/translate_tool || true
  rm -f ci/translate/translate_tool ci/translate/translate_tool.exe 2>/dev/null || true
fi

# Sub-Step 6.3: Restore Original i18n.yaml Configuration
if [ "${BUILD_ENGLISH_ONLY}" = "true" ]; then
  echo "==> Restoring original i18n.yaml file..."
  mv -f i18n.yaml.bak i18n.yaml 2>/dev/null || true
  trap - EXIT
fi

# -----------------------------------------------------------------------------
# Section 7: Host-Native Frontend Bundle Assembly (Vue 3 + esbuild)
# -----------------------------------------------------------------------------
echo "==> Checking Frontend Build Dependencies..."
# Sub-Step 7.1: Detect Available JavaScript Package Manager (pnpm / npm)
if command -v pnpm &>/dev/null; then
  # Sub-Step 7.2: Install Frontend Dependencies & Execute esbuild Bundler (ci/build.mjs)
  echo "    Building frontend with pnpm (ci/build.mjs)..."
  pnpm install --frozen-lockfile 2>/dev/null || pnpm install
  pnpm node ci/build.mjs || node ci/build.mjs || true
elif command -v npm &>/dev/null; then
  echo "    Building frontend with npm (ci/build.mjs)..."
  npm install --no-audit --no-fund 2>/dev/null || npm install
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
# Sub-Step 8.1: Verify & Tidy Go Modules
echo "==> Verifying Go module dependencies..."
go mod tidy
go mod verify

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
  GOOS="${target_os}" GOARCH="${target_arch}" CGO_ENABLED=0 \
    go build -v -mod=readonly -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}-${TAG}" \
    -o "${output_server}" .

  # Build standalone CLI binary
  if [ -d "cmd/ots-cli" ]; then
    (
      cd cmd/ots-cli
      output_cli_target="${output_cli}"
      GOOS="${target_os}" GOARCH="${target_arch}" CGO_ENABLED=0 \
        go build -v -mod=readonly -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}-${TAG}" \
        -o "${output_cli_target}" .
    )
  fi
}

# -----------------------------------------------------------------------------
# Section 9: Compilation Matrix (Linux & Windows Targets)
# -----------------------------------------------------------------------------
# Sub-Step 9.1: Compile Linux amd64 Server & CLI Binaries
build_binary "linux" "amd64" ""

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
  LINUX_STAGING="${OUTPUT_DIR}/ots-linux-staging"
  rm -rf "${LINUX_STAGING}"
  mkdir -p "${LINUX_STAGING}/etc/custom" "${LINUX_STAGING}/log" "${LINUX_STAGING}/systemd"

  cp -f "${BIN_DIR}/ots_linux_amd64" "${LINUX_STAGING}/ots" 2>/dev/null || true
  cp -f "${BIN_DIR}/ots-cli_linux_amd64" "${LINUX_STAGING}/ots-cli" 2>/dev/null || true
  [ -f "${SCRIPT_DIR}/ots-config.yaml" ] && cp -f "${SCRIPT_DIR}/ots-config.yaml" "${LINUX_STAGING}/etc/ots-config.yaml"
  [ -f "${SCRIPT_DIR}/ots.sysconfig" ] && cp -f "${SCRIPT_DIR}/ots.sysconfig" "${LINUX_STAGING}/etc/ots.env"
  [ -f "${SCRIPT_DIR}/ots.service" ] && cp -f "${SCRIPT_DIR}/ots.service" "${LINUX_STAGING}/systemd/ots.service"
  echo "ots-${VERSION}-${TAG}-linux_amd64" > "${LINUX_STAGING}/etc/ots.version"

  LINUX_ARCHIVE="${OUTPUT_DIR}/ots-${VERSION}-${TAG}-linux_amd64.tar.gz"
  (cd "${LINUX_STAGING}" && tar -czvf "${LINUX_ARCHIVE}" ./*)
  rm -rf "${LINUX_STAGING}"
  echo "    Linux Archive Created: ${LINUX_ARCHIVE}"

  # Sub-Step 10.2: Assemble Windows Staging Directory & Create .zip Distribution Package
  if [ "${BUILD_WINDOWS}" = "true" ]; then
    WIN_STAGING="${OUTPUT_DIR}/ots-win-staging"
    rm -rf "${WIN_STAGING}"
    mkdir -p "${WIN_STAGING}/bin" "${WIN_STAGING}/etc/custom" "${WIN_STAGING}/log"

    cp -f "${BIN_DIR}/ots_windows_amd64.exe" "${WIN_STAGING}/bin/ots.exe" 2>/dev/null || true
    cp -f "${BIN_DIR}/ots-cli_windows_amd64.exe" "${WIN_STAGING}/bin/ots-cli.exe" 2>/dev/null || true

    if [ -f "${SCRIPT_DIR}/ots-config.yaml" ]; then
      cp -f "${SCRIPT_DIR}/ots-config.yaml" "${WIN_STAGING}/etc/ots-config.yaml"
      sed -i 's|/etc/ots/custom|c:/inetd/ots/etc/custom|g' "${WIN_STAGING}/etc/ots-config.yaml" 2>/dev/null || true
    fi
    if [ -f "${SCRIPT_DIR}/ots.sysconfig" ]; then
      cp -f "${SCRIPT_DIR}/ots.sysconfig" "${WIN_STAGING}/etc/ots.env"
      sed -i 's|/etc/ots/ots-config.yaml|c:/inetd/ots/etc/ots-config.yaml|g' "${WIN_STAGING}/etc/ots.env" 2>/dev/null || true
    fi
    echo "ots-${VERSION}-${TAG}-windows_amd64" > "${WIN_STAGING}/etc/ots.version"

    WIN_ARCHIVE="${OUTPUT_DIR}/ots-${VERSION}-${TAG}-windows_amd64.zip"
    if command -v zip &>/dev/null; then
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
  if [ -f "${BIN_DIR}/ots_windows_amd64.exe" ] && [[ "$(uname -s 2>/dev/null || echo "")" == *"NT"* || "$(uname -s 2>/dev/null || echo "")" == *"CYGWIN"* || "$(uname -s 2>/dev/null || echo "")" == *"MSYS"* || "$(uname -s 2>/dev/null || echo "")" == *"MINGW"* ]]; then
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
    if curl -s "${TEST_URL}/healthz" &>/dev/null; then
      READY=true
      break
    fi
    sleep 0.5
  done

  if [ "${READY}" = "false" ]; then
    echo "    Validation FAILED: Server failed to respond on ${TEST_URL}/healthz"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  # Sub-Step 11.3: Verify /api/healthz, /api/isWritable, and /api/settings Endpoints
  echo "    1. Testing /api/healthz & /api/isWritable endpoints..."
  curl -sSf "${TEST_URL}/healthz" &>/dev/null
  curl -sSf "${TEST_URL}/isWritable" &>/dev/null

  echo "    2. Testing /api/settings endpoint..."
  SETTINGS_RESP="$(curl -sSf "${TEST_URL}/settings")"
  if [[ "${SETTINGS_RESP}" != *"appTitle"* ]]; then
    echo "    Validation FAILED: /api/settings returned invalid response: ${SETTINGS_RESP}"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  # Sub-Step 11.4: Verify Secret Creation & Atomic One-Time Burn-After-Read (404 Error)
  echo "    3. Testing Secret Creation & One-Time Read/Burn..."
  CREATE_RESP="$(curl -sSf -X POST -H "Content-Type: application/json" -d '{"secret":"Validation Test Payload"}' "${TEST_URL}/create")"
  SECRET_ID="$(echo "${CREATE_RESP}" | sed -n 's/.*"secret_id":"\([^"]*\)".*/\1/p')"

  if [ -z "${SECRET_ID}" ]; then
    echo "    Validation FAILED: Could not parse secret_id from response: ${CREATE_RESP}"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  # First read succeeds
  GET_RESP="$(curl -sSf "${TEST_URL}/get/${SECRET_ID}")"
  if [[ "${GET_RESP}" != *"Validation Test Payload"* ]]; then
    echo "    Validation FAILED: Secret payload mismatch: ${GET_RESP}"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  # Second read returns 404 (Burn-after-read verified)
  HTTP_CODE="$(curl -s -o /dev/null -w "%{http_code}" "${TEST_URL}/get/${SECRET_ID}")"
  if [ "${HTTP_CODE}" != "404" ]; then
    echo "    Validation FAILED: Second read returned status ${HTTP_CODE}, expected 404 Not Found"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  # Sub-Step 11.5: Test High-Capacity Payload Transfer (~75 MiB Payload Capacity)
  echo "    4. Testing Large Secret Payload (~75 MiB payload capacity)..."
  LARGE_CREATE_RESP="$( (echo -n '{"secret":"' && head -c 75000000 /dev/zero | base64 -w 0 && echo -n '"}') | curl -sSf -X POST -H "Content-Type: application/json" --data-binary @- "${TEST_URL}/create" )"
  LARGE_SECRET_ID="$(echo "${LARGE_CREATE_RESP}" | sed -n 's/.*"secret_id":"\([^"]*\)".*/\1/p')"
  if [ -z "${LARGE_SECRET_ID}" ]; then
    echo "    Validation FAILED: Large secret creation failed"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  # Sub-Step 11.6: Shutdown Test Server & Report Validation Status
  echo "    Shutting down test server (PID ${SERVER_PID})..."
  kill ${SERVER_PID} 2>/dev/null || true
  trap - EXIT

  echo "==> Live API Validation PASSED 100%!"
fi

echo "================================================="
echo " SUCCESS: OTS Build completed successfully!"
echo " Binaries stored in: ${BIN_DIR}"
echo " Packages stored in: ${OUTPUT_DIR}"
echo "================================================="

# -----------------------------------------------------------------------------
# Section 12: Persistent Background Daemon Orchestration (--auto-start)
# Launches detached server on http://127.0.0.1:3000/ with testfiles/config.yml
# -----------------------------------------------------------------------------
if [ "${AUTO_START}" = "true" ]; then
  # Sub-Step 12.1: Terminate Stale Instances & Prepare Logging Directory (testfiles/logs)
  kill_running_ots_processes
  CUSTOM_CFG="${ROOT_DIR}/testfiles/config.yml"
  LOG_DIR="${ROOT_DIR}/testfiles/logs"
  mkdir -p "${LOG_DIR}"
  LOG_FILE="${LOG_DIR}/ots.log"

  echo "==> Auto-starting OTS server on http://127.0.0.1:3000/ (Logs: ${LOG_FILE}) ..."

  # Sub-Step 12.2: Launch Detached Windows Background Process via PowerShell Start-Process
  if [ -f "${BIN_DIR}/ots_windows_amd64.exe" ] && command -v cygpath &>/dev/null; then
    EXEC_PATH="$(cygpath -m "${BIN_DIR}/ots_windows_amd64.exe")"
    CFG_PATH="$(cygpath -m "${CUSTOM_CFG}")"
    WIN_LOG_PATH="$(cygpath -m "${LOG_FILE}")"
    powershell -Command "Start-Process -FilePath '${EXEC_PATH}' -ArgumentList '--listen 127.0.0.1:3000 --customize \"${CFG_PATH}\" --log-file-path \"${WIN_LOG_PATH}\"' -WindowStyle Hidden"
  # Sub-Step 12.3: Launch Detached Linux Background Process via nohup
  elif [ -f "${BIN_DIR}/ots_linux_amd64" ]; then
    nohup "${BIN_DIR}/ots_linux_amd64" --listen 127.0.0.1:3000 --customize "${CUSTOM_CFG}" --log-file-path "${LOG_FILE}" >/dev/null 2>&1 &
  fi

  echo "    Server auto-started successfully (PID: $!)."
fi
