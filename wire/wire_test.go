package wire

import (
	"errors"
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
