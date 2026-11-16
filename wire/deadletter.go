package wire

// Dead-letter topics. A consumer that meets a record it can never process
// (malformed, or rejected by its store for a reason that retrying won't fix)
// copies the record, unchanged, to the dead-letter topic of the topic it came
// from and moves on. The record's own headers are kept, and these are added,
// so an engineer can see why it was set aside and replay it after a fix.
const (
	DeadLetterSuffix = ".dlq"

	HeaderDLQReason    = "x-dlq-reason"    // why the consumer gave up, as text
	HeaderDLQGroup     = "x-dlq-group"     // the consumer group that gave up
	HeaderDLQPartition = "x-dlq-partition" // where the record was, in its source topic
	HeaderDLQOffset    = "x-dlq-offset"
)

// DeadLetterTopic names the dead-letter topic for a source topic. Dead-letter
// topics are keyed like their source, so one device's records stay together.
func DeadLetterTopic(topic string) string { return topic + DeadLetterSuffix }

// SourceTopics are the topics devices' data flows through; each has a
// dead-letter topic the platform must create.
var SourceTopics = []string{TopicTelemetry, TopicDeviceEvents, TopicBundles, TopicAlerts}
