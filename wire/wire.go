// Package wire defines the contracts shared by edge and cloud: HTTP payloads
// and the Kafka message formats. Changing anything here is a cross-fleet
// compatibility decision, because old agents stay in the field for months.
package wire

import (
	"errors"
	"fmt"
	"time"
)

const (
	TopicTelemetry    = "telemetry.raw"
	TopicDeviceEvents = "device.events"

	MaxSamplesPerBatch = 500
	// MaxClockSkew is how far into the future a sample timestamp may be before
	// ingestion rejects it. Device clocks drift; this bounds how much.
	MaxClockSkew = 10 * time.Minute
)

var ErrInvalid = errors.New("invalid payload")

type RegisterRequest struct {
	DeviceID string `json:"device_id"`
	Model    string `json:"model"`
	Firmware string `json:"firmware"`
	SiteID   string `json:"site_id"`
}

type RegisterResponse struct {
	DeviceID string `json:"device_id"`
	Token    string `json:"token"`
}

type Sample struct {
	Metric string    `json:"metric"`
	Value  float64   `json:"value"`
	TS     time.Time `json:"ts"`
}

// TelemetryBatch is the unit of upload. BatchID is assigned once on the device
// and reused on every retry, which is what lets the backend deduplicate.
type TelemetryBatch struct {
	BatchID  string   `json:"batch_id"`
	DeviceID string   `json:"device_id"`
	Samples  []Sample `json:"samples"`
}

func (b TelemetryBatch) Validate(now time.Time) error {
	switch {
	case b.BatchID == "":
		return fmt.Errorf("%w: batch_id is required", ErrInvalid)
	case b.DeviceID == "":
		return fmt.Errorf("%w: device_id is required", ErrInvalid)
	case len(b.Samples) == 0:
		return fmt.Errorf("%w: batch has no samples", ErrInvalid)
	case len(b.Samples) > MaxSamplesPerBatch:
		return fmt.Errorf("%w: batch has %d samples, max is %d", ErrInvalid, len(b.Samples), MaxSamplesPerBatch)
	}
	limit := now.Add(MaxClockSkew)
	for i, s := range b.Samples {
		switch {
		case s.Metric == "":
			return fmt.Errorf("%w: sample %d has no metric name", ErrInvalid, i)
		case s.TS.IsZero():
			return fmt.Errorf("%w: sample %d has no timestamp", ErrInvalid, i)
		case s.TS.After(limit):
			return fmt.Errorf("%w: sample %d timestamp %s is too far in the future", ErrInvalid, i, s.TS.Format(time.RFC3339))
		}
	}
	return nil
}

// TelemetryEnvelope is what ingestion writes to Kafka: the device's batch plus
// server-side metadata the device cannot be trusted to supply.
type TelemetryEnvelope struct {
	ReceivedAt time.Time      `json:"received_at"`
	Batch      TelemetryBatch `json:"batch"`
}

type DeviceEventType string

const (
	EventRegistered DeviceEventType = "registered"
	EventHeartbeat  DeviceEventType = "heartbeat"
)

// DeviceEvent records lifecycle changes. Events for one device share a Kafka
// key, so they stay ordered within a partition.
type DeviceEvent struct {
	Type     DeviceEventType `json:"type"`
	DeviceID string          `json:"device_id"`
	Model    string          `json:"model,omitempty"`
	Firmware string          `json:"firmware,omitempty"`
	SiteID   string          `json:"site_id,omitempty"`
	At       time.Time       `json:"at"`
}
