package wire

import (
	"regexp"
	"unicode"
	"unicode/utf8"
)

// Free text and names from devices end up in Kafka, in Postgres TEXT columns,
// in log lines and in prompts. Postgres rejects a NUL byte in TEXT outright,
// and a consumer that retries database errors would then retry that record
// forever, stalling every device on its partition. So text is checked at the
// front door, where rejecting it costs one device one 400.

const (
	MaxMetricNameLen = 64
	MaxDeviceAttrLen = 64 // model, firmware
)

var metricPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,63}$`)

// ValidMetric reports whether s is a metric name: lower-case letters, digits,
// '_' and '.', starting with a letter, at most 64 characters. Bounding the
// alphabet also bounds what a metric name can do to a query or a dashboard.
func ValidMetric(s string) bool { return metricPattern.MatchString(s) }

// ValidText reports whether s is safe to store and display: valid UTF-8 with
// no control characters (NUL, newlines, escape sequences).
func ValidText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
