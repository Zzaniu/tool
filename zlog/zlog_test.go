package zlog

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "io"
    "path/filepath"
    "strings"
    "testing"

    "go.opentelemetry.io/otel/trace"
)

func newTestLogger(t *testing.T, level LogLevel, options ...Option) (*zlog, string) {
    t.Helper()

    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    logger := newZlog(path, level, options)
    t.Cleanup(func() {
        _ = logger.Close()
    })
    return logger, path
}

func readTestLog(t *testing.T, path string) string {
    t.Helper()
    return readLogContent(t, filepath.Dir(path), filepath.Base(path))
}

func TestLoggerLevelFilter(t *testing.T) {
    logger, path := newTestLogger(t, INFO)

    logger.Debug("debug-hidden")
    logger.Info("info-visible")
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    content := readTestLog(t, path)
    if strings.Contains(content, "debug-hidden") {
        t.Fatalf("debug log should be filtered: %s", content)
    }
    if !strings.Contains(content, "info-visible") {
        t.Fatalf("info log missing: %s", content)
    }
}

func TestLoggerInfoAndInfof(t *testing.T) {
    logger, path := newTestLogger(t, ALLLogLevel)

    logger.Info("a", 1)
    logger.Infof("x=%d", 2)
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    content := readTestLog(t, path)
    if !strings.Contains(content, "a 1") {
        t.Fatalf("Info message missing: %s", content)
    }
    if !strings.Contains(content, "x=2") {
        t.Fatalf("Infof message missing: %s", content)
    }
}

func TestLoggerSourcePointsToCaller(t *testing.T) {
    logger, path := newTestLogger(t, ALLLogLevel)

    logger.Info("source-check")
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    content := readTestLog(t, path)
    if !strings.Contains(content, "zlog_test.go") {
        t.Fatalf("source should point to test file: %s", content)
    }
    if strings.Contains(content, "zlog.go:") {
        t.Fatalf("source must not point to zlog wrapper: %s", content)
    }
}

func TestLoggerFatalPanics(t *testing.T) {
    logger, path := newTestLogger(t, ALLLogLevel)

    var panicValue interface{}
    func() {
        defer func() {
            panicValue = recover()
        }()
        logger.Fatal("fatal-msg")
    }()

    if panicValue == nil {
        t.Fatal("Fatal should panic")
    }
    if !strings.Contains(panicValue.(string), "fatal-msg") {
        t.Fatalf("unexpected panic value: %v", panicValue)
    }

    content := readTestLog(t, path)
    if !strings.Contains(content, "level=FATAL") {
        t.Fatalf("Fatal level missing: %s", content)
    }
    if !strings.Contains(content, "fatal-msg") {
        t.Fatalf("Fatal message missing: %s", content)
    }
}

func TestLoggerWithTrace(t *testing.T) {
    oldLogger := zlogx
    defer func() {
        zlogx = oldLogger
    }()

    logger, path := newTestLogger(t, ALLLogLevel)
    zlogx = logger

    traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
    if err != nil {
        t.Fatal(err)
    }
    spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
    if err != nil {
        t.Fatal(err)
    }
    spanContext := trace.NewSpanContext(trace.SpanContextConfig{
        TraceID:    traceID,
        SpanID:     spanID,
        TraceFlags: trace.FlagsSampled,
    })
    ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

    WithTrace(ctx).Info("trace-msg")
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    content := readTestLog(t, path)
    if !strings.Contains(content, "trace_id="+traceID.String()) {
        t.Fatalf("trace_id missing: %s", content)
    }
    if !strings.Contains(content, "zlog_test.go") {
        t.Fatalf("trace source should point to test file: %s", content)
    }
}

func TestLoggerCloseIdempotent(t *testing.T) {
    logger, _ := newTestLogger(t, ALLLogLevel)

    if err := logger.Close(); err != nil {
        t.Fatalf("first close: %v", err)
    }
    if err := logger.Close(); err != nil {
        t.Fatalf("second close: %v", err)
    }
}

func TestFanoutWriterKeepsPrimaryOnSecondaryError(t *testing.T) {
    var primary bytes.Buffer
    writer := &fanoutWriter{
        primary:   &primary,
        secondary: []io.Writer{failingWriter{}},
    }

    n, err := writer.Write([]byte("payload"))
    if n != len("payload") {
        t.Fatalf("got n=%d, want %d", n, len("payload"))
    }
    if err == nil {
        t.Fatal("expected secondary write error")
    }
    if primary.String() != "payload" {
        t.Fatalf("primary writer lost data: %q", primary.String())
    }
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
    return 0, errors.New("write failed")
}

func TestPackageLevelSourcePointsToCaller(t *testing.T) {
    oldLogger := zlogx
    defer func() {
        zlogx = oldLogger
    }()

    logger, path := newTestLogger(t, ALLLogLevel)
    zlogx = logger

    Info("pkg-source-check")
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    content := readTestLog(t, path)
    if !strings.Contains(content, "zlog_test.go") {
        t.Fatalf("package-level source should point to test file: %s", content)
    }
    if strings.Contains(content, "zlog.go:") {
        t.Fatalf("package-level source must not point to zlog wrapper: %s", content)
    }
}

func TestLoggerRotateOptions(t *testing.T) {
    logger, path := newTestLogger(t, ALLLogLevel,
        WithRotateSize(10),
        WithRotateKeep(2),
    )

    logger.Info(strings.Repeat("x", 20))
    logger.Info(strings.Repeat("y", 20))
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    names := listLogFiles(t, filepath.Dir(path), filepath.Base(path))
    if len(names) != 2 {
        t.Fatalf("expected 2 size-rotated files, got %v", names)
    }
}

func BenchmarkLoggerInfof(b *testing.B) {
    logger := newZlog(filepath.Join(b.TempDir(), "app"), ALLLogLevel, nil)
    defer logger.Close()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        logger.Infof("benchmark line %d", i)
    }
}

func BenchmarkLoggerInfofSize(b *testing.B) {
    logger := newZlog(
        filepath.Join(b.TempDir(), "app"),
        ALLLogLevel,
        []Option{WithRotateSize(1 << 30), WithRotateKeep(10)},
    )
    defer logger.Close()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        logger.Infof("benchmark line %d", i)
    }
}

type countingStringer struct {
    calls *int
}

func (s countingStringer) String() string {
    *s.calls++
    return "stringer"
}

func TestLoggerDisabledLevelSkipsFormatting(t *testing.T) {
    oldLogger := zlogx
    defer func() {
        zlogx = oldLogger
    }()

    logger, _ := newTestLogger(t, INFO)
    zlogx = logger

    traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
    if err != nil {
        t.Fatal(err)
    }
    spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
    if err != nil {
        t.Fatal(err)
    }
    spanContext := trace.NewSpanContext(trace.SpanContextConfig{
        TraceID:    traceID,
        SpanID:     spanID,
        TraceFlags: trace.FlagsSampled,
    })
    ctx := trace.ContextWithSpanContext(context.Background(), spanContext)
    traced := WithTrace(ctx)

    calls := 0
    value := countingStringer{calls: &calls}
    logger.Debug("method", value)
    logger.Debugf("method=%v", value)
    Debug("package", value)
    Debugf("package=%v", value)
    traced.Debug("trace", value)
    traced.Debugf("trace=%v", value)

    if calls != 0 {
        t.Fatalf("disabled logs formatted %d times", calls)
    }
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }
}

func BenchmarkLoggerDebugfDisabled(b *testing.B) {
    logger := newZlog(filepath.Join(b.TempDir(), "app"), INFO, nil)
    defer logger.Close()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        logger.Debugf("disabled %d", i)
    }
}

func TestJSONOutputFormat(t *testing.T) {
    logger, path := newTestLogger(t, ALLLogLevel, WithJSON())

    logger.Info("json-msg")
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    content := strings.TrimSpace(readTestLog(t, path))
    var record map[string]any
    if err := json.Unmarshal([]byte(content), &record); err != nil {
        t.Fatalf("invalid json output: %v, content=%q", err, content)
    }
    if record["level"] != "INFO" {
        t.Fatalf("level=%v", record["level"])
    }
    if record["msg"] != "json-msg" {
        t.Fatalf("msg=%v", record["msg"])
    }
    if _, ok := record["source"]; !ok {
        t.Fatalf("source missing: %v", record)
    }
}

func TestJSONFatalLevel(t *testing.T) {
    logger, path := newTestLogger(t, ALLLogLevel, WithOutputFormat(JSONOutput))

    var panicValue interface{}
    func() {
        defer func() {
            panicValue = recover()
        }()
        logger.Fatal("fatal-json")
    }()
    if panicValue == nil {
        t.Fatal("Fatal should panic")
    }
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    content := strings.TrimSpace(readTestLog(t, path))
    var record map[string]any
    if err := json.Unmarshal([]byte(content), &record); err != nil {
        t.Fatalf("invalid json output: %v, content=%q", err, content)
    }
    if record["level"] != "FATAL" {
        t.Fatalf("level=%v", record["level"])
    }
}

func TestStdLoggerJSONOutput(t *testing.T) {
    var buffer bytes.Buffer
    logger := newStdZlog(INFO, JSONOutput, &buffer)

    logger.Info("std-json")
    if err := logger.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    var record map[string]any
    if err := json.Unmarshal(buffer.Bytes(), &record); err != nil {
        t.Fatalf("invalid json output: %v, content=%q", err, buffer.String())
    }
    if record["level"] != "INFO" || record["msg"] != "std-json" {
        t.Fatalf("unexpected record: %v", record)
    }
}

func TestOnlyStdOutputFormatLastOptionWins(t *testing.T) {
    textAfterJSON := resolveOptions("", []Option{
        WithJSON(),
        WithOutputFormat(TextOutput),
    })
    if textAfterJSON.outputFormat != TextOutput {
        t.Fatalf("textAfterJSON=%v", textAfterJSON.outputFormat)
    }

    jsonAfterText := resolveOptions("", []Option{
        WithOutputFormat(TextOutput),
        WithJSON(),
    })
    if jsonAfterText.outputFormat != JSONOutput {
        t.Fatalf("jsonAfterText=%v", jsonAfterText.outputFormat)
    }

    if got := resolveOptions("", nil).outputFormat; got != TextOutput {
        t.Fatalf("default outputFormat=%v", got)
    }
}
