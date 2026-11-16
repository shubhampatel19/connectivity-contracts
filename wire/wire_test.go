package wire

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	good := func() TelemetryBatch {
		return TelemetryBatch{BatchID: "b1", DeviceID: "d1", Samples: []Sample{{Metric: "cpu_temp_c", Value: 50, TS: now}}}
	}

	tests := []struct {
		name    string
		mutate  func(*TelemetryBatch)
		wantErr bool
	}{
		{"valid", func(*TelemetryBatch) {}, false},
		{"missing batch id", func(b *TelemetryBatch) { b.BatchID = "" }, true},
		{"no samples", func(b *TelemetryBatch) { b.Samples = nil }, true},
		{"too many samples", func(b *TelemetryBatch) { b.Samples = make([]Sample, MaxSamplesPerBatch+1) }, true},
		{"empty metric", func(b *TelemetryBatch) { b.Samples[0].Metric = "" }, true},
		{"zero timestamp", func(b *TelemetryBatch) { b.Samples[0].TS = time.Time{} }, true},
		{"within skew", func(b *TelemetryBatch) { b.Samples[0].TS = now.Add(MaxClockSkew - time.Second) }, false},
		{"beyond skew", func(b *TelemetryBatch) { b.Samples[0].TS = now.Add(MaxClockSkew + time.Second) }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := good()
			tc.mutate(&b)
			err := b.Validate(now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Fatalf("error %v does not wrap ErrInvalid", err)
			}
		})
	}
}

func TestValidateRejectsTextThatWouldPoisonAConsumer(t *testing.T) {
	now := time.Now()
	ok := TelemetryBatch{BatchID: "b1", DeviceID: "d1", Samples: []Sample{{Metric: "cpu_temp_c", Value: 1, TS: now}}}
	if err := ok.Validate(now); err != nil {
		t.Fatalf("valid batch rejected: %v", err)
	}
	for name, mutate := range map[string]func(*TelemetryBatch){
		"NUL in batch_id":        func(b *TelemetryBatch) { b.BatchID = "b\x00" },
		"space in batch_id":      func(b *TelemetryBatch) { b.BatchID = "b 1" },
		"NUL in metric":          func(b *TelemetryBatch) { b.Samples[0].Metric = "cpu\x00" },
		"upper-case metric":      func(b *TelemetryBatch) { b.Samples[0].Metric = "CPU" },
		"metric with a quote":    func(b *TelemetryBatch) { b.Samples[0].Metric = "cpu'--" },
		"over-long metric":       func(b *TelemetryBatch) { b.Samples[0].Metric = "m" + strings.Repeat("x", 64) },
		"device_id with a slash": func(b *TelemetryBatch) { b.DeviceID = "a/b" },
	} {
		b := TelemetryBatch{BatchID: "b1", DeviceID: "d1", Samples: []Sample{{Metric: "cpu_temp_c", Value: 1, TS: now}}}
		mutate(&b)
		if err := b.Validate(now); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}

func TestRegisterRequestValidate(t *testing.T) {
	ok := RegisterRequest{DeviceID: "site-a-00001", Model: "PowerEdge R760", Firmware: "1.4.2", SiteID: "site-a"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid registration rejected: %v", err)
	}
	if err := (RegisterRequest{DeviceID: "d1"}).Validate(); err != nil {
		t.Fatalf("optional fields should be optional: %v", err)
	}
	for name, r := range map[string]RegisterRequest{
		"no device_id":     {},
		"NUL in model":     {DeviceID: "d1", Model: "R760\x00"},
		"newline firmware": {DeviceID: "d1", Firmware: "1.4\n2"},
		"bad site_id":      {DeviceID: "d1", SiteID: "site a"},
		"over-long model":  {DeviceID: "d1", Model: strings.Repeat("x", 65)},
		"invalid UTF-8":    {DeviceID: "d1", Model: "\xff\xfe"},
	} {
		if err := r.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}

func TestValidText(t *testing.T) {
	for s, want := range map[string]bool{
		"": true, "fan1 at 0 rpm, cpu 91.5°C": true, "x\x00y": false, "a\nb": false, "a\tb": false, "\x1b[31m": false, "\xff": false,
	} {
		if got := ValidText(s); got != want {
			t.Errorf("ValidText(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestDeadLetterTopic(t *testing.T) {
	if got := DeadLetterTopic(TopicTelemetry); got != "telemetry.raw.dlq" {
		t.Fatalf("got %q", got)
	}
	if len(SourceTopics) != 4 {
		t.Fatalf("SourceTopics = %v", SourceTopics)
	}
}
