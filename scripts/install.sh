#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}  DNSTT Manager Installer${NC}"
echo -e "${BLUE}=========================================${NC}"

# Check root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Please run as root${NC}"
    exit 1
fi

# Detect OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
else
    echo -e "${RED}Unsupported OS${NC}"
    exit 1
fi

echo -e "${GREEN}Detected OS: $OS${NC}"

# Install dependencies
echo -e "${YELLOW}Installing dependencies...${NC}"
case $OS in
    ubuntu|debian)
        apt update -y
        apt install -y curl wget unzip jq
        ;;
    centos|fedora|rocky)
        dnf install -y curl wget unzip jq
        ;;
    *)
        echo -e "${RED}Unsupported OS: $OS${NC}"
        exit 1
        ;;
esac

# Get server IP
SERVER_IP=$(curl -s4 ifconfig.me || curl -s4 icanhazip.com || echo "0.0.0.0")
echo -e "${GREEN}Server IP: $SERVER_IP${NC}"

# Prompt for domain
echo ""
read -p "Enter your DNSTT tunnel domain (e.g., t.example.com): " DNSTT_DOMAIN
if [ -z "$DNSTT_DOMAIN" ]; then
    echo -e "${RED}Domain is required${NC}"
    exit 1
fi

# Prompt for admin password
read -p "Enter admin password [admin]: " ADMIN_PASS
ADMIN_PASS=${ADMIN_PASS:-admin}

# Create data directory
DATA_DIR="$HOME/.dnstt-manager"
mkdir -p "$DATA_DIR"

# Download dnstt-server binary
DNSTT_BIN="/usr/local/bin/dnstt-server"
if [ ! -f "$DNSTT_BIN" ]; then
    echo -e "${YELLOW}Downloading dnstt-server...${NC}"
    ARCH=$(uname -m)
    case $ARCH in
        x86_64|amd64) DNSTT_ARCH="amd64" ;;
        aarch64|arm64) DNSTT_ARCH="arm64" ;;
        *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
    esac

    wget -O "$DNSTT_BIN" "https://dnstt.network/dnstt-server-linux-${DNSTT_ARCH}" || {
        echo -e "${RED}Failed to download dnstt-server. Please download manually.${NC}"
    }
    chmod +x "$DNSTT_BIN"
fi

# Generate DNSTT keys
if [ ! -f "$DATA_DIR/dnstt-server.key" ]; then
    echo -e "${YELLOW}Generating DNSTT keys...${NC}"
    "$DNSTT_BIN" -gen-key -privkey-file "$DATA_DIR/dnstt-server.key" -pubkey-file "$DATA_DIR/dnstt-server.pub"
    chmod 600 "$DATA_DIR/dnstt-server.key"
    chmod 644 "$DATA_DIR/dnstt-server.pub"
fi

PUBKEY=$(cat "$DATA_DIR/dnstt-server.pub")
echo -e "${GREEN}Public key: $PUBKEY${NC}"

# Download dnstt-manager binary
MANAGER_BIN="/usr/local/bin/dnstt-manager"
echo -e "${YELLOW}Downloading dnstt-manager...${NC}"
LATEST=$(curl -s https://api.github.com/repos/mrAboalfazl/dnstt-manager/releases/latest | jq -r '.tag_name // empty')
if [ -n "$LATEST" ]; then
    wget -O "$MANAGER_BIN" "https://github.com/mrAboalfazl/dnstt-manager/releases/download/${LATEST}/dnstt-manager-linux-amd64" || {
        echo -e "${YELLOW}No release found. You can build from source:${NC}"
        echo -e "  go build -o $MANAGER_BIN ."
    }
    chmod +x "$MANAGER_BIN" 2>/dev/null
else
    echo -e "${YELLOW}No release found. Build from source:${NC}"
    echo -e "  cd /path/to/dnstt-manager && go build -o $MANAGER_BIN ."
fi

# Generate API key
API_KEY=$(openssl rand -hex 32)

# Create config
cat > "$DATA_DIR/config.json" <<EOF
{
  "api_key": "$API_KEY",
  "http_port": 8080,
  "admin_user": "admin",
  "admin_pass": "$ADMIN_PASS",
  "ssh_port": 2222,
  "host_key_dir": "$DATA_DIR",
  "dnstt_binary": "$DNSTT_BIN",
  "dnstt_domain": "$DNSTT_DOMAIN",
  "dnstt_port": 5300,
  "privkey_file": "$DATA_DIR/dnstt-server.key",
  "pubkey_file": "$DATA_DIR/dnstt-server.pub",
  "mtu": 1232,
  "forward_addr": "127.0.0.1:2222",
  "dns_resolver": "https://cloudflare-dns.com/dns-query",
  "server_ip": "$SERVER_IP",
  "db_path": "$DATA_DIR/dnstt-manager.db",
  "monitor_interval_sec": 30
}
EOF
chmod 600 "$DATA_DIR/config.json"

# Setup iptables NAT
echo -e "${YELLOW}Setting up iptables NAT rules...${NC}"
iptables -t nat -A PREROUTING -i $(ip route | grep default | awk '{print $5}' | head -1) -p udp --dport 53 -j REDIRECT --to-ports 5300 2>/dev/null || true
ip6tables -t nat -A PREROUTING -i $(ip route | grep default | awk '{print $5}' | head -1) -p udp --dport 53 -j REDIRECT --to-ports 5300 2>/dev/null || true

# Save iptables
if command -v iptables-save &>/dev/null; then
    mkdir -p /etc/iptables
    iptables-save > /etc/iptables/rules.v4 2>/dev/null || true
    ip6tables-save > /etc/iptables/rules.v6 2>/dev/null || true
fi

# Create systemd service for dnstt-manager
cat > /etc/systemd/system/dnstt-manager.service <<EOF
[Unit]
Description=DNSTT Manager Panel
After=network.target

[Service]
Type=simple
ExecStart=$MANAGER_BIN --config $DATA_DIR/config.json
Restart=always
RestartSec=5
WorkingDirectory=$DATA_DIR

[Install]
WantedBy=multi-user.target
EOF

# Create systemd service for dnstt-server (managed separately if not through panel)
cat > /etc/systemd/system/dnstt-server.service <<EOF
[Unit]
Description=DNSTT DNS Tunnel Server
After=network.target

[Service]
Type=simple
ExecStart=$DNSTT_BIN -udp :5300 -privkey-file $DATA_DIR/dnstt-server.key -mtu 1232 $DNSTT_DOMAIN 127.0.0.1:2222
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable and start services
systemctl daemon-reload
systemctl enable dnstt-manager
systemctl start dnstt-manager || echo -e "${YELLOW}Note: dnstt-manager binary needs to be built/downloaded first${NC}"

# Open firewall ports
if command -v ufw &>/dev/null; then
    ufw allow 53/udp
    ufw allow 8080/tcp
    ufw allow 2222/tcp
elif command -v firewall-cmd &>/dev/null; then
    firewall-cmd --permanent --add-port=53/udp
    firewall-cmd --permanent --add-port=8080/tcp
    firewall-cmd --permanent --add-port=2222/tcp
    firewall-cmd --reload
fi

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}  Installation Complete!${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""
echo -e "  Dashboard:  http://$SERVER_IP:8080"
echo -e "  Admin User: admin"
echo -e "  Admin Pass: $ADMIN_PASS"
echo -e "  API Key:    $API_KEY"
echo ""
echo -e "  DNSTT Domain: $DNSTT_DOMAIN"
echo -e "  Public Key:   $PUBKEY"
echo ""
echo -e "${YELLOW}DNS Records needed:${NC}"
echo -e "  A    tns.${DNSTT_DOMAIN#*.}  ->  $SERVER_IP"
echo -e "  NS   $DNSTT_DOMAIN  ->  tns.${DNSTT_DOMAIN#*.}"
echo ""
echo -e "${YELLOW}Config file: $DATA_DIR/config.json${NC}"
echo -e "${YELLOW}Database:    $DATA_DIR/dnstt-manager.db${NC}"
echo ""
