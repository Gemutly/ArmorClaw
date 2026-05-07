# browser-service

> **DEPRECATED: Legacy Browser Backend**
>
> This service implements the legacy Playwright-based browser backend. It remains
> supported as a fallback but is **deprecated** in favor of the [Jetski CDP
> proxy](../jetski/README.md).
>
> To switch to the modern backend, set:
>
> ```
> ARMORCLAW_BROWSER_BACKEND=jetski
> ```
>
> The legacy backend (`ARMORCLAW_BROWSER_BACKEND=legacy`) will be removed in a
> future version.

## Overview

Legacy browser automation service built on Playwright. Provides HTTP API for
browser operations used by the ArmorClaw Bridge.

## Usage

```bash
# Build
npm install && npm run build

# Run (Docker recommended — see Dockerfile)
node dist/index.js
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP listen port | `3002` |
| `BROWSER_WS_ENDPOINT` | Playwright browser WebSocket endpoint | — |

## API

See `src/types.ts` for request/response types.
