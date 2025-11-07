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

MASTER_URL=""
NODE_NAME=""
XRAY_VERSION="latest"
LISTEN_ADDR=":8080"
INSTALL_PATH="/usr/local/bin"
REG_SECRET=""

while getopts "u:n:v:l:r:p:" opt; do
	case ${opt} in
		u) MASTER_URL="${OPTARG}" ;;
		n) NODE_NAME="${OPTARG}" ;;
		v) XRAY_VERSION="${OPTARG}" ;;
		l) LISTEN_ADDR="${OPTARG}" ;;
		r) REG_SECRET="${OPTARG}" ;;
		p) INSTALL_PATH="${OPTARG}" ;;
		*) error "Invalid option: -${opt}" ;;
	esac
done

if [[ -z "${MASTER_URL}" ]]; then
	error "Master URL is required (-u)"
fi

if [[ -z "${REG_SECRET}" ]]; then
	error "Registration secret is required (-r)"
fi

if [[ -z "${NODE_NAME}" ]]; then
	NODE_NAME=$(hostname)
fi

NODE_ID="${NODE_NAME// /-}-$(openssl rand -hex 4)"
SECRET_KEY=$(openssl rand -hex 32)

log "Starting Node Agent installation"
log "Master URL: ${MASTER_URL}"
log "Node Name: ${NODE_NAME}"
log "Listen Addr: ${LISTEN_ADDR}"
log "Install Path: ${INSTALL_PATH}"

ARCH=$(uname -m)
case ${ARCH} in
	x86_64) ARCH_SUFFIX="linux-amd64" ;;
	aarch64) ARCH_SUFFIX="linux-arm64" ;;
	armv7l) ARCH_SUFFIX="linux-armv7" ;;
	*) error "Unsupported architecture: ${ARCH}" ;;
esac

log "Downloading Node Agent binary (${ARCH_SUFFIX})"
BIN_URL="https://github.com/your-org/vpn-node-agent/releases/latest/download/node-agent-${ARCH_SUFFIX}"
curl -fsSL "${BIN_URL}" -o "${INSTALL_PATH}/node-agent"
chmod +x "${INSTALL_PATH}/node-agent"

mkdir -p /etc/node-agent

cat > /etc/node-agent/config.json <<EOF
{
  "master_url": "${MASTER_URL}",
  "node_id": "${NODE_ID}",
  "node_name": "${NODE_NAME}",
  "registration_secret": "${REG_SECRET}",
  "secret_key": "${SECRET_KEY}",
  "xray_version": "${XRAY_VERSION}",
  "install_path": "${INSTALL_PATH}",
  "listen_addr": "${LISTEN_ADDR}",
  "log_level": "info"
}
EOF

cat > /etc/systemd/system/node-agent.service <<EOF
[Unit]
Description=VPN Node Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=${INSTALL_PATH}/node-agent -config /etc/node-agent/config.json
Restart=always
RestartSec=5
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now node-agent

log "Node Agent installed successfully"
log "Node ID: ${NODE_ID}"
log "Initial secret key: ${SECRET_KEY}"
log "Check status: systemctl status node-agent"
log "Follow logs: journalctl -u node-agent -f"

