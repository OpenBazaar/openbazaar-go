package rt

// Specifies importance of message, LogLevel numbering is consistent with the uber-go/zap package.
type LogLevel int

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in
	// production.
	DEBUG LogLevel = iota - 1
	// InfoLevel is the default logging priority.
	INFO
	// WarnLevel logs are more important than Info, but don't need individual
	// human review.
	WARN
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-level logs.
	ERROR
)
