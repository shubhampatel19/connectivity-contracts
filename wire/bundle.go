package wire

// Log bundles: a device packages its recent logs into a gzipped tar and
// uploads it in fixed-size chunks. Uploads resume after a dropped connection
// or an agent restart: the device asks which chunks the cloud already holds
// and sends only the rest.
//
//	POST /v1/bundles                         BundleInitRequest -> BundleStatus
//	GET  /v1/bundles/{bundle_id}             -> BundleStatus
//	PUT  /v1/bundles/{bundle_id}/chunks/{n}  raw bytes + HeaderChunkSHA256 -> 204
//	POST /v1/bundles/{bundle_id}/complete    -> BundleStatus (State: complete)
//
// On completion the cloud publishes a BundleUploadedEvent to TopicBundles.

import (
	"fmt"
	"regexp"
	"time"
)

const (
	TopicBundles = "bundles.uploaded"

	// HeaderChunkSHA256 carries the hex SHA-256 of one chunk's bytes, so a
	// chunk corrupted in transit is rejected instead of poisoning the bundle.
	HeaderChunkSHA256 = "X-Chunk-SHA256"
)

type BundleReason string

const (
	ReasonScheduled BundleReason = "scheduled" // periodic collection
	ReasonAlert     BundleReason = "alert"     // collected because the device detected a fault
	ReasonOnDemand  BundleReason = "on_demand" // requested by support
)

func (r BundleReason) Valid() bool {
	switch r {
	case ReasonScheduled, ReasonAlert, ReasonOnDemand:
		return true
	}
	return false
}

type BundleState string

const (
	BundleUploading BundleState = "uploading"
	BundleComplete  BundleState = "complete"
)

// idPattern restricts IDs that end up in URL paths and object keys.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ValidID reports whether s is safe to use as a bundle or device ID in URL
// paths and storage keys.
func ValidID(s string) bool { return idPattern.MatchString(s) }

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type BundleInitRequest struct {
	BundleID    string       `json:"bundle_id"`
	DeviceID    string       `json:"device_id"`
	SizeBytes   int64        `json:"size_bytes"`
	SHA256      string       `json:"sha256"`
	Reason      BundleReason `json:"reason"`
	CollectedAt time.Time    `json:"collected_at"`
}

func (r BundleInitRequest) Validate(maxBytes int64) error {
	switch {
	case !ValidID(r.BundleID):
		return fmt.Errorf("%w: bundle_id must be 1-64 characters of letters, digits, '-' or '_'", ErrInvalid)
	case !ValidID(r.DeviceID):
		return fmt.Errorf("%w: device_id must be 1-64 characters of letters, digits, '-' or '_'", ErrInvalid)
	case r.SizeBytes <= 0:
		return fmt.Errorf("%w: size_bytes must be positive", ErrInvalid)
	case r.SizeBytes > maxBytes:
		return fmt.Errorf("%w: bundle is %d bytes, max is %d", ErrInvalid, r.SizeBytes, maxBytes)
	case !sha256Pattern.MatchString(r.SHA256):
		return fmt.Errorf("%w: sha256 must be 64 lowercase hex characters", ErrInvalid)
	case !r.Reason.Valid():
		return fmt.Errorf("%w: unknown reason %q", ErrInvalid, r.Reason)
	case r.CollectedAt.IsZero():
		return fmt.Errorf("%w: collected_at is required", ErrInvalid)
	}
	return nil
}

// BundleStatus is the cloud's view of an upload. The chunk size is decided
// by the cloud, so it can be tuned without updating devices.
type BundleStatus struct {
	BundleID       string      `json:"bundle_id"`
	State          BundleState `json:"state"`
	ChunkSize      int64       `json:"chunk_size"`
	ChunkCount     int         `json:"chunk_count"`
	ReceivedChunks []int       `json:"received_chunks"`
}

// Missing returns, in order, the chunk indexes the cloud has not received.
func (s BundleStatus) Missing() []int {
	have := make(map[int]bool, len(s.ReceivedChunks))
	for _, i := range s.ReceivedChunks {
		have[i] = true
	}
	var out []int
	for i := 0; i < s.ChunkCount; i++ {
		if !have[i] {
			out = append(out, i)
		}
	}
	return out
}

// ChunkCount returns how many chunks a bundle of size bytes needs.
func ChunkCount(size, chunkSize int64) int {
	if size <= 0 || chunkSize <= 0 {
		return 0
	}
	return int((size + chunkSize - 1) / chunkSize)
}

// ChunkLen returns the expected length of chunk i. Every chunk is chunkSize
// bytes except possibly the last.
func ChunkLen(size, chunkSize int64, i int) int64 {
	start := int64(i) * chunkSize
	if i < 0 || start >= size {
		return 0
	}
	return min(chunkSize, size-start)
}

// BundleUploadedEvent is published once a bundle is assembled, verified and
// stored. Consumers must tolerate duplicates: a retried completion can
// publish the same event twice.
type BundleUploadedEvent struct {
	BundleID    string       `json:"bundle_id"`
	DeviceID    string       `json:"device_id"`
	Reason      BundleReason `json:"reason"`
	ObjectKey   string       `json:"object_key"`
	SizeBytes   int64        `json:"size_bytes"`
	SHA256      string       `json:"sha256"`
	CollectedAt time.Time    `json:"collected_at"`
	UploadedAt  time.Time    `json:"uploaded_at"`
}
