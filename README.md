# Yundera SMTP Handler

Standalone SMTP relay service for Yundera Personal Cloud Server (PCS) deployments. This service receives emails from containerized applications (Vaultwarden, Nextcloud, etc.) and forwards them to the Yundera orchestrator API for delivery via SendGrid.

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
Yundera Orchestrator
     │
     ↓
SendGrid → Recipient
```

## Features

- **SMTP Relay**: Accepts SMTP connections on port 587
- **Email Parsing**: Full RFC-compliant MIME email parsing with inline image support
- **API Forwarding**: Forwards emails to Yundera orchestrator with JWT authentication
- **Network Isolation**: Runs in private Docker network (pcs)
- **Multi-platform**: Supports linux/amd64 and linux/arm64
- **Lightweight**: ~20MB Alpine-based Docker image

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `USER_JWT` | Yes | - | JWT token for authenticating with Yundera orchestrator |
| `ORCHESTRATOR_URL` | No | `https://nasselle.com/service/pcs` | Yundera orchestrator API endpoint |
| `SMTP_PORT` | No | `587` | SMTP listening port |

### Docker Compose Example

```yaml
services:
  smtp:
    image: ghcr.io/yundera/yundera-smtp-handler:latest
    container_name: smtp
    hostname: smtp
    restart: unless-stopped
    environment:
      USER_JWT: "${USER_JWT}"
      ORCHESTRATOR_URL: "https://orchestrator.yundera.com"
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
docker build -t yundera-smtp-handler .
```

### Run Locally

```bash
docker run --rm \
  -e USER_JWT="your-jwt-token" \
  -e ORCHESTRATOR_URL="https://nasselle.com/service/pcs" \
  -p 587:587 \
  yundera-smtp-handler
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

This service is automatically deployed to Yundera PCS instances via the `compose-template.yml` in the settings-center-app package.

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
- Missing `USER_JWT` environment variable
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
