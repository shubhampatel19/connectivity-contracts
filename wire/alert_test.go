package wire

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDeviceAlertValidate(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	good := func() DeviceAlert {
		return DeviceAlert{
			AlertID: "a1", DeviceID: "site-a-00001", SiteID: "site-a",
			Code: AlertThermalCritical, Severity: SeverityCritical, Value: 93.4,
			Message: "cpu_temp over fault threshold", ObservedAt: now, BundleID: "b1",
		}
	}
	tests := []struct {
		name    string
		mutate  func(*DeviceAlert)
		wantErr bool
	}{
		{"valid", func(*DeviceAlert) {}, false},
		{"no site, no bundle, no message", func(a *DeviceAlert) { a.SiteID, a.BundleID, a.Message = "", "", "" }, false},
		{"code the cloud doesn't know yet", func(a *DeviceAlert) { a.Code = "fan.stalled" }, false},
		{"missing alert id", func(a *DeviceAlert) { a.AlertID = "" }, true},
		{"alert id with a slash", func(a *DeviceAlert) { a.AlertID = "a/1" }, true},
		{"missing device id", func(a *DeviceAlert) { a.DeviceID = "" }, true},
		{"bad site id", func(a *DeviceAlert) { a.SiteID = "site a" }, true},
		{"code without subsystem", func(a *DeviceAlert) { a.Code = "overheat" }, true},
		{"uppercase code", func(a *DeviceAlert) { a.Code = "Thermal.Critical" }, true},
		{"overlong code", func(a *DeviceAlert) { a.Code = AlertCode("a." + strings.Repeat("b", 63)) }, true},
		{"unknown severity", func(a *DeviceAlert) { a.Severity = "urgent" }, true},
		{"missing severity", func(a *DeviceAlert) { a.Severity = "" }, true},
		{"message too long", func(a *DeviceAlert) { a.Message = strings.Repeat("x", MaxAlertMessageLen+1) }, true},
		{"zero observed_at", func(a *DeviceAlert) { a.ObservedAt = time.Time{} }, true},
		{"within skew", func(a *DeviceAlert) { a.ObservedAt = now.Add(MaxClockSkew - time.Second) }, false},
		{"beyond skew", func(a *DeviceAlert) { a.ObservedAt = now.Add(MaxClockSkew + time.Second) }, true},
		{"bad bundle id", func(a *DeviceAlert) { a.BundleID = "../etc" }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := good()
			tc.mutate(&a)
			err := a.Validate(now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Fatalf("error %v does not wrap ErrInvalid", err)
			}
		})
	}
}

// The JSON field names are the contract; a rename here would break every
// agent in the field.
func TestDeviceAlertJSONFieldNames(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	b, err := json.Marshal(AlertEnvelope{ReceivedAt: now, Alert: DeviceAlert{
		AlertID: "a1", DeviceID: "d1", SiteID: "s1", Code: AlertPSURail, Severity: SeverityWarning,
		Value: 11.7, Message: "m", ObservedAt: now, BundleID: "b1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	alert, _ := got["alert"].(map[string]any)
	if _, ok := got["received_at"]; !ok || alert == nil {
		t.Fatalf("envelope fields changed: %s", b)
	}
	for _, f := range []string{"alert_id", "device_id", "site_id", "code", "severity", "value", "message", "observed_at", "bundle_id"} {
		if _, ok := alert[f]; !ok {
			t.Errorf("alert JSON has no %q field: %s", f, b)
		}
	}
}
