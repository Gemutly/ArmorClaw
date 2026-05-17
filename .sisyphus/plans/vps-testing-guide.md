# VPS Testing Guide for ArmorClaw

> **Version**: v4.11.0 | **Last Updated**: 2026-03-15
>
> Comprehensive guide for installing, running, and testing ArmorClaw on a VPS using Element X for Matrix chatroom-based feature validation.

---

## Table of Contents

1. [VPS Preparation](#1-vps-preparation)
2. [Installation](#2-installation)
3. [Running the Bridge](#3-running-the-bridge)
4. [Connecting to Element X](#4-connecting-to-element-x)
5. [Feature Testing via Matrix Chatroom](#5-feature-testing-via-matrix-chatroom)
6. [Troubleshooting](#6-troubleshooting)
7. [Verification Checklist](#7-verification-checklist)

---

## 1. VPS Preparation

### 1.1 System Requirements

| Requirement | Minimum | Recommended |
|-------------|---------|-------------|
| CPU | 1 core | 2+ cores |
| RAM | 2 GB | 4 GB |
| Disk | 10 GB | 20 GB |
| OS | Linux (Ubuntu 20.04+) | Ubuntu 22.04 |
| Docker | 24.0+ | Latest |

### 1.2 Pre-Installation Checklist

```bash
# Check system resources
free -h
df -h

# Verify Docker is installed
docker --version
docker compose version

# Verify Go (for source builds)
go version  # Should be 1.24+

# Check for SQLCipher dev libraries (for source builds)
dpkg -l | grep sqlcipher
```

### 1.3 Install Prerequisites

```bash
# Update system
sudo apt-get update && sudo apt-get upgrade -y

# Install Docker (if not present)
curl -fsSL https://get.docker.com | bash
sudo usermod -aG docker $USER

# Install required utilities
sudo apt-get install -y curl wget qrencode socat

# Install SQLCipher dev libraries (for source builds)
sudo apt-get install -y libsqlcipher-dev sqlcipher

# Logout and login to apply Docker group
```

### 1.4 Prepare API Keys

Add API keys to your shell profile (keys are never persisted to disk):

```bash
# Edit .zshrc or .bashrc
nano ~/.zshrc  # or ~/.bashrc

# Add these lines:
export OPENROUTER_API_KEY=sk-or-v1-xxx   # Recommended - access to many providers
export ZAI_API_KEY=xxx                   # xAI (Grok)
export OPEN_AI_KEY=xxx                   # OpenAI

# Reload shell profile
source ~/.zshrc  # or source ~/.bashrc
```

---

## 2. Installation

### 2.1 Quick Install (Recommended)

The all-in-one installer with Matrix auto-install:

```bash
curl -fsSL https://raw.githubusercontent.com/Gemutly/ArmorClaw/main/deploy/install.sh | bash
```

**11-Step Installation Flow:**
1. **Prerequisites Check** — Docker, Docker Compose, memory (2GB+), disk (2GB+), Go version, qrencode
2. **Build/Install Bridge** — Auto-downloads prebuilt binary or builds from source
3. **Create System User** — Creates armored user (armorclaw) with no login shell
4. **Generate Configuration** — Creates config.toml with secure defaults
5. **Initialize Keystore** — Creates SQLCipher encrypted keystore
6. **Setup Systemd Service** — Creates `/etc/systemd/system/armorclaw-bridge.service`
7. **Start Bridge** — Starts bridge service, waits for socket at `/run/armorclaw/bridge.sock`
8. **Verify Health** — Checks all components (config, data, run, binary, socket, service)
9. **Matrix Setup** — Optional but recommended: installs Conduit homeserver
10. **API Key Setup** — Optional: adds API key via RPC
11. **Provisioning QR** — Generates QR code for ArmorChat connection

### 2.2 Non-Interactive (CI/CD)

```bash
# Set environment variables
export OPENROUTER_API_KEY=sk-or-v1-xxx
export ARMORCLAW_SERVER_NAME=192.168.1.50
export CONDUIT_VERSION=v1.0.0

# Run installer
curl -fsSL https://raw.githubusercontent.com/Gemutly/ArmorClaw/main/deploy/install.sh | bash
```

### 2.3 Full Installer (Stage-1)

```bash
curl -fsSL https://raw.githubusercontent.com/Gemutly/ArmorClaw/main/deploy/installer-v5.sh | bash
```

Features:
- Lockfile protection (flock) prevents parallel installs
- Docker daemon readiness check (20s timeout)
- Persistent logging to `/var/log/armorclaw/install.log`
- Docker Compose auto-detection

### 2.4 Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENROUTER_API_KEY` | OpenRouter API key (recommended) | (from .zshrc) |
| `OPEN_AI_KEY` | OpenAI API key | (from .zshrc) |
| `ZAI_API_KEY` | xAI API key | (from .zshrc) |
| `ARMORCLAW_SERVER_NAME` | Server hostname or IP | auto-detected |
| `ARMORCLAW_ADMIN_PASSWORD` | Admin password | auto-generated |
| `CONDUIT_VERSION` | Conduit version | latest |
| `CONDUIT_IMAGE` | Custom image | matrixconduit/matrix-conduit:latest |
| `INSTALL_MODE` | quick \| matrix | quick |

---

## 3. Running the Bridge

### 3.1 Start Bridge Service

```bash
# Start via systemd
sudo systemctl start armorclaw-bridge

# Enable auto-start on boot
sudo systemctl enable armorclaw-bridge

# Check status
sudo systemctl status armorclaw-bridge
```

### 3.2 Verify Bridge Health

```bash
# Check socket exists
sudo test -S /run/armorclaw/bridge.sock && echo "Bridge socket OK"

# Test RPC connection
echo '{"jsonrpc":"2.0","method":"status","id":1}' | \
  socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

# Check logs
journalctl -u armorclaw-bridge -n 50 -f
```

### 3.3 Bridge Logs

```bash
# View recent logs
journalctl -u armorclaw-bridge -n 100

# Follow logs in real-time
journalctl -u armorclaw-bridge -f

# Filter for errors
journalctl -u armorclaw-bridge -p err
```

### 3.4 Matrix/Conduit Service

```bash
# Check Matrix container
docker ps | grep conduit

# Test Matrix API
curl http://localhost:6167/_matrix/client/versions

# Check Matrix logs
docker logs armorclaw-conduit -f
```

### 3.5 Tunnel Setup (Optional)

**Cloudflare Quick Tunnel** (Free, instant):
```bash
docker run -d \
  --name armorclaw-tunnel \
  cloudflare/cloudflared:latest \
  tunnel --url http://host.docker.internal:6167
```

**ngrok Tunnel** (Free account needed):
```bash
# Install ngrok
curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc
echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list
sudo apt-get update -qq && sudo apt-get install -y ngrok

# Start
ngrok http 6167
```

---

## 4. Connecting to Element X

### 4.1 Install Element X

- **Android**: [Google Play](https://play.google.com/store/apps/details?id=io.element.x)
- **iOS**: [App Store](https://apps.apple.com/app/element-x/id1632605858)
- **Desktop**: [element.io](https://element.io/download)

### 4.2 Create Account on Your Matrix Server

1. Open Element X
2. Tap **Edit** next to the server URL
3. Enter your VPS URL: `http://YOUR_VPS_IP:6167`
4. Tap **Continue**
5. Create a new account:
   - Username: `testuser`
   - Password: `[secure password]`
6. Complete registration

### 4.3 Join Bridge Room

The bridge creates a control room automatically. If not:

1. Tap **+** (new chat)
2. Tap **Explore rooms**
3. Search for `#bridge:your-server`
4. Join the room

### 4.4 Verify Connection

Send a test message in the bridge room:
```
!secretary help
```

You should see the help response with all available commands.

---

## 5. Feature Testing via Matrix Chatroom

### 5.1 Secretary Commands

#### Help and Status

```
!secretary help
```

Expected output: List of all secretary commands including workflow, contact, trust, and template operations.

#### List Available Templates

```
!secretary list templates
```

Expected output: List of workflow templates available for use.

#### List Agents

```
!secretary list agents
```

Expected output: List of active secretary agents.

---

### 5.2 Contact Management (Rolodex)

#### Create Contact

```
!secretary contact create "John Doe" company="Acme Corp" relationship="business" phone="555-1234" email="john@acme.com" notes="Met at conference"
```

Expected output: Contact ID and confirmation message.

#### List Contacts

```
!secretary contact list
```

Expected output: Table of all contacts with ID, name, company, relationship.

#### Search Contacts

```
!secretary contact search "John"
```

Expected output: Matching contacts with "John" in name or company.

#### Get Contact by ID

```
!secretary contact get <contact_id>
```

Expected output: Full contact details.

#### Update Contact

```
!secretary contact update <contact_id> notes="Updated notes"
```

Expected output: Confirmation of update.

#### Delete Contact

```
!secretary contact delete <contact_id>
```

Expected output: Confirmation of deletion.

---

### 5.3 WebDAV Skill Testing

> **Prerequisite**: WebDAV server must be configured and accessible.

#### List WebDAV Files

Via secretary command (if implemented):
```
!secretary webdav list http://your-webdav-server/
```

Via skill call (internal):
- The WebDAV skill supports: `list`, `get`, `put`, `delete` operations

#### Upload File to WebDAV

```
!secretary webdav put http://your-webdav-server/docs/test.txt "Hello World"
```

Expected output: Confirmation of upload.

#### Download File from WebDAV

```
!secretary webdav get http://your-webdav-server/docs/test.txt
```

Expected output: File contents.

---

### 5.4 Calendar Skill Testing (CalDAV)

> **Prerequisite**: CalDAV server (e.g., Radicale) must be configured.

#### List Calendars

```
!secretary calendar list
```

Expected output: List of available calendars.

#### Create Event

```
!secretary calendar add "Team Meeting" start="2026-03-20T10:00:00" end="2026-03-20T11:00:00"
```

Expected output: Event ID and confirmation.

#### Get Events

```
!secretary calendar get <calendar_id>
```

Expected output: List of events with dates and times.

#### Delete Event

```
!secretary calendar delete <event_id>
```

Expected output: Confirmation of deletion.

---

### 5.5 Workflow Management

#### Create Workflow from Template

```
!secretary create workflow <template_id>
```

Expected output: Workflow ID and status.

#### Start Workflow

```
!secretary start workflow <workflow_id>
```

Expected output: Workflow started confirmation.

#### List Workflows

```
!secretary list workflows
```

Expected output: Table of all workflows with status.

#### Workflow Status

```
!secretary workflow status <workflow_id>
```

Expected output: Detailed workflow status and progress.

#### Cancel Workflow

```
!secretary workflow cancel <workflow_id>
```

Expected output: Cancellation confirmation.

---

### 5.6 BlindFill and Learn Website

#### Learn Website (Form Discovery)

```
!secretary learn website https://example.com/form
```

Expected output: Discovered form fields and draft mapping ID.

#### Review Mapping Draft

```
!secretary review mapping <draft_id>
```

Expected output: Field mappings for review.

#### Confirm Mapping

```
!secretary confirm mapping <draft_id> field1="value1" field2="value2"
```

Expected output: Confirmation of mapping.

#### Execute BlindFill

```
!secretary run blindfill <template> https://example.com/form
```

Expected output: Form filled confirmation (with dry-run mode available).

---

### 5.7 Trust Policies

#### List Trust Policies

```
!secretary trust list
```

Expected output: List of configured trust policies.

#### Create Trust Policy

```
!secretary trust create "auto_approve_low_risk"
```

Expected output: Policy ID and configuration options.

#### Revoke Trust Policy

```
!secretary trust revoke <policy_id>
```

Expected output: Revocation confirmation.

---

### 5.8 AI Provider Commands (Catwalk)

#### Show AI Help

```
/ai
```

Expected output: Available AI commands.

#### List Providers

```
/ai providers
```

Expected output: List of 12+ providers (openai, anthropic, google, xai, etc.).

#### List Models for Provider

```
/ai models openai
```

Expected output: Available OpenAI models.

#### Switch Provider/Model

```
/ai switch openai gpt-4o
```

Expected output: Provider switched confirmation.

#### Show Status

```
/ai status
```

Expected output: Current provider, model, and configuration.

---

### 5.9 Agent Creation

#### Create Agent

```
!agent create name="Researcher" skills="web_browsing"
```

Expected output: Agent created, room invitation sent.

#### Interact with Agent

In the agent's dedicated room:
```
Research the best restaurants in NYC for a birthday dinner
```

Expected output: Agent begins research, provides progress updates, and delivers results.

---

### 5.10 Three-Way Consent Testing

> **Prerequisite**: Agent requests PII access, triggering consent flow.

#### Trigger Consent Request

When an agent requests access to PII fields (e.g., credit card, address), the Three-Way Consent system:

1. Creates a Matrix room with: User + Agent + Bridge
2. Sends a consent request event with requested fields
3. Waits for approval via reaction or message

#### Approve/Reject Consent

In the consent room:
```
!approve <request_id>
```
or
```
!reject <request_id> "Reason for rejection"
```

Expected output: Consent status updated, agent proceeds or is blocked.

---

### 5.11 Browser Skills Testing

> **Prerequisite**: Agent with browser skills active.

The bridge supports 21 browser DevTools MCP skills:

**Safe Primitives (No approval):**
- `navigate` — Navigate to URL
- `click` — Click element
- `fill` — Fill form field
- `screenshot` — Take screenshot
- `snapshot` — Get page snapshot

**Guarded Skills (Require approval):**
- `eval_privileged` — Execute JavaScript
- `fill_with_pii` — Fill with PII data
- `network_inspect` — Inspect network

Test via agent:
```
Navigate to https://example.com and take a screenshot
```

Expected output: Screenshot URL and page summary.

---

## 6. Troubleshooting

### 6.1 Bridge Not Starting

```bash
# Check service status
sudo systemctl status armorclaw-bridge

# Check for port conflicts
sudo netstat -tlnp | grep 8443

# Check socket
ls -la /run/armorclaw/

# Check logs
journalctl -u armorclaw-bridge -n 100 --no-pager
```

### 6.2 Matrix Connection Issues

```bash
# Verify Matrix is running
docker ps | grep conduit

# Check Matrix API
curl -v http://localhost:6167/_matrix/client/versions

# Check bridge user registration
curl -X GET "http://localhost:6167/_matrix/client/v3/profile/@bridge:$(hostname)"

# Restart Matrix
docker restart armorclaw-conduit
```

### 6.3 Socket Connection Refused

```bash
# Verify socket exists
test -S /run/armorclaw/bridge.sock && echo "OK" || echo "Missing"

# Check permissions
ls -la /run/armorclaw/bridge.sock

# Restart bridge
sudo systemctl restart armorclaw-bridge

# Wait for socket
sleep 5 && test -S /run/armorclaw/bridge.sock && echo "Ready"
```

### 6.4 API Key Not Recognized

```bash
# Verify environment variable is set
echo $OPENROUTER_API_KEY

# Reload shell profile
source ~/.zshrc

# Check bridge logs for key errors
journalctl -u armorclaw-bridge | grep -i "api key"
```

### 6.5 Consent Room Not Created

```bash
# Check Three-Way Consent logs
journalctl -u armorclaw-bridge | grep -i "consent"

# Verify PII detection is working
# Check pii package logs
journalctl -u armorclaw-bridge | grep -i "pii"
```

### 6.6 Skills Not Working

```bash
# Check skill registration
grep -r "RegisterSkill" /var/log/armorclaw/

# Verify skill is in registry
# Connect to bridge and query skill list
echo '{"jsonrpc":"2.0","method":"skills.list","id":1}' | \
  socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
```

---

## 7. Verification Checklist

### 7.1 Installation Verification

```bash
# Run verification script
sudo ./deploy/verify-bridge.sh

# Check prerequisites
sudo ./deploy/check-prerequisites.sh
```

### 7.2 Service Health

- [ ] Bridge service running: `systemctl is-active armorclaw-bridge`
- [ ] Bridge socket exists: `test -S /run/armorclaw/bridge.sock`
- [ ] Matrix container running: `docker ps | grep conduit`
- [ ] Matrix API responding: `curl http://localhost:6167/_matrix/client/versions`
- [ ] Registration disabled: `grep allow_registration /etc/conduit.toml` shows `false`

### 7.3 Feature Checklist

- [ ] Secretary help command works: `!secretary help`
- [ ] Contact creation works: `!secretary contact create "Test"`
- [ ] Contact listing works: `!secretary contact list`
- [ ] WebDAV skill registered (check logs)
- [ ] Calendar skill registered (check logs)
- [ ] Workflow creation works: `!secretary create workflow <template>`
- [ ] AI provider switching works: `/ai switch openai gpt-4o`
- [ ] Agent creation works: `!agent create name="Test"`
- [ ] Three-way consent triggers for PII requests

### 7.4 Security Verification

- [ ] SQLCipher keystore encrypted
- [ ] API keys from environment (not persisted)
- [ ] Matrix E2EE enabled
- [ ] Registration disabled after setup
- [ ] Agent containers hardened (check `docker inspect`)

### 7.5 Test Suite

```bash
# Run CI/CD test suite
cd /home/mink/src/armorclaw-omo/tests

# E2E tests
bash test-e2e.sh

# Matrix integration
bash test-matrix-integration.sh

# Security tests
bash test-exploits.sh
bash test-secrets.sh

# Element X flow
bash test-element-x-flow.sh
```

---

## Appendix A: Quick Reference Commands

### Bridge Commands

```bash
# Start bridge
sudo systemctl start armorclaw-bridge

# Stop bridge
sudo systemctl stop armorclaw-bridge

# Restart bridge
sudo systemctl restart armorclaw-bridge

# Bridge status
sudo systemctl status armorclaw-bridge

# Bridge logs
journalctl -u armorclaw-bridge -f

# Test RPC
echo '{"jsonrpc":"2.0","method":"status","id":1}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
```

### Matrix Commands

```bash
# Matrix container status
docker ps | grep conduit

# Matrix logs
docker logs armorclaw-conduit -f

# Test Matrix API
curl http://localhost:6167/_matrix/client/versions

# Restart Matrix
docker restart armorclaw-conduit
```

### Docker Commands

```bash
# List containers
docker ps -a

# Container logs
docker logs <container_name> -f

# Restart container
docker restart <container_name>

# Container stats
docker stats --no-stream
```

---

## Appendix B: Available Skills Reference

### Core Skills (15)

| Skill | Description | Operations |
|-------|-------------|------------|
| `calendar` | CalDAV client | list, create, get, delete events |
| `webdav` | WebDAV client | list, get, put, delete files |
| `data_analyze` | Data analysis | Analyze data from sources |
| `web_extract` | Web extraction | Extract structured data |
| `web_search` | Web search | Search for information |
| `email_send` | Email sending | Send emails |
| `slack_message` | Slack messaging | Send Slack messages |
| `file_read` | File reading | Read files from filesystem |
| `policy` | Trust policies | Policy enforcement |
| `allowlist` | Security allowlist | Allowlist management |
| `ssrf` | SSRF validation | URL validation |
| `router` | Skill routing | Route to skills |
| `schema` | Skill schemas | Schema definitions |
| `executor` | Skill execution | Execute skills |
| `registry` | Skill registry | Register skills |

### Browser Skills (21)

| Skill | Category | Requires Approval |
|-------|----------|-------------------|
| `navigate` | Safe | No |
| `click` | Safe | No |
| `fill` | Safe | No |
| `wait_for` | Safe | No |
| `screenshot` | Safe | No |
| `snapshot` | Safe | No |
| `list_pages` | Safe | No |
| `select_page` | Safe | No |
| `resize` | Safe | No |
| `emulate` | Safe | No |
| `extract_page` | Workflow | No |
| `login_assist` | Workflow | No |
| `form_submit` | Workflow | No |
| `upload_document` | Workflow | No |
| `trace_performance` | Workflow | No |
| `eval_privileged` | Guarded | **Yes** |
| `network_inspect` | Guarded | **Yes** |
| `console_inspect` | Guarded | **Yes** |
| `lighthouse_audit` | Guarded | **Yes** |
| `memory_snapshot` | Guarded | **Yes** |
| `fill_with_pii` | Guarded | **Yes** |

---

## Appendix C: AI Providers Reference

| Provider | ID | Protocol | Description |
|----------|----|----------|-------------|
| OpenAI | `openai` | openai | GPT-4o, o1, etc. |
| Anthropic | `anthropic` | anthropic | Claude 3.5 Sonnet/Opus |
| Zhipu AI | `zhipu` | openai | api.z.ai (aliases: zai, glm) |
| DeepSeek | `deepseek` | openai | DeepSeek R1, V3 |
| Moonshot | `moonshot` | openai | Moonshot/Kimi |
| Google | `google` | openai | Gemini 1.5 Pro/Flash |
| xAI | `xai` | openai | Grok-1, Grok-2 |
| OpenRouter | `openrouter` | openai | Multi-provider aggregator |
| Groq | `groq` | openai | Ultra-fast inference |
| NVIDIA NIM | `nvidia` | openai | NVIDIA-hosted models |
| Cloudflare | `cloudflare` | openai | AI Gateway |
| Ollama | `ollama` | openai | Local models (localhost:11434) |

---

## Appendix D: Test Scripts Reference

| Test Script | Purpose |
|-------------|---------|
| `test-e2e.sh` | End-to-end flow testing |
| `test-element-x-flow.sh` | Element X mobile app flow |
| `test-matrix-integration.sh` | Matrix/Conduit integration |
| `test-exploits.sh` | Exploit attempt detection |
| `test-secrets.sh` | Secret handling validation |
| `test-hardening.sh` | Hardening validation |
| `verify-security.sh` | Security verification |
| `test-quickstart-entrypoint.sh` | Quickstart entrypoint |
| `test-container-setup.sh` | Container setup wizard |
| `test-discovery.sh` | Service discovery |

---

*End of VPS Testing Guide*
