## Architecture Overview

This repository implements a distributed VPN management platform built around a central control plane and lightweight node agents.

### Components

- **Master Panel (`cmd/master/`)**
  - Gin-based REST API consumed by the web UI, node agents and subscription clients
  - GORM data layer with support for SQLite, MySQL and PostgreSQL
  - Background workers for health monitoring and statistics aggregation
  - Subscription engine that produces multi-format bundles (JSON, Clash, V2Ray, Shadowrocket)
- **Node Agent (`cmd/agent/`)**
  - Go daemon exposing authenticated control endpoints (`/api/health`, `/api/sync`, `/api/update`, `/api/restart`)
  - Handles unattended Xray-core installation, configuration updates and service lifecycle
  - Periodically reports telemetry (CPU, memory, online users, per-client usage)
- **Frontend (`web/admin/`)**
  - Vue 3 + Vite single-page application for administrators
  - Provides dashboard metrics and node inventory views using the `/api/admin/*` endpoints
- **Shared Packages (`internal/`)**
  - `config` — environment/file configuration helpers
  - `database` — connection factory + migrations
  - `service` — domain services (nodes, subscriptions, health monitor)
  - `security` — HMAC utilities and secret generation
- **Deployment Scripts (`scripts/`)**
  - `install-master.sh` — bootstrap master panel, systemd service, env file
  - `install-agent.sh` — install node agent binary, systemd unit, initial configuration

### Data Flow

1. **Registration** — the agent posts metadata to `/api/nodes/register` signed with its registration secret; the master issues a shared secret for subsequent calls.
2. **Configuration Sync** — the master generates Xray JSON via `ConfigService`, the agent applies it locally and restarts the service if required.
3. **Telemetry** — agents push stats to `/api/nodes/{id}/stats`; the master updates node status and per-client usage.
4. **Subscriptions** — clients fetch bundles from `/api/subscriptions/{uuid}` receiving load-balanced node endpoints in multiple formats.
5. **Monitoring** — the master-side `HealthMonitor` periodically hits `/api/health` and Xray stats endpoints on each node to derive `online/offline/degraded` statuses.

### Current Status

- ✅ Node agent installation, config application and telemetry
- ✅ Master panel CRUD for nodes, groups, subscriptions (via services) and multi-format exports
- ✅ Admin API + initial Vue dashboard
- 🚧 Extended frontend (groups, subscription management UI) — forthcoming
- 🚧 Role-based access control and API keys — planned

The roadmap will evolve together with the remaining modules (full admin UI, billing integrations, granular RBAC).

