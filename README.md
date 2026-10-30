<p align="center">
  <img src="docs/assets/readme-banner.svg" width="1280" alt="Axottle — Infrastructure, connected." />
</p>

<p align="center"><strong>Your infrastructure. One place to run it.</strong></p>
<p align="center"><strong>English</strong> · <a href="README.ru.md">Русский</a> · <a href="https://docs.axottle.com">Documentation</a> · <a href="https://docs.axottle.com/ru/getting-started/quickstart">Getting started</a></p>

---

Axottle is a self-hosted control panel for running infrastructure built around Remnawave. Connect servers, design traffic routes, monitor services and manage customer support from one workspace.

It brings the operational picture together: which nodes serve a route, where a connection fails, which recovery policy applies and which customers are affected. From the first server to a distributed fleet, the same panel handles configuration, day-to-day operations and incident response.

## What you can do

### Design networks with Flow Studio

Build visual topologies from entry points, nodes, routes and transit pools. Import existing Remnawave configurations, manage Flow versions, inspect diagnostics and apply changes from the panel. HAProxy, WARP and Telegram proxies fit into the same network workflow.

### Manage nodes throughout their lifecycle

Connect existing servers over SSH or provision them through supported providers. Organize nodes into groups, configure domains and TLS, inspect metrics and logs, manage services and roll out updates through node agents.

### Detect problems and plan recovery

Health checks show the state of your infrastructure. Flow Guard lets you assemble recovery scenarios, define execution conditions and choose manual or explicitly permitted automatic recovery. Execution history keeps the outcome of each attempt visible.

### Run customer support alongside your infrastructure

Receive Telegram messages as tickets, assign operators, track priorities and SLA, and keep the conversation history attached to the request. Operators can work from the panel, a Telegram forum or a dedicated portal.

### Keep the rest of your operations connected

| Area | What it brings together |
| :--- | :--- |
| **Remnawave** | Panel connections, synchronization, backups, updates and migration workflows. |
| **Network services** | HAProxy parks, proxy services, OpenFlux, CDN resources and DNS integrations. |
| **Security** | Abuse signals, Egress Guard policies, node protection, access controls and an audit trail. |
| **Subscriptions** | Customer subscription pages connected to Remnawave. |
| **Statuspage** | Public service status, monitored components, incidents and scheduled maintenance. |
| **Providers & integrations** | Provider accounts, server provisioning and connections to external services. |
| **API & MCP** | External clients and automation with scoped access and a searchable operations catalog. |

## From your first node to daily operations

1. **Connect your installation.** Set up Axottle, connect Remnawave and register your nodes.
2. **Build a Flow.** Choose the nodes, entry points and routing your service needs.
3. **Check and apply.** Review the configuration, apply it and inspect the result.
4. **Operate from one panel.** Watch health, configure recovery, handle support requests and publish service status.

## Documentation

**[Read the Axottle documentation →](https://docs.axottle.com)**

Installation, configuration, operator guides and troubleshooting are available in the documentation. The guides are currently in Russian.

[Requirements](https://docs.axottle.com/ru/getting-started/requirements) · [Quick start](https://docs.axottle.com/ru/getting-started/quickstart) · [First connection](https://docs.axottle.com/ru/getting-started/first-connection) · [Flow Studio](https://docs.axottle.com/ru/connection-units/overview) · [Flow Guard](https://docs.axottle.com/ru/connection-units/recovery)

<details>
<summary><strong>Building from source</strong></summary>

This repository contains the source available for the shared foundation of Axottle. Operational modules are supplied separately; building this checkout alone does not provide all of the product features described above. The split distribution is still being prepared.

Use Go from `go.mod`, Node.js from `web/admin/.nvmrc`, npm and Python 3.

Before building the Go binary, compile the public UI. Generated bundles are not tracked in this source repository.

```sh
python3 scripts/open-core/build-ui.py
go build -trimpath -buildvcs=false -o axottle ./cmd/axottle-core
go test ./...
```

For standalone development, configure `AXOTTLE_DB_URL` and a persistent `AXOTTLE_SECRET_KEY`, then run `AXOTTLE_HOST=127.0.0.1 ./axottle serve`. Set `AXOTTLE_ENGINE_PATH` to use a separately supplied engine binary, or `AXOTTLE_ENGINE_SOCKET` to connect to a running engine.

The source in this repository is licensed under MIT; see [LICENSE](LICENSE). Separately supplied modules and third-party components retain their own terms.

</details>
