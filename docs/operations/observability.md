# Observability

The controller emits OpenTelemetry traces, metrics, and logs when `otel.enabled: true` in `config.yaml`. All three signals are opt-in and disabled by default; no OTLP traffic leaves the host until configured.

## Quick start

```yaml
# config.yaml (snippet)
otel:
  enabled: true
  service_name: "coolledux-controller"
  endpoint: "otel-gateway-collector.observability.svc.cluster.local:4317"
  protocol: "grpc"
  insecure: true
  resource_attributes:
    deployment.environment: "homelab"
```

Boot the service; the logs should report:

```
INFO opentelemetry enabled endpoint=... protocol=grpc traces=true metrics=true logs=true
```

If the endpoint is unreachable, the SDK buffers in memory and retries; the service itself stays healthy.

## Config reference

| Field | Default | Notes |
|---|---|---|
| `otel.enabled` | `false` | Master switch. When false, every signal uses OTel no-op providers — zero network traffic, zero goroutines. |
| `otel.service_name` | `"coolledux-controller"` | Populates `service.name` resource attribute (what shows up in Tempo / Grafana dropdowns). |
| `otel.service_version` | Go build info `Main.Version` | Overridable. |
| `otel.endpoint` | *(none)* | **Required when enabled.** Typical forms: `host:4317` (gRPC), `host:4318` or `http://host:4318` (HTTP). A scheme prefix (`http://` / `https://`) is accepted and controls TLS. |
| `otel.protocol` | `"grpc"` | `grpc` or `http`. |
| `otel.insecure` | `true` | Set `false` for TLS with the system cert pool. A `https://` endpoint scheme also turns TLS on regardless of this flag. |
| `otel.timeout` | `10s` | Per-request export timeout. |
| `otel.headers` | `{}` | Arbitrary `string: string` headers, e.g. `authorization: "Bearer ..."` for hosted backends. |
| `otel.resource_attributes` | `{}` | Extra resource attrs; merged with `service.name`/`service.version`. |
| `otel.traces.enabled` | `true` | Controlled independently — set `false` to keep metrics + logs without emitting spans. |
| `otel.traces.endpoint` / `.protocol` | *inherits* | Override if traces go to a different collector than the rest. |
| `otel.metrics.enabled` | `true` | Same per-signal toggle. |
| `otel.metrics.interval` | `60s` | Metric export cadence (PeriodicReader). |
| `otel.metrics.runtime` | `true` | Include Go runtime metrics (heap, goroutines, GC). |
| `otel.logs.enabled` | `true` | Bridges `log/slog` records into OTel logs. Stdout output is preserved — records go to both. |

All fields are also settable via `COOLLEDUX_OTEL_*` env vars (Viper handles the mapping; dots become underscores).

## Homelab target cheatsheet

The `homelab` k3s cluster in the `observability` namespace runs:

| Component | Service DNS | Accepts |
|---|---|---|
| DaemonSet collector | `otel-daemonset-collector.observability.svc.cluster.local` | OTLP gRPC `:4317`, OTLP HTTP `:4318`, Zipkin `:9411` |
| Gateway collector | `otel-gateway-collector.observability.svc.cluster.local` | Same ports |
| Tempo | *(via gateway)* | Traces arrive over OTLP from the collector |
| Prometheus | *(via gateway)* | Metrics arrive via `prometheusremotewrite` |
| Loki | *(via gateway)* | Logs arrive over OTLP HTTP at `/otlp` |
| Grafana | `grafana.home.liskl.com` | Single pane of glass |

Both collectors accept plain OTLP with no auth and `insecure: true`. Neither is externally exposed yet — wire an HTTPRoute on kgateway before pointing non-cluster workloads at them.

### Picking gateway vs daemonset

- **Pod in the cluster** → point at the local node's DaemonSet (`otel-daemonset-collector...`). Lowest latency, drops on node failure don't cascade.
- **Single consumer outside the cluster** → use the gateway once it's exposed.

## Finding your telemetry in Grafana

1. Open `https://grafana.home.liskl.com`.
2. **Traces:** Explore → select Tempo → search `{service.name="coolledux-controller"}`. Filter by span name (`ble.send`, `controller.program_upload`, `http.server.request`, etc.).
3. **Logs:** Explore → select Loki → `{service_name="coolledux-controller"}`. slog attributes like `device`, `mac` become structured labels.
4. **Metrics:** Explore → select Prometheus → browse under `coolledux_*` (BLE, program uploads, MQTT) and `http_server_*` (from `otelfiber`).

## What's instrumented

**HTTP (automatic via `otelfiber`):** every REST request produces a span (`http.server.request`) with `http.method`, `http.route`, `http.status_code`, latency, request/response body size.

**BLE:**
- `ble.connect` span per connection attempt, attrs `ble.address`
- `ble.send` span wrapping every framed send, attrs `ble.bytes.total`, `ble.chunks`, `ble.chunk_size`
- Counter `coolledux.ble.bytes.out`
- Histogram `coolledux.ble.send.duration` (seconds)
- Counter `coolledux.ble.reconnects.total`

**Controller:**
- Top-level spans on public methods: `controller.set_power`, `controller.set_brightness`, `controller.display_text`, `controller.display_image`, `controller.display_gif`
- `controller.program_upload` span with child spans `program.crc`, `program.compress`, `program.chunks`
- Histogram `coolledux.program.upload.duration` (seconds, label `outcome`)
- Histogram `coolledux.program.upload.bytes` (compressed size, label `compressed`)
- Counter `coolledux.program.uploads.total` (label `outcome`)

**MQTT:**
- `mqtt.connect` span
- Counter `coolledux.mqtt.messages.out` (label `messaging.destination.name`)

**Runtime (when `otel.metrics.runtime: true`):** `process.runtime.go.*` — goroutines, heap, GC pauses, memory.

## Disabling verification

Confirm "off by default" with tcpdump:

```bash
# In one shell, run with the unchanged example config:
go run ./cmd/coolledux-controller --config config.example.yaml &
# In another:
sudo tcpdump -i any -n 'port 4317 or port 4318' -c 20
# Exercise some endpoints:
curl -s localhost:8080/health >/dev/null
curl -s -XPOST -d '{"state":"on"}' -H 'content-type: application/json' \
  localhost:8080/device/010000fba416/power
# tcpdump should show zero packets.
```

## Local smoke test (no cluster)

Run [`otel-tui`](https://github.com/ymtdzzz/otel-tui) next to the service:

```bash
otel-tui &  # listens on :4317 gRPC
cat > /tmp/otel.yaml <<'EOF'
otel:
  enabled: true
  endpoint: localhost:4317
  protocol: grpc
  insecure: true
EOF
go run ./cmd/coolledux-controller --config /tmp/otel.yaml &
curl -s localhost:8080/health
# otel-tui should show the http.server.request span.
```

## Precedence rules

1. YAML config (`config.yaml`)
2. `COOLLEDUX_*` env vars (Viper override)
3. Standard `OTEL_*` env vars (the SDK reads these natively when options are left unset)
4. SDK built-in defaults

So a production deployment can override any setting without a config-file change by setting, e.g., `COOLLEDUX_OTEL_ENDPOINT=otel.internal:4317`.
