#!/usr/bin/env bash
# JobPilot AI — one-shot installer.
# Usage:  curl -fsSL https://raw.githubusercontent.com/XtraveNation/Projct-A-A/main/install.sh | bash
#   or:   ./install.sh
set -euo pipefail

REPO="${REPO:-https://github.com/XtraveNation/Projct-A-A.git}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/jobpilot}"
PORT="${PORT:-8000}"
ADMIN_EMAIL="${ADMIN_EMAIL:-}"

say(){ printf "\n\033[1;36m▸ %s\033[0m\n" "$*"; }
need(){ command -v "$1" >/dev/null 2>&1 || { echo "missing: $1"; exit 1; }; }

say "Checking prerequisites"
need git
if ! command -v go >/dev/null 2>&1; then
  say "Installing Go 1.23"
  GOVER=1.23.4
  ARCH=$(uname -m); case "$ARCH" in x86_64) GA=amd64;; aarch64|arm64) GA=arm64;; *) echo "unsupported arch $ARCH"; exit 1;; esac
  curl -fsSL "https://go.dev/dl/go${GOVER}.linux-${GA}.tar.gz" -o /tmp/go.tgz
  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go.tgz
  export PATH=/usr/local/go/bin:$PATH
  grep -q '/usr/local/go/bin' "$HOME/.profile" 2>/dev/null || echo 'export PATH=/usr/local/go/bin:$PATH' >> "$HOME/.profile"
fi

say "Cloning repo to $INSTALL_DIR"
if [ -d "$INSTALL_DIR/.git" ]; then
  git -C "$INSTALL_DIR" pull --ff-only
else
  git clone "$REPO" "$INSTALL_DIR"
fi

say "Building"
cd "$INSTALL_DIR"
go build -o jobpilot ./cmd/srv

say "Writing systemd unit"
sudo tee /etc/systemd/system/jobpilot.service >/dev/null <<UNIT
[Unit]
Description=JobPilot AI
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/jobpilot -listen :$PORT
Restart=on-failure
RestartSec=3
Environment=JOBPILOT_CONFIG=$INSTALL_DIR/jobpilot.config.json
${ADMIN_EMAIL:+Environment=ADMIN_EMAILS=$ADMIN_EMAIL}

[Install]
WantedBy=multi-user.target
UNIT

sudo systemctl daemon-reload
sudo systemctl enable --now jobpilot

say "Done!"
echo "Running on :$PORT — open http://<your-server>:$PORT"
echo "Configure at /admin (sign in with the email you set in ADMIN_EMAILS)."
echo "Logs:    sudo journalctl -u jobpilot -f"
echo "Restart: sudo systemctl restart jobpilot"
