#!/usr/bin/env bash

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
  echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
  echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
  echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
  exit 1
}

if [[ $EUID -ne 0 ]]; then
  error "Please run as root"
fi

REPO_URL="https://github.com/your-org/vpn-master-panel.git"
INSTALL_DIR="/opt/vpn-master"
DATA_DIR="/var/lib/vpn-master"
BIN_PATH="/usr/local/bin/vpn-master"
PORT="8085"
SECRET=""

usage() {
cat <<USAGE
Usage: $0 [options]

Options:
  -r <repo>     Git repository URL (default: ${REPO_URL})
  -d <dir>      Installation directory (default: ${INSTALL_DIR})
  -D <dir>      Data directory for SQLite (default: ${DATA_DIR})
  -p <port>     HTTP port (default: ${PORT})
  -s <secret>   HMAC secret (default: generated)
  -h            Show this help
USAGE
}

while getopts "r:d:D:p:s:h" opt; do
  case ${opt} in
    r) REPO_URL="${OPTARG}" ;;
    d) INSTALL_DIR="${OPTARG}" ;;
    D) DATA_DIR="${OPTARG}" ;;
    p) PORT="${OPTARG}" ;;
    s) SECRET="${OPTARG}" ;;
    h)
      usage
      exit 0
      ;;
    *)
      usage
      exit 1
      ;;
  esac
done

command -v git >/dev/null 2>&1 || error "git is required"
command -v go >/dev/null 2>&1 || error "go toolchain is required"

mkdir -p "${INSTALL_DIR}"
mkdir -p "${DATA_DIR}"

if [[ -d ${INSTALL_DIR}/.git ]]; then
  log "Updating existing repository in ${INSTALL_DIR}"
  git -C "${INSTALL_DIR}" fetch --all --tags
  git -C "${INSTALL_DIR}" reset --hard origin/main
else
  log "Cloning ${REPO_URL} into ${INSTALL_DIR}"
  git clone "${REPO_URL}" "${INSTALL_DIR}"
fi

pushd "${INSTALL_DIR}" >/dev/null

log "Building vpn-master binary"
GOOS=linux GOARCH=$(go env GOARCH) go build -o "${BIN_PATH}" ./cmd/master

popd >/dev/null

if [[ -z "${SECRET}" ]]; then
  SECRET=$(openssl rand -hex 32)
  log "Generated HMAC secret"
fi

ENV_FILE="/etc/vpn-master.env"
cat >"${ENV_FILE}" <<EOF
MASTER_HTTP_PORT=${PORT}
MASTER_DB_DRIVER=sqlite
MASTER_DB_DSN=${DATA_DIR}/master.db
MASTER_DB_AUTO_MIGRATE=true
MASTER_HMAC_SECRET=${SECRET}
EOF

SERVICE_FILE="/etc/systemd/system/vpn-master.service"
cat >"${SERVICE_FILE}" <<EOF
[Unit]
Description=VPN Master Control Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
EnvironmentFile=${ENV_FILE}
ExecStart=${BIN_PATH}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

log "Reloading systemd and starting service"
systemctl daemon-reload
systemctl enable --now vpn-master.service

log "Installation complete"
log "Master panel listening on port ${PORT}"
log "HMAC secret stored in ${ENV_FILE}"
log "SQLite database located at ${DATA_DIR}/master.db"
