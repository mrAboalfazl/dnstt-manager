# DNSTT Manager

A management panel with REST API for DNSTT DNS tunnel servers. Provides per-user connection limits, traffic tracking, automatic expiry, and a web dashboard.

## Features

- **Per-User Connection Limits** - Limit concurrent SSH sessions per user (1, 2, 5, etc.)
- **Traffic Tracking** - Optional per-user traffic limits (or unlimited)
- **Auto-Disable** - Automatically disables expired users and users who exceed traffic limits
- **REST API** - Full API for integration with external services
- **Web Dashboard** - Admin panel for managing users and server
- **Config Generation** - Generate per-user config files and `dnstt://` links
- **Test Users** - Create temporary test accounts with custom limits

## Architecture

```
Internet → DNS Query → dnstt-server (:5300) → SSH Server (:2222) → Proxy/Tunnel
                                                    ↑
                                          User Auth + Connection Limits
                                                    ↑
                                            DNSTT Manager API (:8080)
```

DNSTT itself has no per-user authentication. This panel layers user management on top via a custom SSH server that authenticates users and enforces limits.

## Quick Install

```bash
curl -sL https://raw.githubusercontent.com/mrAboalfazl/dnstt-manager/main/scripts/install.sh | sudo bash
```

## Manual Install

### Prerequisites

- Linux server (Ubuntu 22/24, Debian 12, CentOS, Fedora)
- Domain with NS records pointing to your server
- Go 1.21+ (for building)

### Build

```bash
git clone https://github.com/mrAboalfazl/dnstt-manager.git
cd dnstt-manager
go build -o dnstt-manager .
```

### Setup

1. Download the dnstt-server binary:
```bash
wget -O /usr/local/bin/dnstt-server https://dnstt.network/dnstt-server-linux-amd64
chmod +x /usr/local/bin/dnstt-server
```

2. Generate DNSTT keys:
```bash
dnstt-server -gen-key -privkey-file ~/.dnstt-manager/dnstt-server.key -pubkey-file ~/.dnstt-manager/dnstt-server.pub
```

3. Configure DNS records:
```
A    tns.example.com    → YOUR_SERVER_IP
NS   t.example.com      → tns.example.com
```

4. Setup iptables NAT:
```bash
iptables -t nat -A PREROUTING -p udp --dport 53 -j REDIRECT --to-ports 5300
```

5. Run:
```bash
./dnstt-manager
```

## Configuration

Config file is stored at `~/.dnstt-manager/config.json`:

```json
{
  "api_key": "your-api-key-here",
  "http_port": 8080,
  "admin_user": "admin",
  "admin_pass": "admin",
  "ssh_port": 2222,
  "dnstt_binary": "/usr/local/bin/dnstt-server",
  "dnstt_domain": "t.example.com",
  "dnstt_port": 5300,
  "mtu": 1232,
  "forward_addr": "127.0.0.1:2222",
  "dns_resolver": "https://cloudflare-dns.com/dns-query",
  "server_ip": "YOUR_IP",
  "monitor_interval_sec": 30
}
```

## API Reference

All API endpoints require the `X-API-Key` header.

### Users

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/users` | Create user |
| GET | `/api/users` | List users (pagination, filter) |
| GET | `/api/users/:id` | Get user details |
| PUT | `/api/users/:id` | Update user |
| DELETE | `/api/users/:id` | Delete user |
| POST | `/api/users/:id/enable` | Enable user |
| POST | `/api/users/:id/disable` | Disable user |
| POST | `/api/users/:id/reset-traffic` | Reset traffic counter |
| GET | `/api/users/:id/config` | Get config file |
| GET | `/api/users/:id/link` | Get connection link |

### Test Users

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/test-users` | Create test user |

### Server

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/server/status` | Server status + stats |
| POST | `/api/server/start` | Start DNSTT server |
| POST | `/api/server/stop` | Stop DNSTT server |
| POST | `/api/server/restart` | Restart DNSTT server |
| GET | `/api/server/config` | Get server config |
| PUT | `/api/server/config` | Update server config |

### Connections

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/connections` | List active connections |
| POST | `/api/connections/:username/kick` | Kick user |

### Example: Create User

```bash
curl -X POST http://localhost:8080/api/users \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john",
    "password": "secret123",
    "max_connections": 2,
    "traffic_limit_gb": 50,
    "expires_at": "2026-04-01"
  }'
```

### Example: Create Test User

```bash
curl -X POST http://localhost:8080/api/test-users \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "max_connections": 1,
    "traffic_limit_gb": 1,
    "expires_at": "2026-03-06T00:00:00Z"
  }'
```

Response includes username, password, config, and connection link.

### Example: Get User Config

```bash
curl http://localhost:8080/api/users/1/config \
  -H "X-API-Key: YOUR_API_KEY"
```

### Example: Get Connection Link

```bash
curl http://localhost:8080/api/users/1/link \
  -H "X-API-Key: YOUR_API_KEY"
```

## Web Dashboard

Access the admin dashboard at `http://YOUR_IP:8080/login`.

Default credentials: `admin` / `admin`

Features:
- Dashboard with stats overview
- User management (create, edit, enable/disable, delete)
- Server controls (start, stop, restart)
- Settings management
- Quick test user generation

## How It Works

1. **DNSTT Server** listens on UDP port 5300, receives DNS-tunneled traffic
2. Port 53 is redirected to 5300 via iptables NAT
3. Decoded traffic is forwarded to the **SSH Server** on port 2222
4. SSH Server **authenticates** users against the database
5. **Connection limits** are enforced per user
6. **Traffic** is tracked through SSH channel wrappers
7. **Monitor** goroutine auto-disables expired / over-quota users every 30s

## License

MIT
