# AWX Host Metrics Exporter

A Prometheus exporter for Ansible AWX/Tower that provides detailed host and group metrics not available in the default AWX metrics endpoint.

## Why This Exporter?

While AWX provides its own metrics endpoint (`/api/v2/metrics`), it lacks detailed host-level information including:
- Host group membership relationships
- Detailed host status (active failures, inventory sources)
- Comprehensive timestamp data (creation, modification, facts updates)
- Group inventory context

This exporter fetches data from AWX's `/api/v2/hosts/` endpoint and transforms it into Prometheus metrics with rich labeling.

## Features

- **Host Information**: Basic host metadata (ID, name, inventory, enabled status)
- **Host Status**: Active failures and inventory source status
- **Timestamps**: Creation, modification, and facts update times
- **Group Membership**: Which hosts belong to which groups
- **Group Information**: Group names and inventory context
- **Configurable Scraping**: Adjustable scrape interval (default: 5 minutes)

## Installation

### Docker

```bash
docker run -d \
  -e AWX_HOST="your.awx.server" \
  -e AWX_USER="username" \
  -e AWX_PASSWORD="password" \
  -e HTTP="false" \
  -e TLS_INSECURE="false" \
  -p 8080:8080 \
  ghcr.io/your-repo/awx-exporter:latest