# Sandbox Reverse Tunnel — Design Spec

**Date:** 2026-03-26  
**Status:** Approved

---

## Problem

Sandbox instances run inside the yuanrong cloud platform. User code
executing in a sandbox cannot directly reach services on the user's local
machine, for example a local HTTP API at `192.168.3.45:8000`. We need a
reverse tunnel so sandbox code can transparently call local services through
a WebSocket-based bidirectional channel.

---

## Goals

- Sandbox code calls a local proxy URL
  (`http://127.0.0.1:{portB}/...`) and transparently reaches a local service
- Supports HTTP, HTTPS, and WebSocket proxying
- Single API call from the user (`yr.sandbox.create(upstream=...)`) with no
  separate server to start
- Reuses existing Traefik/JWT auth — no new auth layer
- Concurrent HTTP and WebSocket requests

---

## Architecture

```text
[Local Machine]
  yr.sandbox.create(upstream="192.168.3.45:8000")
    ├── creates sandbox in cloud (Port A + Port B)
    └── starts TunnelClient in background thread
              └── connects to Port A via WSS (through Traefik)

  Local Service C: 192.168.3.45:8000

         ▲ HTTP / HTTPS / WS
         │
   TunnelClient (local, background thread)
         │ WSS long-lived connection
         ▼
[Cloud Sandbox]
  Port A (:8765)  0.0.0.0   WS   ← Traefik registered, tunnel endpoint
  Port B (:8766)  127.0.0.1 HTTP ← internal proxy for sandbox code

  sandbox code:
    curl http://127.0.0.1:8766/api/data
         ↓ Port B receives
         ↓ sends request frame over WS tunnel (Port A)
         ↓ awaits response frame
         ↓ returns HTTP response
```

---

## Port Assignments

- Port B: `proxy_port` (default `8766`)
  Bind: `127.0.0.1`
  Protocol: HTTP/WS
  Purpose: user-facing proxy for sandbox code; user can override it with
  `proxy_port`
- Port A: `proxy_port - 1` (default `8765`)
  Bind: `0.0.0.0`
  Protocol: WebSocket
  Purpose: internal tunnel endpoint that the SDK reaches through Traefik;
  derived automatically and not user-facing

Port B binds only on loopback — not reachable from outside the sandbox.

---

## Wire Protocol

All messages are JSON frames sent over the WebSocket tunnel (Port A).  
Both HTTP and WebSocket proxy use the same `id`-based concurrent dispatch.

### HTTP Request / Response

```json
// sandbox → TunnelClient
{
  "type": "http_req",
  "id": "uuid-1234",
  "method": "POST",
  "path": "/api/v1/data",
  "headers": {"Content-Type": "application/json"},
  "body": "<base64-encoded>"
}

// TunnelClient → sandbox
{
  "type": "http_resp",
  "id": "uuid-1234",
  "status": 200,
  "headers": {"Content-Type": "application/json"},
  "body": "<base64-encoded>"
}
```

### WebSocket Proxy

```json
// open WS channel
{"type": "ws_connect",   "id": "chan-abc", "path": "/ws/stream"}
{"type": "ws_connected", "id": "chan-abc"}               // ack from client

// bidirectional messages
{"type": "ws_message",   "id": "chan-abc", "data": "...", "binary": false}

// close
{"type": "ws_close",     "id": "chan-abc", "code": 1000, "reason": "done"}
```

### Error Frame

```json
{"type": "error", "id": "uuid-1234", "message": "connection refused"}
```

---

## Components

### `tunnel_server.py` (runs in sandbox)

```text
TunnelServer
  ├── _ws_server      asyncio websockets server on Port A (:8765)
  │     └── _handle_tunnel_conn()   one SDK connection at a time
  │           └── _dispatch_frame() routes incoming frames to pending futures
  └── _http_server    aiohttp server on Port B (:8766, loopback only)
        ├── _handle_http()    HTTP → http_req frame → await future →
        │                     HTTP response
        └── _handle_ws()      WS Upgrade → ws_connect frame → relay messages
```

**Concurrency model:**

- Port A runs one long-lived SDK connection; frames dispatched by `id`
- Port B: each incoming request/WS gets a unique `id`, sends frame, awaits `asyncio.Future`
- `_dispatch_frame()` resolves the correct future when a response/message arrives

### `tunnel_client.py` (runs locally, background thread)

```text
TunnelClient
  ├── connect(tunnel_url)    connects WebSocket to Port A
  ├── _recv_loop()           dispatches incoming frames:
  │     ├── http_req   → httpx request to upstream → http_resp frame back
  │     └── ws_connect → open WS to upstream → relay ws_message / ws_close
  └── ws_channels: dict[id, WebSocketClientProtocol]
```

### `sandbox.py` extensions

```python
def create(upstream: str = None, proxy_port: int = 8766, ...) -> SandBox:
    # Port B = proxy_port (user-facing, loopback-only)
    # Port A = proxy_port - 1 (WS tunnel endpoint, Traefik-registered)
    tunnel_port = proxy_port - 1
    opt = yr.InvokeOptions()
    opt.port_forwardings = [
        yr.PortForwarding(port=tunnel_port),  # Port A only; Port B is loopback
    ]
    sb = SandBoxInstance.options(opt).invoke()
    sb.execute_bg(
        f"python -m yr.sandbox.tunnel_server "
        f"--ws-port {tunnel_port} --http-port {proxy_port}"
    )
    if upstream:
        client = TunnelClient(upstream)
        tunnel_ws_url = sb.get_gateway_url(tunnel_port)
        client.start(tunnel_ws_url)   # background thread
        sb._tunnel_client = client
    return SandBox(sb, proxy_port=proxy_port)

class SandBox:
    def get_tunnel_url(self) -> str:
        return f"http://127.0.0.1:{self._proxy_port}"

    def close(self):
        if self._tunnel_client:
            self._tunnel_client.stop()
        self._instance.terminate()
```

---

## User API

```python
import yr

# proxy_port = Port B (default 8766, user-facing)
# Port A = proxy_port - 1 (default 8765, internal, Traefik-registered)
sb = yr.sandbox.create(
    upstream="192.168.3.45:8000",
    proxy_port=8766,   # optional, default 8766
)

# Get the internal proxy URL for use inside sandbox
url = sb.get_tunnel_url()  # → "http://127.0.0.1:8766"

# Sandbox code accesses local service C transparently
result = yr.get(sb.execute(f"curl {url}/api/v1/data"))
print(result)

sb.close()
```

---

## Dependencies

| Library      | Side            | Use                            |
| ------------ | --------------- | ------------------------------ |
| `websockets` | sandbox + local | WS server (Port A) and client  |
| `aiohttp`    | sandbox         | HTTP server (Port B)           |
| `httpx`      | local           | async HTTP client to upstream  |
| `asyncio`    | both            | concurrency                    |

All are pure Python, no new system dependencies.

---

## Limitations & Future Work

- One SDK connection at a time (Port A). If SDK disconnects, in-flight
  requests get error frames; TunnelClient reconnects automatically.
- HTTPS upstream (`https://...`): TunnelClient forwards as-is with no
  MITM/certificate inspection.
- WS-over-WS: supported via channel multiplexing, but each channel is independent.
- No request size limit in v1 — large bodies buffered fully in memory.
