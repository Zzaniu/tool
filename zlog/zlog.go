package zlog

import (
    "context"
    "fmt"
    "io"
    "log/slog"
    "os"
    "runtime"
    "strings"
    "sync"
    "time"

    "go.opentelemetry.io/otel/trace"
)

type (
    // LogLevel 是 zlog 的级别位掩码。
    LogLevel   int8
    ModelLevel int8

    // OutputFormat 是初始化时选择的日志输出编码。
    OutputFormat int

    Logger interface {
        Debug(args ...interface{})
        Info(args ...interface{})
        Warn(args ...interface{})
        Error(args ...interface{})
        Fatal(args ...interface{})
        Debugf(format string, args ...interface{})
        Infof(format string, args ...interface{})
        Warnf(format string, args ...interface{})
        Errorf(format string, args ...interface{})
        Fatalf(format string, args ...interface{})
    }

    noCopy struct{}

    zlog struct {
        noCopy

        logger   *slog.Logger
        logLevel LogLevel
        closer   io.Closer
    }

    traceLog struct {
        z      *zlog
        logger *slog.Logger
        ctx    context.Context
    }
)

const (
    DEBUG LogLevel = 1 << iota
    INFO
    WARN
    ERROR
    FATAL
    ALLLogLevel = DEBUG | INFO | WARN | ERROR | FATAL
)

const (
    OnlyStd ModelLevel = iota
    OnlyFile
    AllModelLevel
)

const (
    TextOutput OutputFormat = iota
    JSONOutput
)

const levelFatal = slog.LevelError + 4

var (
    zlogx = newStdZlog(ALLLogLevel, TextOutput)
    once  sync.Once
)

func (n *noCopy) Lock() {}

func (n *noCopy) Unlock() {}

// InitZlog 初始化全局日志器。
// 文件输出统一使用 RotatingWriter；选项按传入顺序生效。
func InitZlog(logFilePath string, logLevel LogLevel, modelLevel ModelLevel, option ...Option) {
    once.Do(func() {
        switch modelLevel {
        case OnlyStd:
            stdOption := resolveOptions(logFilePath, option)
            zlogx = newStdZlog(logLevel, stdOption.outputFormat)
        case OnlyFile:
            zlogx = newZlog(logFilePath, logLevel, option)
        default:
            zlogx = newZlog(logFilePath, logLevel, option, os.Stdout)
        }
    })
}

// newStdZlog 创建只写 stdout 的 logger。
// 测试可传入 writer 覆盖 stdout。
func newStdZlog(logLevel LogLevel, outputFormat OutputFormat, writers ...io.Writer) *zlog {
    out := io.Writer(os.Stdout)
    if len(writers) > 0 && writers[0] != nil {
        out = writers[0]
    }
    return &zlog{
        logger:   slog.New(newHandler(out, outputFormat)),
        logLevel: logLevel,
    }
}

// newZlog 创建使用 RotatingWriter 的文件 logger。
func newZlog(logFilePath string, logLevel LogLevel, setters []Option, writers ...io.Writer) *zlog {
    option := resolveOptions(logFilePath, setters)
    writer, err := NewRotatingWriter(option.rotation)
    if err != nil {
        fatalInit(err)
    }
    var out io.Writer = writer

    if len(writers) > 0 {
        secondary := make([]io.Writer, 0, len(writers))
        for _, writer := range writers {
            if writer != nil {
                secondary = append(secondary, writer)
            }
        }
        if len(secondary) > 0 {
            out = &fanoutWriter{primary: out, secondary: secondary}
        }
    }

    return &zlog{
        logger:   slog.New(newHandler(out, option.outputFormat)),
        logLevel: logLevel,
        closer:   writer,
    }
}

// newHandler 根据初始化配置创建 Text 或 JSON Handler。
func newHandler(w io.Writer, format OutputFormat) slog.Handler {
    options := &slog.HandlerOptions{
        Level:     slog.LevelDebug,
        AddSource: true,
        ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
            if a.Key == slog.LevelKey {
                if level, ok := a.Value.Any().(slog.Level); ok && level == levelFatal {
                    a.Value = slog.StringValue("FATAL")
                }
            }
            return a
        },
    }
    if format == JSONOutput {
        return slog.NewJSONHandler(w, options)
    }
    return slog.NewTextHandler(w, options)
}

// fatalInit 是初始化阶段的 fail-fast 入口。
func fatalInit(err error) {
    fmt.Fprintf(os.Stderr, "zlog: init failed: %v\n", err)
    os.Exit(1)
}

// fanoutWriter 先写 primary，再写 secondary。
// 与 io.MultiWriter 不同，secondary 写失败不会阻止 primary 的写入。
type fanoutWriter struct {
    primary   io.Writer
    secondary []io.Writer
}

func (w *fanoutWriter) Write(p []byte) (int, error) {
    n, err := w.primary.Write(p)
    for _, writer := range w.secondary {
        if _, writeErr := writer.Write(p); writeErr != nil && err == nil {
            err = writeErr
        }
    }
    return n, err
}

// callerPC 返回调用方 PC。
// 使用 noinline 保证 skip 计算不受编译器内联影响。
//
//go:noinline
func callerPC(skip int) uintptr {
    var pcs [1]uintptr
    if n := runtime.Callers(skip+2, pcs[:]); n == 0 {
        return 0
    }
    return pcs[0]
}

func (z *zlog) Close() error {
    if z == nil || z.closer == nil {
        return nil
    }
    return z.closer.Close()
}

func (z *zlog) enabled(level LogLevel) bool {
    if z == nil {
        return false
    }
    return level == FATAL || z.logLevel&level != 0
}

// log 是所有日志入口的统一实现。
func (z *zlog) log(ctx context.Context, logger *slog.Logger, level LogLevel, pc uintptr, msg string) {
    if z == nil || logger == nil {
        return
    }
    if level != FATAL && z.logLevel&level == 0 {
        return
    }
    if ctx == nil {
        ctx = context.Background()
    }

    slogLevel := toSlogLevel(level)
    handler := logger.Handler()
    if !handler.Enabled(ctx, slogLevel) {
        return
    }

    record := slog.NewRecord(time.Now(), slogLevel, msg, pc)
    if err := handler.Handle(ctx, record); err != nil {
        // slog.Logger 会丢弃 Handler 错误，这里显式输出到 stderr。
        fmt.Fprintf(os.Stderr, "zlog: handle log failed: %v\n", err)
    }
}

func toSlogLevel(level LogLevel) slog.Level {
    switch level {
    case DEBUG:
        return slog.LevelDebug
    case WARN:
        return slog.LevelWarn
    case ERROR:
        return slog.LevelError
    case FATAL:
        return levelFatal
    default:
        return slog.LevelInfo
    }
}

func sprintln(args []interface{}) string {
    return strings.TrimSuffix(fmt.Sprintln(args...), "\n")
}

func (z *zlog) Debug(args ...interface{}) {
    if !z.enabled(DEBUG) {
        return
    }
    z.log(context.Background(), z.logger, DEBUG, callerPC(1), sprintln(args))
}

func (z *zlog) Info(args ...interface{}) {
    if !z.enabled(INFO) {
        return
    }
    z.log(context.Background(), z.logger, INFO, callerPC(1), sprintln(args))
}

func (z *zlog) Warn(args ...interface{}) {
    if !z.enabled(WARN) {
        return
    }
    z.log(context.Background(), z.logger, WARN, callerPC(1), sprintln(args))
}

func (z *zlog) Error(args ...interface{}) {
    if !z.enabled(ERROR) {
        return
    }
    z.log(context.Background(), z.logger, ERROR, callerPC(1), sprintln(args))
}

func (z *zlog) Fatal(args ...interface{}) {
    z.log(context.Background(), z.logger, FATAL, callerPC(1), sprintln(args))
    panic(fmt.Sprintln(args...))
}

func (z *zlog) Debugf(format string, args ...interface{}) {
    if !z.enabled(DEBUG) {
        return
    }
    z.log(context.Background(), z.logger, DEBUG, callerPC(1), fmt.Sprintf(format, args...))
}

func (z *zlog) Infof(format string, args ...interface{}) {
    if !z.enabled(INFO) {
        return
    }
    z.log(context.Background(), z.logger, INFO, callerPC(1), fmt.Sprintf(format, args...))
}

func (z *zlog) Warnf(format string, args ...interface{}) {
    if !z.enabled(WARN) {
        return
    }
    z.log(context.Background(), z.logger, WARN, callerPC(1), fmt.Sprintf(format, args...))
}

func (z *zlog) Errorf(format string, args ...interface{}) {
    if !z.enabled(ERROR) {
        return
    }
    z.log(context.Background(), z.logger, ERROR, callerPC(1), fmt.Sprintf(format, args...))
}

func (z *zlog) Fatalf(format string, args ...interface{}) {
    z.log(context.Background(), z.logger, FATAL, callerPC(1), fmt.Sprintf(format, args...))
    panic(fmt.Sprintf(format, args...))
}

func Close() error {
    if zlogx == nil {
        return nil
    }
    return zlogx.Close()
}

func Debug(args ...interface{}) {
    if zlogx == nil || !zlogx.enabled(DEBUG) {
        return
    }
    zlogx.log(context.Background(), zlogx.logger, DEBUG, callerPC(1), sprintln(args))
}

func Info(args ...interface{}) {
    if zlogx == nil || !zlogx.enabled(INFO) {
        return
    }
    zlogx.log(context.Background(), zlogx.logger, INFO, callerPC(1), sprintln(args))
}

func Warn(args ...interface{}) {
    if zlogx == nil || !zlogx.enabled(WARN) {
        return
    }
    zlogx.log(context.Background(), zlogx.logger, WARN, callerPC(1), sprintln(args))
}

func Error(args ...interface{}) {
    if zlogx == nil || !zlogx.enabled(ERROR) {
        return
    }
    zlogx.log(context.Background(), zlogx.logger, ERROR, callerPC(1), sprintln(args))
}

func Fatal(args ...interface{}) {
    zlogx.log(context.Background(), zlogx.logger, FATAL, callerPC(1), sprintln(args))
    panic(fmt.Sprintln(args...))
}

func Debugf(format string, args ...interface{}) {
    if zlogx == nil || !zlogx.enabled(DEBUG) {
        return
    }
    zlogx.log(context.Background(), zlogx.logger, DEBUG, callerPC(1), fmt.Sprintf(format, args...))
}

func Infof(format string, args ...interface{}) {
    if zlogx == nil || !zlogx.enabled(INFO) {
        return
    }
    zlogx.log(context.Background(), zlogx.logger, INFO, callerPC(1), fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
    if zlogx == nil || !zlogx.enabled(WARN) {
        return
    }
    zlogx.log(context.Background(), zlogx.logger, WARN, callerPC(1), fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...interface{}) {
    if zlogx == nil || !zlogx.enabled(ERROR) {
        return
    }
    zlogx.log(context.Background(), zlogx.logger, ERROR, callerPC(1), fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...interface{}) {
    zlogx.log(context.Background(), zlogx.logger, FATAL, callerPC(1), fmt.Sprintf(format, args...))
    panic(fmt.Sprintf(format, args...))
}

func WithTrace(ctx context.Context) Logger {
    if zlogx == nil {
        return zlogx
    }
    if ctx == nil {
        ctx = context.Background()
    }
    spanContext := trace.SpanContextFromContext(ctx)
    if spanContext.HasTraceID() {
        traceId := spanContext.TraceID().String()
        return &traceLog{
            z:      zlogx,
            logger: zlogx.logger.With("trace_id", traceId),
            ctx:    ctx,
        }
    }
    return zlogx
}

func (t *traceLog) Debug(args ...interface{}) {
    if t == nil || !t.z.enabled(DEBUG) {
        return
    }
    t.z.log(t.ctx, t.logger, DEBUG, callerPC(1), sprintln(args))
}

func (t *traceLog) Info(args ...interface{}) {
    if t == nil || !t.z.enabled(INFO) {
        return
    }
    t.z.log(t.ctx, t.logger, INFO, callerPC(1), sprintln(args))
}

func (t *traceLog) Warn(args ...interface{}) {
    if t == nil || !t.z.enabled(WARN) {
        return
    }
    t.z.log(t.ctx, t.logger, WARN, callerPC(1), sprintln(args))
}

func (t *traceLog) Error(args ...interface{}) {
    if t == nil || !t.z.enabled(ERROR) {
        return
    }
    t.z.log(t.ctx, t.logger, ERROR, callerPC(1), sprintln(args))
}

func (t *traceLog) Fatal(args ...interface{}) {
    t.z.log(t.ctx, t.logger, FATAL, callerPC(1), sprintln(args))
    panic(fmt.Sprintln(args...))
}

func (t *traceLog) Debugf(format string, args ...interface{}) {
    if t == nil || !t.z.enabled(DEBUG) {
        return
    }
    t.z.log(t.ctx, t.logger, DEBUG, callerPC(1), fmt.Sprintf(format, args...))
}

func (t *traceLog) Infof(format string, args ...interface{}) {
    if t == nil || !t.z.enabled(INFO) {
        return
    }
    t.z.log(t.ctx, t.logger, INFO, callerPC(1), fmt.Sprintf(format, args...))
}

func (t *traceLog) Warnf(format string, args ...interface{}) {
    if t == nil || !t.z.enabled(WARN) {
        return
    }
    t.z.log(t.ctx, t.logger, WARN, callerPC(1), fmt.Sprintf(format, args...))
}

func (t *traceLog) Errorf(format string, args ...interface{}) {
    if t == nil || !t.z.enabled(ERROR) {
        return
    }
    t.z.log(t.ctx, t.logger, ERROR, callerPC(1), fmt.Sprintf(format, args...))
}

func (t *traceLog) Fatalf(format string, args ...interface{}) {
    t.z.log(t.ctx, t.logger, FATAL, callerPC(1), fmt.Sprintf(format, args...))
    panic(fmt.Sprintf(format, args...))
}
