// Package logger provides JSON structured logging compatible with Google Cloud Logging.
package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	AUDIT
)

var levelStrings = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	AUDIT: "AUDIT",
}

type Fields map[string]interface{}

// contextKey is a private type for context keys defined in this package.
// Using a private type prevents collisions with keys defined in other
// packages (staticcheck SA1029).
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	TraceIDKey   contextKey = "trace_id"
	SpanIDKey    contextKey = "span_id"
	UserIDKey    contextKey = "user_id"
	ClientIPKey  contextKey = "client_ip"
	UserAgentKey contextKey = "user_agent"
)

type Entry struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Caller     string                 `json:"caller,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	SpanID     string                 `json:"span_id,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
}

type Logger interface {
	Debug(msg string, fields Fields)
	Info(msg string, fields Fields)
	Warn(msg string, fields Fields)
	Error(msg string, fields Fields)
	Audit(msg string, fields Fields)
	WithContext(ctx context.Context) Logger
	WithFields(fields Fields) Logger
}

type StructuredLogger struct {
	output     io.Writer
	level      LogLevel
	fields     Fields
	mu         sync.Mutex
	withCaller bool
	ctx        context.Context
}

var (
	defaultLogger *StructuredLogger
	once          sync.Once
)

func Init(level LogLevel, withCaller bool) {
	once.Do(func() {
		defaultLogger = &StructuredLogger{
			output:     os.Stdout,
			level:      level,
			fields:     make(Fields),
			withCaller: withCaller,
		}
	})
}

func Default() Logger {
	// Always go through Init to avoid racy nil-checks under concurrent use.
	// sync.Once provides the required synchronization.
	Init(INFO, true)
	return defaultLogger
}

func WithContext(ctx context.Context) Logger {
	return Default().WithContext(ctx)
}

func WithFields(fields Fields) Logger {
	return Default().WithFields(fields)
}

func NewLogger(output io.Writer, level LogLevel, withCaller bool) *StructuredLogger {
	return &StructuredLogger{
		output:     output,
		level:      level,
		fields:     make(Fields),
		withCaller: withCaller,
	}
}

func (l *StructuredLogger) WithContext(ctx context.Context) Logger {
	newLogger := &StructuredLogger{
		output:     l.output,
		level:      l.level,
		fields:     make(Fields),
		withCaller: l.withCaller,
		ctx:        ctx,
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	if ctx != nil {
		if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
			newLogger.fields["request_id"] = requestID
		}
		if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
			newLogger.fields["trace_id"] = traceID
		}
		if spanID, ok := ctx.Value(SpanIDKey).(string); ok {
			newLogger.fields["span_id"] = spanID
		}
		if userID, ok := ctx.Value(UserIDKey).(string); ok {
			newLogger.fields["user_id"] = userID
		}
		if clientIP, ok := ctx.Value(ClientIPKey).(string); ok {
			newLogger.fields["client_ip"] = clientIP
		}
	}

	return newLogger
}

func (l *StructuredLogger) WithFields(fields Fields) Logger {
	newLogger := &StructuredLogger{
		output:     l.output,
		level:      l.level,
		fields:     make(Fields),
		withCaller: l.withCaller,
		ctx:        l.ctx,
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

func (l *StructuredLogger) log(level LogLevel, msg string, fields Fields) {
	if level < l.level {
		return
	}

	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     levelStrings[level],
		Message:   msg,
		Fields:    make(map[string]interface{}),
	}

	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	for k, v := range fields {
		entry.Fields[k] = v
	}

	if requestID, ok := entry.Fields["request_id"].(string); ok {
		entry.RequestID = requestID
		delete(entry.Fields, "request_id")
	}
	if traceID, ok := entry.Fields["trace_id"].(string); ok {
		entry.TraceID = traceID
		delete(entry.Fields, "trace_id")
	}
	if spanID, ok := entry.Fields["span_id"].(string); ok {
		entry.SpanID = spanID
		delete(entry.Fields, "span_id")
	}

	if l.withCaller && level >= WARN {
		if pc, file, line, ok := runtime.Caller(3); ok {
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				parts := strings.Split(file, "/")
				if len(parts) > 2 {
					file = strings.Join(parts[len(parts)-2:], "/")
				}
				entry.Caller = fmt.Sprintf("%s:%d %s", file, line, fn.Name())
			}
		}
	}

	if level == ERROR {
		if stackTrace, ok := fields["stack_trace"].(string); ok {
			entry.StackTrace = stackTrace
			delete(entry.Fields, "stack_trace")
		} else if err, ok := fields["error"].(error); ok {
			entry.Fields["error"] = err.Error()
		}
	}

	if len(entry.Fields) == 0 {
		entry.Fields = nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(l.output, `{"timestamp":"%s","level":"ERROR","message":"Failed to marshal log entry","error":"%s"}`+"\n",
			time.Now().UTC().Format(time.RFC3339), err.Error())
		return
	}

	l.output.Write(data)
	l.output.Write([]byte("\n"))
}

func (l *StructuredLogger) Debug(msg string, fields Fields) {
	l.log(DEBUG, msg, fields)
}

func (l *StructuredLogger) Info(msg string, fields Fields) {
	l.log(INFO, msg, fields)
}

func (l *StructuredLogger) Warn(msg string, fields Fields) {
	l.log(WARN, msg, fields)
}

func (l *StructuredLogger) Error(msg string, fields Fields) {
	l.log(ERROR, msg, fields)
}

func (l *StructuredLogger) Audit(msg string, fields Fields) {
	l.log(AUDIT, msg, fields)
}

func Debug(msg string, fields Fields) {
	Default().Debug(msg, fields)
}

func Info(msg string, fields Fields) {
	Default().Info(msg, fields)
}

func Warn(msg string, fields Fields) {
	Default().Warn(msg, fields)
}

func Error(msg string, fields Fields) {
	Default().Error(msg, fields)
}

func Audit(msg string, fields Fields) {
	Default().Audit(msg, fields)
}
