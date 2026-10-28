# connectivity-contracts

The wire contracts every Connectivity component shares: the HTTP payloads devices send, the Kafka message formats, and the topic names.

Part of [Connectivity](https://github.com/spr732/connectivity-platform), a mock edge-to-cloud telemetry system built for practicing production debugging.

```go
import "github.com/spr732/connectivity-contracts/wire"
```

## Compatibility rules

Devices in the field keep running old agent versions for months. The cloud must accept every payload an older agent can still send.

- Add fields. Never rename or remove a field in place.
- New fields must be optional, with a zero value that means "not set".
- A change in a field's meaning is a new field, not an edit.
- Topic names and Kafka message keys are part of the contract.

## Versioning

Services pin this module by version. To change a contract together with a service, work in the shared `go.work` workspace that `connectivity-platform` sets up, then release this module before the service that depends on it.
