<div align="center">

<img src="web/public/logo.svg" alt="Octopus Logo" width="120" height="120">

### Octopus

**A Simple, Beautiful, and Elegant LLM API Aggregation & Load Balancing Service for Individuals**

 English | [简体中文](README_zh.md)

</div>


## ✨ Features

- 🔀 **Multi-Channel Aggregation** - Connect multiple LLM provider channels with unified management
- ⚖️ **Load Balancing** - Automatic request distribution for stable and efficient service
- 🔄 **Protocol Conversion** - Seamless conversion between OpenAI Chat / OpenAI Responses / Anthropic / Gemini / Antigravity API formats
- 💰 **Price Sync** - Automatic model pricing updates
- 🔃 **Model Sync** - Automatic synchronization of available model lists with channels
- 📊 **Analytics** - Comprehensive request statistics, token consumption, and cost tracking
- 🎨 **Elegant UI** - Clean and beautiful web management panel
- 🗄️ **Multi-Database Support** - Support for SQLite, MySQL, PostgreSQL
- 🏷️ **Per-Channel Model Pricing** - Independent model pricing configuration for each channel, supporting same model names across different channels with unique pricing
- 🔌 **Dynamic Channel Types** - Channel types are fetched from backend API, making it easy to add new provider support


## 🚀 Quick Start

### 🐳 Docker

Run directly:

```bash
docker run -d --name octopus -v /path/to/data:/app/data -p 8080:8080 bestrui/octopus
```

Or use docker compose:

```bash
wget https://raw.githubusercontent.com/bestruirui/octopus/refs/heads/dev/docker-compose.yml
docker compose up -d
```


### 📦 Download from Release

Download the binary for your platform from [Releases](https://github.com/ffflyZzz/octopus/releases), then run:

```bash
./octopus start
```

### 🛠️ Build from Source

**Requirements:**
- Go 1.24.4
- Node.js 18+
- npm or pnpm

```bash
# Clone the repository
git clone https://github.com/ffflyZzz/octopus.git
cd octopus

# 1. Build frontend
cd web

# Using npm
npm install
npm run build

# Or using pnpm
pnpm install
pnpm run build

cd ..

# 2. Move frontend assets to static directory
mv web/out static/

# 3. Start the backend service
go run . start
```

> 💡 **Tip**: The frontend build artifacts are embedded into the Go binary, so you must build the frontend before starting the backend.

### 🔐 Default Credentials

After first launch, visit http://localhost:8080 and log in to the management panel with:

- **Username**: `admin`
- **Password**: `admin`

> ⚠️ **Security Notice**: Please change the default password immediately after first login.

### 📝 Configuration File

The configuration file is located at `data/config.json` by default and is automatically generated on first startup.

**Complete Configuration Example:**

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "database": {
    "type": "sqlite",
    "path": "data/data.db"
  },
  "log": {
    "level": "info"
  }
}
```

**Configuration Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `server.host` | Listen address | `0.0.0.0` |
| `server.port` | Server port | `8080` |
| `database.type` | Database type | `sqlite` |
| `database.path` | Database connection string | `data/data.db` |
| `log.level` | Log level | `info` |

**Database Configuration:**

Three database types are supported:

| Type | `database.type` | `database.path` Format |
|------|-----------------|-----------------------|
| SQLite | `sqlite` | `data/data.db` |
| MySQL | `mysql` | `user:password@tcp(host:port)/dbname` |
| PostgreSQL | `postgres` | `postgresql://user:password@host:port/dbname?sslmode=disable` |

**MySQL Configuration Example:**

```json
{
  "database": {
    "type": "mysql",
    "path": "root:password@tcp(127.0.0.1:3306)/octopus"
  }
}
```

**PostgreSQL Configuration Example:**

```json
{
  "database": {
    "type": "postgres",
    "path": "postgresql://user:password@localhost:5432/octopus?sslmode=disable"
  }
}
```

> 💡 **Tip**: MySQL and PostgreSQL require manual database creation. The application will automatically create the table structure.

### 🌐 Environment Variables

All configuration options can be overridden via environment variables using the format `OCTOPUS_` + configuration path (joined with `_`):

| Environment Variable | Configuration Option |
|---------------------|---------------------|
| `OCTOPUS_SERVER_PORT` | `server.port` |
| `OCTOPUS_SERVER_HOST` | `server.host` |
| `OCTOPUS_DATABASE_TYPE` | `database.type` |
| `OCTOPUS_DATABASE_PATH` | `database.path` |
| `OCTOPUS_LOG_LEVEL` | `log.level` |
| `OCTOPUS_GITHUB_PAT` | For rate limiting when getting the latest version (optional) |

### 🔌 Amp CLI Integration

Octopus supports [Amp CLI](https://ampcode.com) proxy integration, allowing you to use Octopus as a proxy for Amp CLI requests.

**Configuration** (in `data/config.json`):

```json
{
  "ampcode": {
    "enabled": true,
    "upstream_url": "https://ampcode.com",
    "upstream_api_key": "your-api-key"
  }
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `enabled` | Enable Amp CLI proxy | `false` |
| `upstream_url` | Amp upstream URL | `https://ampcode.com` |
| `upstream_api_key` | Amp API key (optional) | `""` |
| `restrict_management_to_localhost` | Restrict management API to localhost | `false` |

**API Key Priority:**

The API key is resolved in the following order:
1. **Config file**: `ampcode.upstream_api_key` in config.json
2. **Environment variable**: `AMP_API_KEY`
3. **Amp secrets file**: `~/.local/share/amp/secrets.json`

> 💡 **Tip**: If you have Amp CLI installed locally, Octopus can automatically use its stored credentials.

## 📸 Screenshots

### 🖥️ Desktop

<div align="center">
<table>
<tr>
<td align="center"><b>Dashboard</b></td>
<td align="center"><b>Channel Management</b></td>
<td align="center"><b>Group Management</b></td>
</tr>
<tr>
<td><img src="web/public/screenshot/desktop-home.png" alt="Dashboard" width="400"></td>
<td><img src="web/public/screenshot/desktop-channel.png" alt="Channel" width="400"></td>
<td><img src="web/public/screenshot/desktop-group.png" alt="Group" width="400"></td>
</tr>
<tr>
<td align="center"><b>Price Management</b></td>
<td align="center"><b>Logs</b></td>
<td align="center"><b>Settings</b></td>
</tr>
<tr>
<td><img src="web/public/screenshot/desktop-price.png" alt="Price Management" width="400"></td>
<td><img src="web/public/screenshot/desktop-log.png" alt="Logs" width="400"></td>
<td><img src="web/public/screenshot/desktop-setting.png" alt="Settings" width="400"></td>
</tr>
</table>
</div>

### 📱 Mobile

<div align="center">
<table>
<tr>
<td align="center"><b>Home</b></td>
<td align="center"><b>Channel</b></td>
<td align="center"><b>Group</b></td>
<td align="center"><b>Price</b></td>
<td align="center"><b>Logs</b></td>
<td align="center"><b>Settings</b></td>
</tr>
<tr>
<td><img src="web/public/screenshot/mobile-home.png" alt="Mobile Home" width="140"></td>
<td><img src="web/public/screenshot/mobile-channel.png" alt="Mobile Channel" width="140"></td>
<td><img src="web/public/screenshot/mobile-group.png" alt="Mobile Group" width="140"></td>
<td><img src="web/public/screenshot/mobile-price.png" alt="Mobile Price" width="140"></td>
<td><img src="web/public/screenshot/mobile-log.png" alt="Mobile Logs" width="140"></td>
<td><img src="web/public/screenshot/mobile-setting.png" alt="Mobile Settings" width="140"></td>
</tr>
</table>
</div>


## 📖 Documentation

### 📡 Channel Management

Channels are the basic configuration units for connecting to LLM providers.

**Base URL Guide:**

The program automatically appends API paths based on channel type. You only need to provide the base URL:

| Channel Type | Auto-appended Path | Base URL | Full Request URL Example |
|--------------|-------------------|----------|--------------------------|
| OpenAI Chat | `/chat/completions` | `https://api.openai.com/v1` | `https://api.openai.com/v1/chat/completions` |
| OpenAI Responses | `/responses` | `https://api.openai.com/v1` | `https://api.openai.com/v1/responses` |
| Anthropic | `/messages` | `https://api.anthropic.com/v1` | `https://api.anthropic.com/v1/messages` |
| Gemini | `/models/:model:generateContent` | `https://generativelanguage.googleapis.com/v1beta` | `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent` |
| Antigravity | `/v1internal:streamGenerateContent` or `/v1internal:generateContent` | `https://daily-cloudcode-pa.sandbox.googleapis.com` | `https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:streamGenerateContent` |

> 💡 **Tip**: No need to include specific API endpoint paths in the Base URL - the program handles this automatically.

**Antigravity Channel:**

Antigravity provides access to Google's Claude models through their internal API:

- **Base URL**: `https://daily-cloudcode-pa.sandbox.googleapis.com`
- **API Key**: Supports both Refresh Token (starting with `1//`) and Access Token (starting with `ya29.`)
- **Auto Token Refresh**: When using Refresh Token, the system automatically refreshes access tokens before expiration
- **Models**: Supports Claude models like `claude-sonnet-4-5-20250929`, `claude-opus-4-5-20251101`, etc.

> 💡 **Tip**: The system intelligently detects token type and handles refresh automatically. Refresh tokens are cached and refreshed 5 minutes before expiration.

---

### 📁 Group Management

Groups aggregate multiple channels into a unified external model name.

**Core Concepts:**

- **Group name** is the model name exposed by the program
- When calling the API, set the `model` parameter to the group name

**Load Balancing Modes:**

| Mode | Description |
|------|-------------|
| 🔄 **Round Robin** | Cycles through channels sequentially for each request |
| 🎲 **Random** | Randomly selects an available channel for each request |
| 🛡️ **Failover** | Prioritizes high-priority channels, switches to lower priority only on failure |
| ⚖️ **Weighted** | Distributes requests based on configured channel weights |

> 💡 **Example**: Create a group named `gpt-4o`, add multiple providers' GPT-4o channels to it, then access all channels via a unified `model: gpt-4o`.

---

### 💰 Price Management

Manage model pricing information in the system with **per-channel pricing support**.

**Architecture:**

The system uses a **one-to-many** relationship architecture:
- **Channel Type** → **Channels** → **Models**
- Each channel can have independent model pricing
- Same model name can exist across different channels with different prices

**Data Sources:**

- The system periodically syncs model pricing data from [models.dev](https://github.com/sst/models.dev)
- When creating a channel, if the channel contains models not in models.dev, the system automatically creates pricing information for those models, allowing users to set prices manually
- Manual creation of models that exist in models.dev is also supported for custom pricing

**Price Priority:**

| Priority | Source | Description |
|:--------:|--------|-------------|
| 🥇 High | Channel-Specific Price | Prices configured for specific channel-model combinations |
| 🥈 Medium | Default Price | User-defined default prices in price management page |
| 🥉 Low | models.dev | Auto-synced upstream default prices |

**Key Features:**

- 🎯 **Channel-Based Organization** - View and manage models organized by channel tabs
- 💱 **Independent Pricing** - Set different prices for the same model across different channels
- 🔄 **Automatic Cost Calculation** - System automatically uses channel-specific prices when calculating request costs
- 📊 **Per-Channel Statistics** - Track token usage and costs separately for each channel

> 💡 **Example**: You can have `claude-sonnet-4` at $3.00/$15.00 on Channel A and $2.50/$12.00 on Channel B, with accurate cost tracking for each.

> 💡 **Tip**: To override a model's default price for a specific channel, navigate to that channel's tab in the price management page and edit the model's pricing.

---

### ⚙️ Settings

Global system configuration.

**Statistics Save Interval (minutes):**

Since the program handles numerous statistics, writing to the database on every request would impact read/write performance. The program uses this strategy:

- Statistics are first stored in **memory**
- Periodically **batch-written** to the database at the configured interval

> ⚠️ **Important**: When exiting the program, use proper shutdown methods (like `Ctrl+C` or sending `SIGTERM` signal) to ensure in-memory statistics are correctly written to the database. **Do NOT use `kill -9` or other forced termination methods**, as this may result in statistics data loss.

---

## 🤝 Acknowledgments

- 🐙 [bestruirui/octopus](https://github.com/bestruirui/octopus) - This project is forked from the original Octopus project
- 🙏 [looplj/axonhub](https://github.com/looplj/axonhub) - The LLM API adaptation module in this project is directly derived from this repository
- 📊 [sst/models.dev](https://github.com/sst/models.dev) - AI model database providing model pricing data

