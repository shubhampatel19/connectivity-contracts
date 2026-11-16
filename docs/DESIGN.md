# connectivity-contracts: design

This module is the only thing every Connectivity component agrees on. It has no dependencies and no behaviour beyond validation: it defines the bytes that cross a boundary, and nothing else.

## Where each contract crosses a boundary

```
device ──HTTP──▶ site gateway (site B only) ──HTTP──▶ ingest API ──Kafka──▶ consumers
        JSON payloads in wire/*.go                     envelopes and events in wire/*.go
```

| Flow | Device sends | Ingest publishes | Topic (key) | Consumers |
|---|---|---|---|---|
| Registration | `RegisterRequest` → `RegisterResponse` | `DeviceEvent{registered}` | `device.events` (device ID) | telemetry processor |
| Heartbeat | empty body | `DeviceEvent{heartbeat}` | `device.events` (device ID) | telemetry processor |
| Telemetry | `TelemetryBatch` | `TelemetryEnvelope` | `telemetry.raw` (device ID) | telemetry processor |
| Alert | `DeviceAlert` → `AlertAccepted` | `AlertEnvelope` | `device.alerts` (device ID) | case service |
| Log bundle | `BundleInitRequest`, raw chunks, complete → `BundleStatus` | `BundleUploadedEvent` | `bundles.uploaded` (device ID) | telemetry processor, case service |

Every topic is keyed by device ID. That is a contract, not a detail: it puts one device's records on one partition, in order, which the consumers rely on.

## Files

| File | Contents |
|---|---|
| `wire/wire.go` | Registration, heartbeat and telemetry types; `telemetry.raw` and `device.events` |
| `wire/alert.go` | Alert codes, severities, `DeviceAlert`, `AlertEnvelope`; `device.alerts` |
| `wire/bundle.go` | The resumable upload protocol, chunk arithmetic (`ChunkCount`, `ChunkLen`, `Missing`), `BundleUploadedEvent`; `bundles.uploaded` |
| `wire/text.go` | `ValidText` and `ValidMetric`: what free text and names may contain |
| `wire/deadletter.go` | Dead-letter topic naming and headers |

## Design decisions

**Envelopes separate what the device said from what the cloud knows.** `TelemetryEnvelope` and `AlertEnvelope` wrap the device's payload with `received_at`, set by the ingest API. A device's clock can't be trusted, so anything measured in the cloud (ingest-to-store delay, alert-to-decision delay) uses the envelope's time, never the payload's.

**IDs are idempotency keys, assigned at the edge.** `batch_id`, `alert_id` and `bundle_id` are created once on the device and reused on every retry. Every consumer deduplicates on them. If a device generated a new ID per attempt, every retry would be stored twice.

**Validation lives here so every hop applies the same rules.** The ingest API validates on receipt; the case service validates again when it reads from Kafka, against the envelope's `received_at` rather than the current time, so a replayed week-old alert is still valid.

**Text is restricted because of what it can do downstream.** Postgres rejects a NUL byte in a `TEXT` column. A consumer that treats database errors as transient would retry such a record forever and stall every device on that partition. So:

- IDs (`device_id`, `batch_id`, `alert_id`, `bundle_id`, `site_id`) match `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`. They also appear in URL paths and object keys.
- Metric names match `^[a-z][a-z0-9_.]{0,63}$`.
- Free text (alert `message`, device `model` and `firmware`) must be valid UTF-8 with no control characters, and is length-limited.

**Timestamps may be late, never far in the future.** A sample or alert more than `MaxClockSkew` (10 minutes) ahead of the cloud's clock is rejected. Old timestamps are accepted: a gateway draining a backlog delivers hours-old data legitimately.

**The cloud accepts alert codes it doesn't know.** A code only has to look like `subsystem.condition`. Agents can be newer than the cloud, and the case service records unknown codes without opening a case.

**The cloud chooses the chunk size.** `BundleStatus.ChunkSize` comes from the server, and the device splits its bundle accordingly. The size can be tuned for slow links without a fleet upgrade. An upload in progress keeps the size it started with.

**Bundle events may be delivered twice.** A retried completion can publish `BundleUploadedEvent` again. Consumers must treat `bundle_id` as a key.

**Poison records go to a dead-letter topic, with their history.** `DeadLetterTopic(topic)` is `topic + ".dlq"`. A consumer copies the record unchanged and adds `x-dlq-*` headers saying why, from which group, partition and offset.

## Compatibility

Old agents stay in the field for months, so the cloud must accept everything an older agent can still send. Add optional fields; never rename, remove or change the meaning of one. Tightening validation is also a compatibility change: before tightening, check what deployed agents actually send. The current rules accept everything `connectivity-edge-agent` has ever produced (hex IDs, snake-case metric names, single-line messages).

## Releasing a change

1. Change this module, with tests, and merge it.
2. In each service that needs the change, `go get github.com/shubhampatel19/connectivity-contracts@<commit>` and commit `go.mod` and `go.sum`.
3. Deploy consumers before producers when adding a field consumers must understand; deploy the ingest API before agents when adding a field devices send.

For day-to-day work, `make bootstrap` in `connectivity-platform` creates a `go.work` so a contract and a service can be changed together.
