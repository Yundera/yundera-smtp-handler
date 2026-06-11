# Mail Gateway

Standalone SMTP gateway for Personal Cloud Server (PCS) deployments. This service receives emails from containerized applications (Vaultwarden, Nextcloud, etc.) and forwards them over HTTPS to a relay backend (the orchestrator email API) for delivery via SendGrid.

## Architecture

```
App (Vaultwarden, Nextcloud, etc.)
     │
     │ SMTP (port 587)
     ↓
smtp container (this service)
     │
     │ HTTPS API call
     ↓
Relay backend (orchestrator)
     │
     ↓
SendGrid → Recipient
```

## Features

- **SMTP Relay**: Accepts SMTP connections on port 587
- **Email Parsing**: Full RFC-compliant MIME email parsing with inline image support
- **API Forwarding**: Forwards emails to the relay backend with JWT authentication
- **Network Isolation**: Runs in private Docker network (pcs)
- **Multi-platform**: Supports linux/amd64 and linux/arm64
- **Lightweight**: ~20MB Alpine-based Docker image

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RELAY_ENDPOINT_URL` | Yes | - | Base URL of the relay backend's email API (gateway POSTs to `{url}/email/send`) |
| `RELAY_CREDENTIAL` | Yes | - | Opaque bearer credential the relay verifies (a JWT on Yundera, a `userid:signature` on nsl) |
| `SMTP_PORT` | No | `587` | SMTP listening port |

### Docker Compose Example

```yaml
services:
  smtp:
    image: ghcr.io/yundera/mail-gateway:latest
    container_name: smtp
    hostname: smtp
    restart: unless-stopped
    environment:
      RELAY_CREDENTIAL: "${RELAY_CREDENTIAL}"
      RELAY_ENDPOINT_URL: "${RELAY_ENDPOINT_URL}"
      SMTP_PORT: "587"
    expose:
      - "587"
    networks:
      - pcs

networks:
  pcs:
    name: pcs
```

## App Configuration

Apps running in the same Docker network can connect to the SMTP service using the hostname `smtp`.

### Vaultwarden Example

```env
SMTP_HOST=smtp
SMTP_PORT=587
SMTP_FROM=vaultwarden@yourdomain.com
SMTP_SECURITY=off  # No TLS needed within Docker network
```

### Nextcloud Example

In Nextcloud admin settings:
- **Server address**: smtp
- **Port**: 587
- **Encryption**: None/STARTTLS

## Security

- **Network Isolation**: Service only accepts connections from the private `pcs` Docker network
- **No Public Exposure**: SMTP port is NOT exposed to the internet
- **JWT Authentication**: All API calls to orchestrator are authenticated with JWT
- **Rate Limiting**: Orchestrator enforces rate limits (100 emails/hour per user)

## Development

### Build Locally

```bash
docker build -t mail-gateway .
```

### Run Locally

```bash
docker run --rm \
  -e RELAY_CREDENTIAL="your-credential" \
  -e RELAY_ENDPOINT_URL="https://your-relay.example.com/service/pcs" \
  -p 587:587 \
  mail-gateway
```

### Test Email

```bash
# Send a test email using telnet
telnet localhost 587
> EHLO test
> MAIL FROM:<test@example.com>
> RCPT TO:<recipient@example.com>
> DATA
> Subject: Test Email
>
> This is a test email body.
> .
> QUIT
```

## Deployment

This service is automatically deployed to PCS instances via the `docker-compose.yml` template shipped with the mesh-router installer.

### GitHub Actions

The service is automatically built and published to GitHub Container Registry on:
- Push to `main` branch → `latest` tag
- Version tags (e.g., `v1.0.0`) → version-specific tags

## Troubleshooting

### Service won't start

Check logs:
```bash
docker logs smtp
```

Common issues:
- Missing `RELAY_CREDENTIAL` environment variable
- Port 587 already in use
- Network connectivity to orchestrator

### Emails not being sent

1. Check SMTP service logs: `docker logs smtp`
2. Verify orchestrator URL is correct
3. Check JWT token is valid
4. Verify app is configured to use `smtp` as hostname
5. Check orchestrator logs for API errors

## License

Copyright Yundera Team
