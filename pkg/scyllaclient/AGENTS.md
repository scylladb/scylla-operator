# pkg/scyllaclient

HTTP client for the ScyllaDB REST API (v1 + v2). Wraps the auto-generated swagger client with a host-pool, middleware pipeline, and convenience methods used by controllers and the sidecar.

## Files

| File | Purpose |
|---|---|
| `client.go` | `Client` struct, `NewClient`, transport middleware assembly, most API methods |
| `config.go` | `Config` struct, `DefaultConfig`, `Validate` |
| `model.go` | All model types and helpers (`NodeState`, `NodeStatus`, `OperationalMode`, `Snapshot`, etc.) |
| `client_ping.go` | `Ping` implementation |
| `hostpool.go` | Host-pool middleware (EpsilonGreedy selection, host marking) |
| `timeout.go` | Timeout middleware, `ErrTimeout` |
| `context.go` | Per-request context helpers (`forceHost`, `noRetry`, `customTimeout`) |
| `status.go` | `StatusCodeOf` — extract HTTP status code from error |
| `log.go` | Request/response logging middleware |
| `config_client.go` | `ConfigClient` (v2) for reading Scylla config parameters |

## Construction

```go
// Build with defaults (Scheme=https, Port=10001, Timeout=15s, PoolDecay=30m)
cfg := scyllaclient.DefaultConfig(authToken, "10.0.0.1", "10.0.0.2")
client, err := scyllaclient.NewClient(cfg)

// ConfigClient (v2 API — config reads only)
cc := scyllaclient.NewConfigClient(host, authToken)
```

`DefaultTransport()` sets `TLSClientConfig.InsecureSkipVerify = true` — intentional for the Scylla agent endpoint.

## Transport pipeline (inner → outer)

```
base transport
  → timeout        (per-request deadline; ErrTimeout on expiry)
  → requestLogger  (dump req/resp on status ≥ 400)
  → hostPool       (select host, rewrite URL, mark host on error)
  → auth.AddToken  (Authorization header)
  → fixContentType (force Content-Type: application/json)
```

## Context flags

These are **package-internal** functions used within `pkg/scyllaclient` itself. They are not part of the public API and cannot be called from outside the package.

```go
// (inside pkg/scyllaclient only)

// Target a specific node (bypasses host-pool selection)
ctx = forceHost(ctx, "10.0.0.1")

// Disable retry (used for Ping and Decommission)
ctx = noRetry(ctx)

// Override timeout for long-running ops
ctx = customTimeout(ctx, 24*time.Hour)
```

## Key API method groups

**Node/cluster status**
- `NodesStatusInfo(ctx, host)` → `NodeStatusInfoSlice`
- `NodesStatusAndStateInfo(ctx, host)` → `NodeStatusAndStateInfoSlice`
- `GetLocalHostId(ctx, host, retry)` → `string`
- `GetIPToHostIDMap(ctx, host)` → `map[string]string`
- `GetTokenRing(ctx, host)` → `[]string`
- `ScyllaVersion(ctx)` → `string`
- `OperationMode(ctx, host)` → `OperationalMode`
- `IsNativeTransportEnabled(ctx, host)` → `bool`
- `HasSchemaAgreement(ctx, host)` → `bool`
- `Ping(ctx, host)` → `(time.Duration, error)` — no retry, uses `SystemUptimeMsGet`

**Keyspace/schema**
- `Keyspaces(ctx)` → `[]string`
- `GetSnitchDatacenter(ctx, host)` → `string`

**Snapshots**
- `Snapshots(ctx, host)` → tags `[]string`
- `ListSnapshots(ctx, host)` → `[]*Snapshot`
- `TakeSnapshot(ctx, host, tag, keyspace, tables...)` → `error`
- `DeleteSnapshot(ctx, host, tag)` → `error`

**Maintenance**
- `Cleanup(ctx, host, keyspace)` — 24 h timeout, synchronous
- `StopCleanup(ctx, host)` — stop CLEANUP compaction
- `Drain(ctx, host)` — flush memtables; 5 min timeout
- `Decommission(ctx, host)` — sets `noRetry` automatically

**Config (v2 ConfigClient)**
- `BroadcastRPCAddress`, `BroadcastAddress`, `ReplaceAddressFirstBoot`, `ReplaceNodeFirstBoot`, `ReadRequestTimeoutInMs`

## Key model types

```go
type NodeStatus   string  // NodeStatusUp / NodeStatusDown
type NodeState    string  // NodeStateNormal / Leaving / Joining / Moving
type OperationalMode string // many constants + IsDecommissioned(), IsJoining(), etc.

type NodeStatusInfo struct { ... }
type Snapshot struct { Tag, Keyspace, Table string }
```

## Error handling

```go
// Get HTTP status code from any error shape
code := scyllaclient.StatusCodeOf(err) // returns 0 if no code

// Detect timeout
if errors.Is(err, scyllaclient.ErrTimeout) { ... }
```

`StatusCodeOf` handles: errors with `Code() int`, `GetPayload() *models.ErrorModel`, and `runtime.APIError`.

## Host-pool behavior

- Pool type: `EpsilonGreedy` with `PoolDecayDuration` (default 30 min).
- After each request the middleware marks the host:
  - `err != nil` → `mark(err)` — reduces host score
  - `status 401/403` or `≥ 500` → `mark(errPoolServerError)`
  - otherwise → `mark(nil)` — success
- Use `forceHost(ctx, host)` to bypass pool selection and target a specific node.

## Gotchas

- **`InsecureSkipVerify = true`** — DefaultTransport skips TLS verification. This is intentional for the Scylla agent port but means certificate errors are silently ignored.
- **`fixContentType` middleware** — Scylla sometimes returns `text/plain`; this wrapper forces `application/json` so the generated swagger client can unmarshal.
- **Long-running ops** — `Cleanup` uses a 24 h timeout internally. Don't wrap it in a short context or you'll cut it off. Use `customTimeout` when you need a different value.
- **Decommission** — internally sets `noRetry` because retrying decommission can cause split-brain. Don't retry at the call site.
- **Config timeout** — `Config.Timeout` (default 15 s) applies to all requests unless overridden with `customTimeout`.

## Import aliases

```go
scyllaclient "github.com/scylladb/scylla-operator/pkg/scyllaclient"
```

The generated swagger clients use internal aliases (`scyllaoperations`, `scyllav2models`, etc.) — do not import those directly from callers; use the `Client` methods instead.
