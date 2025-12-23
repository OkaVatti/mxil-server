# MXIL Server - Multi-Network Email Server

![Version](https://img.shields.io/badge/version-1.0.0-blue)
![Go Version](https://img.shields.io/badge/go-1.25%2B-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Build Status](https://img.shields.io/badge/build-passing-brightgreen)

MXIL (Multi-network eXchange & Interconnect Layer) is a privacy-focused, multi-network email server that supports clearnet, I2P, and other networks. It provides end-to-end encryption, advanced security features, and a modern REST API.

## Features

- **Multi-Network Support**: Send and receive emails via clearnet, I2P, and Tor
- **End-to-End Encryption**: Built-in encryption for all communications
- **Privacy Focused**: Metadata minimization, no tracking, self-hosted
- **Modern API**: RESTful API with WebSocket support for real-time updates
- **Security**: Argon2 password hashing, JWT authentication, rate limiting
- **Storage**: Local, S3, and IPFS storage backends
- **Scalable**: Built with Go, PostgreSQL, and Redis

## Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL 15+
- Redis 7+
- Docker & Docker Compose (optional)

### Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com/yourusername/mxil-server.git
   cd mxil-server
   ```

2. **Configure Environment Variables**
   Copy the example configuration file and modify it as needed:

   ```bash
   cp .env.example .env
   cp config.example.yml config.yml
   ```

3. **Build and run**

   Using Docker Compose:

   ```bash
   docker-compose up --build
   ```

   ```bash
   make build
   ./build/mxil-server
   ```

   Or build and run manually:

   ```bash
   go build -o mxil-server ./cmd/mxil-server
   ./mxil-server
   ```

### Using Docker

#### Generate SSL certificates

```bash
make generate-certs
```

##### Start all services

```
make compose-up
```

##### View logs

```
make compose-logs
```

### API Documentation

Once running, the API is available at `http://localhost:8080/api/v1`

#### Authentication

```bash
# Register a new user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "SecurePassword123!",
    "email": "test@example.com",
    "display_name": "Test User"
  }'

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "SecurePassword123!"
  }'
```

#### Sending an Email

```bash
# Send an email via clearnet
curl -X POST http://localhost:8080/api/v1/emails \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "to": ["recipient@example.com"],
    "subject": "Hello from MXIL",
    "body_plain": "This is a test email sent via MXIL server.",
    "network": "clearnet"
  }'
```

#### Receiving Email

MXIL automatically processes incoming emails via configured networks (SMTP, I2P, etc.).

### Network Configuration

#### Clearnet (SMTP/IMAP)

Configure your domain's MX records to point to your MXIL server.

#### I2P Network

    1. Install and run I2P router

    2. Enable I2P in config:

    ```yaml
    network:
      enable_i2p: true
      i2p_router_host: "localhost"
      i2p_router_port: 7656
    ```

#### TOR Network

    1. Install and run Tor service

    2. Enable Tor in config:

    ```yaml
    network:
      enable_tor: true
      tor_socks_host: "localhost"
      tor_socks_port: 9050
    ```

### Security Features

    - Password Requirements: Minimum 12 characters, symbols, numbers, uppercase
    - Rate Limiting: Built-in rate limiting for all endpoints
    - Session Management: Multiple sessions, device tracking
    - MFA Support: Time-based OTP (TOTP) authentication
    - Encryption: AES-256-GCM for data at rest, TLS for transport
    - Audit Logging: Comprehensive audit trail for all actions
