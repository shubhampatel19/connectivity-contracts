package wire

// Device alerts: a device reports a fault it detected itself. The cloud's
// case service decides whether the fault becomes a support case; the device
// only reports what it saw.
//
//	POST /v1/alerts   DeviceAlert -> 202 AlertAccepted
//
// Ingestion publishes an AlertEnvelope to TopicAlerts, keyed by device ID, so
// one device's alerts stay in order on one partition.

import (
	"fmt"
	"regexp"
	"time"
)

const (
	TopicAlerts = "device.alerts"

	MaxAlertMessageLen = 512
)

// AlertCode names the condition, as "<subsystem>.<condition>". The cloud
// accepts codes it doesn't know yet, because an agent can be newer than the
// cloud; the case service records them without opening a case.
type AlertCode string

const (
	AlertThermalCritical AlertCode = "thermal.critical"            // CPU over the fault threshold
	AlertDiskReallocated AlertCode = "storage.reallocated_sectors" // Value is the new sector count
	AlertPSURail         AlertCode = "power.rail_out_of_range"     // Value is the rail voltage
)

var alertCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func (c AlertCode) Valid() bool { return len(c) <= 64 && alertCodePattern.MatchString(string(c)) }

type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"
)

func (s AlertSeverity) Valid() bool {
	switch s {
	case SeverityCritical, SeverityWarning, SeverityInfo:
		return true
	}
	return false
}

// DeviceAlert is one fault, raised once when the condition starts. AlertID is
// assigned on the device and reused on every retry, which is what lets the
// cloud deduplicate deliveries.
type DeviceAlert struct {
	AlertID    string        `json:"alert_id"`
	DeviceID   string        `json:"device_id"`
	SiteID     string        `json:"site_id,omitempty"`
	Code       AlertCode     `json:"code"`
	Severity   AlertSeverity `json:"severity"`
	Value      float64       `json:"value"` // the reading that tripped the alert
	Message    string        `json:"message,omitempty"`
	ObservedAt time.Time     `json:"observed_at"`
	// BundleID is the log bundle the device collected for this fault, if it
	// collected one. The bundle may reach the cloud before or after the alert.
	BundleID string `json:"bundle_id,omitempty"`
}

func (a DeviceAlert) Validate(now time.Time) error {
	switch {
	case !ValidID(a.AlertID):
		return fmt.Errorf("%w: alert_id must be 1-64 characters of letters, digits, '-' or '_'", ErrInvalid)
	case !ValidID(a.DeviceID):
		return fmt.Errorf("%w: device_id must be 1-64 characters of letters, digits, '-' or '_'", ErrInvalid)
	case a.SiteID != "" && !ValidID(a.SiteID):
		return fmt.Errorf("%w: site_id must be 1-64 characters of letters, digits, '-' or '_'", ErrInvalid)
	case !a.Code.Valid():
		return fmt.Errorf("%w: code %q must look like subsystem.condition", ErrInvalid, a.Code)
	case !a.Severity.Valid():
		return fmt.Errorf("%w: unknown severity %q", ErrInvalid, a.Severity)
	case len(a.Message) > MaxAlertMessageLen:
		return fmt.Errorf("%w: message is %d bytes, max is %d", ErrInvalid, len(a.Message), MaxAlertMessageLen)
	case a.ObservedAt.IsZero():
		return fmt.Errorf("%w: observed_at is required", ErrInvalid)
	case a.ObservedAt.After(now.Add(MaxClockSkew)):
		return fmt.Errorf("%w: observed_at %s is too far in the future", ErrInvalid, a.ObservedAt.Format(time.RFC3339))
	case a.BundleID != "" && !ValidID(a.BundleID):
		return fmt.Errorf("%w: bundle_id must be 1-64 characters of letters, digits, '-' or '_'", ErrInvalid)
	}
	return nil
}

// AlertAccepted is the cloud's answer: the alert is durably queued for the
// case service, not that a case exists.
type AlertAccepted struct {
	AlertID string `json:"alert_id"`
}

// AlertEnvelope is what ingestion writes to Kafka: the device's alert plus
// server-side metadata.
type AlertEnvelope struct {
	ReceivedAt time.Time   `json:"received_at"`
	Alert      DeviceAlert `json:"alert"`
}
