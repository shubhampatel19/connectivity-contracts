package wire

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBundleInitValidate(t *testing.T) {
	good := func() BundleInitRequest {
		return BundleInitRequest{
			BundleID: "b-1", DeviceID: "site-a-00001", SizeBytes: 1024,
			SHA256: strings.Repeat("ab", 32), Reason: ReasonAlert, CollectedAt: time.Now(),
		}
	}
	tests := []struct {
		name    string
		mutate  func(*BundleInitRequest)
		wantErr bool
	}{
		{"valid", func(*BundleInitRequest) {}, false},
		{"bundle id with slash", func(r *BundleInitRequest) { r.BundleID = "../etc" }, true},
		{"bundle id too long", func(r *BundleInitRequest) { r.BundleID = strings.Repeat("a", 65) }, true},
		{"device id with slash", func(r *BundleInitRequest) { r.DeviceID = "a/b" }, true},
		{"zero size", func(r *BundleInitRequest) { r.SizeBytes = 0 }, true},
		{"over max", func(r *BundleInitRequest) { r.SizeBytes = 4097 }, true},
		{"uppercase sha", func(r *BundleInitRequest) { r.SHA256 = strings.Repeat("AB", 32) }, true},
		{"short sha", func(r *BundleInitRequest) { r.SHA256 = "abc" }, true},
		{"unknown reason", func(r *BundleInitRequest) { r.Reason = "because" }, true},
		{"no collected_at", func(r *BundleInitRequest) { r.CollectedAt = time.Time{} }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := good()
			tc.mutate(&r)
			err := r.Validate(4096)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Fatalf("error %v does not wrap ErrInvalid", err)
			}
		})
	}
}

func TestChunkMath(t *testing.T) {
	tests := []struct {
		size, chunk int64
		count       int
		lens        []int64
	}{
		{size: 10, chunk: 4, count: 3, lens: []int64{4, 4, 2}},
		{size: 8, chunk: 4, count: 2, lens: []int64{4, 4}},
		{size: 1, chunk: 4, count: 1, lens: []int64{1}},
		{size: 0, chunk: 4, count: 0},
	}
	for _, tc := range tests {
		if got := ChunkCount(tc.size, tc.chunk); got != tc.count {
			t.Fatalf("ChunkCount(%d,%d) = %d, want %d", tc.size, tc.chunk, got, tc.count)
		}
		var total int64
		for i, want := range tc.lens {
			got := ChunkLen(tc.size, tc.chunk, i)
			if got != want {
				t.Fatalf("ChunkLen(%d,%d,%d) = %d, want %d", tc.size, tc.chunk, i, got, want)
			}
			total += got
		}
		if total != tc.size {
			t.Fatalf("chunk lengths sum to %d, want %d", total, tc.size)
		}
		if ChunkLen(tc.size, tc.chunk, tc.count) != 0 {
			t.Fatal("chunk past the end should have length 0")
		}
	}
}

func TestMissing(t *testing.T) {
	s := BundleStatus{ChunkCount: 5, ReceivedChunks: []int{3, 0, 1}}
	if got, want := s.Missing(), []int{2, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Missing() = %v, want %v", got, want)
	}
	done := BundleStatus{ChunkCount: 2, ReceivedChunks: []int{1, 0}}
	if got := done.Missing(); len(got) != 0 {
		t.Fatalf("Missing() = %v, want none", got)
	}
}

func TestValidID(t *testing.T) {
	for _, ok := range []string{"a", "site-a-00001", "B_2", strings.Repeat("x", 64)} {
		if !ValidID(ok) {
			t.Errorf("ValidID(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "-lead", "a/b", "a b", "../x", strings.Repeat("x", 65)} {
		if ValidID(bad) {
			t.Errorf("ValidID(%q) = true, want false", bad)
		}
	}
}
