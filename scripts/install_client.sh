#!/bin/bash
# Installation script for probaky-client (Go binary)
# Installs the monitoring client on Proxmox VE or PBS nodes.
#
# Usage: install_client.sh [--api-url URL] [--api-key pbk-...] [--proxmox-token TOKEN] [--proxmox-secret SECRET] [--replace-env]

set -e

API_URL_ARG=""
API_KEY_ARG=""
PROXMOX_TOKEN_ARG=""
PROXMOX_SECRET_ARG=""
PRESERVE_ENV=true

while [[ $# -gt 0 ]]; do
    case "$1" in
        --api-url)      API_URL_ARG="$2";      shift 2 ;;
        --api-key)      API_KEY_ARG="$2";      shift 2 ;;
        --proxmox-token)   PROXMOX_TOKEN_ARG="$2";   shift 2 ;;
        --proxmox-secret)  PROXMOX_SECRET_ARG="$2";  shift 2 ;;
        --replace-env)  PRESERVE_ENV=false;    shift   ;;
        *)
            echo "Unknown parameter: $1"
            echo "Usage: $0 [--api-url URL] [--api-key pbk-...] [--proxmox-token TOKEN] [--proxmox-secret SECRET] [--replace-env]"
            exit 1
            ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="/opt/probaky"
LOG_DIR="/var/log/probaky"

echo "Installing probaky-client..."

# Create directories
mkdir -p "$INSTALL_DIR" "$LOG_DIR"

# Copy binary
BINARY="$PROJECT_ROOT/probaky-client"
if [ ! -f "$BINARY" ]; then
    echo "ERROR: Binary not found at $BINARY"
    echo "Build it first: go build -o probaky-client ./client/"
    exit 1
fi
cp "$BINARY" "$INSTALL_DIR/probaky-client"
chmod +x "$INSTALL_DIR/probaky-client"
echo "Binary installed: $INSTALL_DIR/probaky-client"

# Auto-generate Proxmox API token if not provided
AUTO_PROXMOX_TOKEN=""
AUTO_PROXMOX_SECRET=""

generate_proxmox_credentials() {
    if [ -n "$PROXMOX_TOKEN_ARG" ] && [ -n "$PROXMOX_SECRET_ARG" ]; then
        return 0
    fi
    if [ "$PRESERVE_ENV" = true ] && [ -f "$INSTALL_DIR/.env" ]; then
        echo "Preserving existing .env; skipping token generation"
        return 0
    fi

    local token_id="probaky-client"

    # Proxmox VE
    if command -v pveum >/dev/null 2>&1; then
        echo "Generating Proxmox VE API token..."
        pveum user token remove root@pam "$token_id" >/dev/null 2>&1 || true
        local pve_json
        pve_json=$(pveum user token add root@pam "$token_id" --privsep 0 \
            --comment "Probaky monitoring client" --output-format json 2>/dev/null || true)
        if [ -n "$pve_json" ]; then
            AUTO_PROXMOX_SECRET=$(python3 -c \
                "import json,sys; d=json.loads(sys.stdin.read()); print(d.get('value',''))" \
                <<< "$pve_json" 2>/dev/null || true)
            AUTO_PROXMOX_TOKEN="root@pam!$token_id"
        fi
        if [ -n "$AUTO_PROXMOX_TOKEN" ] && [ -n "$AUTO_PROXMOX_SECRET" ]; then
            echo "PVE API token generated"
            return 0
        fi
        echo "WARN: Could not auto-generate PVE API token"
    fi

    # Proxmox Backup Server
    if command -v proxmox-backup-manager >/dev/null 2>&1; then
        echo "Generating Proxmox Backup Server API token..."
        proxmox-backup-manager user delete-token root@pam "$token_id" >/dev/null 2>&1 || \
        proxmox-backup-manager user token remove root@pam "$token_id" >/dev/null 2>&1 || true

        local pbs_json=""
        pbs_json=$(proxmox-backup-manager user generate-token root@pam "$token_id" \
            --output-format json 2>/dev/null) || \
        pbs_json=$(proxmox-backup-manager user token add root@pam "$token_id" \
            --output-format json 2>/dev/null) || true

        if [ -n "$pbs_json" ]; then
            AUTO_PROXMOX_SECRET=$(python3 -c \
                "import json,sys; d=json.loads(sys.stdin.read()); print(d.get('value',''))" \
                <<< "$pbs_json" 2>/dev/null || true)
            AUTO_PROXMOX_TOKEN="root@pam!$token_id"
        fi
        if [ -n "$AUTO_PROXMOX_TOKEN" ] && [ -n "$AUTO_PROXMOX_SECRET" ]; then
            echo "PBS API token generated"
            return 0
        fi
        echo "WARN: Could not auto-generate PBS API token"
    fi
}

generate_proxmox_credentials || true

[ -z "$PROXMOX_TOKEN_ARG" ] && [ -n "$AUTO_PROXMOX_TOKEN" ] && PROXMOX_TOKEN_ARG="$AUTO_PROXMOX_TOKEN"
[ -z "$PROXMOX_SECRET_ARG" ] && [ -n "$AUTO_PROXMOX_SECRET" ] && PROXMOX_SECRET_ARG="$AUTO_PROXMOX_SECRET"

# Configure .env
ENV_FILE="$INSTALL_DIR/.env"
if [ -f "$ENV_FILE" ] && [ "$PRESERVE_ENV" = true ]; then
    echo ".env already exists — preserving (use --replace-env to overwrite)"
elif [ -n "$API_KEY_ARG" ]; then
    cat > "$ENV_FILE" << EOF
API_KEY=$API_KEY_ARG
API_URL=${API_URL_ARG:-http://localhost:36748}
PROXMOX_TOKEN=${PROXMOX_TOKEN_ARG}
PROXMOX_SECRET=${PROXMOX_SECRET_ARG}
# Set to false if Proxmox uses a self-signed certificate (common default)
PROXMOX_VERIFY_TLS=false
EOF
    chmod 600 "$ENV_FILE"
    echo ".env written to $ENV_FILE"
    [ -z "$PROXMOX_TOKEN_ARG" ] && echo "WARN: PROXMOX_TOKEN missing — edit $ENV_FILE"
else
    cat > "$ENV_FILE" << 'EOF'
# API key from the Probaky web interface (Settings > API Keys, must start with pbk-)
API_KEY=pbk-...
# URL of the Probaky server
API_URL=http://your-probaky-server:36748
# Proxmox API token — generated automatically during install, or set manually
PROXMOX_TOKEN=root@pam!probaky-client
PROXMOX_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
# Set to false for self-signed certificates (default on Proxmox)
PROXMOX_VERIFY_TLS=false
EOF
    chmod 600 "$ENV_FILE"
    echo "Template .env created — edit $ENV_FILE with correct values"
fi

# Install vzdump hook
echo "Installing vzdump hook..."
if [ ! -f "$SCRIPT_DIR/vzdump_client.sh" ]; then
    echo "ERROR: vzdump_client.sh not found in $SCRIPT_DIR"
    exit 1
fi
cp "$SCRIPT_DIR/vzdump_client.sh" "$INSTALL_DIR/vzdump_client.sh"
chmod +x "$INSTALL_DIR/vzdump_client.sh"

# Register hook in /etc/vzdump.conf (PVE only)
if [ -f /etc/vzdump.conf ]; then
    if ! grep -q 'script: /opt/probaky/vzdump_client.sh' /etc/vzdump.conf; then
        echo "script: /opt/probaky/vzdump_client.sh" >> /etc/vzdump.conf
        echo "vzdump hook added to /etc/vzdump.conf"
    else
        echo "vzdump hook already configured"
    fi
fi

# Configure logrotate
cat > /etc/logrotate.d/probaky << EOF
$LOG_DIR/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
}
EOF
echo "logrotate configured"

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
if [ -z "$API_KEY_ARG" ]; then
    echo "  1. Edit $ENV_FILE with your API key and server URL"
fi
echo "  2. Test: $INSTALL_DIR/probaky-client"
echo "  3. Backups will be reported automatically via the vzdump hook"
echo ""
echo "To update: copy new probaky-client binary to $INSTALL_DIR/"
