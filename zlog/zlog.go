/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/12 14:26
Desc   : 实现的很丑, 后续不再维护, 建议是直接使用 zap/logrus 即可

    ......................我佛慈悲......................

                           _oo0oo_
                          o8888888o
                          88" . "88
                          (| -_- |)
                          0\  =  /0
                        ___/`---'\___
                      .' \\|     |// '.
                     / \\|||  :  |||// \
                    / _||||| -卍-|||||- \
                   |   | \\\  -  /// |   |
                   | \_|  ''\---/''  |_/ |
                   \  .-\__  '-'  ___/-. /
                 ___'. .'  /--.--\  `. .'___
              ."" '<  `.___\_<|>_/___.' >' "".
             | | :  `- \`.;`\ _ /`;.`/ - ` : | |
             \  \ `_.   \_ __\ /__ _/   .-` /  /
         =====`-.____`.___ \_____/___.-`___.-'=====
                           `=---='

    ..................佛祖保佑, 永无BUG...................

*/

package zlog

import (
    "bytes"
    "context"
    "fmt"
    "go.opentelemetry.io/otel/trace"
    "golang.org/x/xerrors"
    "io"
    "log"
    "os"
    "sync"
)

type (
    LogLevel   int8
    ModelLevel int8

    SetOutput func(io.Writer)

    LogFileManage interface {
        IsCut(SetOutput) error // 是否切割文件
        GetFile() *os.File     // 获取文件操作对象
        FilePath() string      // 文件名
        Close() error          // 关闭文件
    }

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

        logger        *log.Logger
        logLevel      LogLevel
        logFileManage LogFileManage
    }

    traceLog struct {
        ctx     context.Context
        traceId string
    }

    Option struct {
        logFileManage LogFileManage
    }

    LogFileManageOption func(*Option)
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
    FmtStandard  = log.Llongfile | log.Ldate | log.Lmicroseconds
    FileStandard = os.O_WRONLY | os.O_CREATE | os.O_APPEND
    prefixName   = "[zlog] "
    callDepth    = 4
)

var (
    zlogx = &zlog{logger: log.New(io.Writer(os.Stdout), prefixName, FmtStandard), logLevel: ALLLogLevel}
    once  sync.Once
)

func (n *noCopy) Lock() {}

func (n *noCopy) Unlock() {}

func newZlog(logFilePath string, logLevel LogLevel, options []LogFileManageOption, writers ...io.Writer) *zlog {
    option := Option{logFileManage: NewDateCutMode(logFilePath)}
    for _, o := range options {
        o(&option)
    }
    ret := &zlog{
        logLevel:      logLevel,
        logFileManage: option.logFileManage,
    }
    writers = append(writers, ret.logFileManage.GetFile())
    ret.logger = log.New(io.MultiWriter(writers...), prefixName, FmtStandard)
    return ret
}

func WithLogFileManage(logFileManage LogFileManage) LogFileManageOption {
    return func(o *Option) {
        if o.logFileManage != nil {
            if err := o.logFileManage.Close(); err != nil {
                log.Fatalf("关闭文件`%v`失败, err = %v\n", o.logFileManage.FilePath(), err)
            }
        }
        o.logFileManage = logFileManage
    }
}

func InitZlog(logFilePath string, logLevel LogLevel, modelLevel ModelLevel, option ...LogFileManageOption) {
    once.Do(func() {
        switch modelLevel {
        case OnlyStd:
            zlogx.logLevel = logLevel
        case OnlyFile:
            zlogx = newZlog(logFilePath, logLevel, option)
        default:
            zlogx = newZlog(logFilePath, logLevel, option, os.Stdout)
        }
    })
}

func (z *zlog) Close() error {
    return z.logFileManage.Close()
}

func (z *zlog) Debugf(format string, args ...interface{}) {
    if z.logLevel&DEBUG > 0 {
        z.writef(DEBUG, format, args...)
    }
}

func (z *zlog) Infof(format string, args ...interface{}) {
    if z.logLevel&INFO > 0 {
        z.writef(INFO, format, args...)
    }
}

func (z *zlog) Warnf(format string, args ...interface{}) {
    if z.logLevel&WARN > 0 {
        z.writef(WARN, format, args...)
    }
}

func (z *zlog) Errorf(format string, args ...interface{}) {
    if z.logLevel&ERROR > 0 {
        z.writef(ERROR, format, args...)
    }
}

func (z *zlog) Fatalf(format string, args ...interface{}) {
    z.writef(FATAL, format, args...)
    panic(fmt.Sprintf(format, args...))
}

func (z *zlog) Debug(args ...interface{}) {
    if z.logLevel&DEBUG > 0 {
        z.write(DEBUG, args...)
    }
}

func (z *zlog) Info(args ...interface{}) {
    if z.logLevel&INFO > 0 {
        z.write(INFO, args...)
    }
}

func (z *zlog) Warn(args ...interface{}) {
    if z.logLevel&WARN > 0 {
        z.write(WARN, args...)
    }
}

func (z *zlog) Error(args ...interface{}) {
    if z.logLevel&ERROR > 0 {
        z.write(ERROR, args...)
    }
}

func (z *zlog) Fatal(args ...interface{}) {
    z.write(FATAL, args...)
    panic(fmt.Sprintln(args...))
}

func (z *zlog) setOutput(w io.Writer) {
    z.logger.SetOutput(w)
}

func (z *zlog) cut() {
    if z.logFileManage == nil {
        return
    }
    err := z.logFileManage.IsCut(func(writer io.Writer) {
        z.setOutput(writer)
    })
    if err != nil {
        log.Printf("日志切换发生错误, err = %+v", xerrors.Errorf("%w", err))
        Errorf("日志切换发生错误, err = %+v", xerrors.Errorf("%w", err))
    }
}

func (z *zlog) write(loglevel LogLevel, args ...interface{}) {
    z.cut()
    err := z.logger.Output(callDepth, fmt.Sprintln(setPrefix(loglevel, args)...))
    if err != nil {
        log.Printf("日志写入错误, err = %v\n", err)
    }
}

func (z *zlog) writef(loglevel LogLevel, format string, args ...interface{}) {
    z.cut()
    err := z.logger.Output(callDepth, fmt.Sprintf(setPrefixf(loglevel, format), args...))
    if err != nil {
        log.Printf("日志写入错误, err = %v\n", err)
    }
}

func setPrefix(level LogLevel, args []interface{}) []interface{} {
    levelStr := getLogLevelString(level)
    t := make([]interface{}, 0, len(args)+1)
    t = append(t, "["+levelStr+"]")
    t = append(t, args...)
    return t
}

func setPrefixf(level LogLevel, format string) string {
    levelStr := getLogLevelString(level)
    b := bytes.Buffer{}
    b.WriteString("[")
    b.WriteString(levelStr)
    b.WriteString("] ")
    b.WriteString(format)
    return b.String()
}

func getLogLevelString(level LogLevel) string {
    if level == DEBUG {
        return "DEBUG"
    } else if level == INFO {
        return "INFO"
    } else if level == WARN {
        return "WARN"
    } else if level == ERROR {
        return "ERROR"
    }
    return "FATAL"
}

func Close() error {
    if zlogx.logFileManage == nil {
        return nil
    }
    return zlogx.logFileManage.Close()
}

func Debugf(format string, args ...interface{}) {
    zlogx.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
    zlogx.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
    zlogx.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
    zlogx.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
    zlogx.Fatalf(format, args...)
}

func Debug(args ...interface{}) {
    zlogx.Debug(args...)
}

func Info(args ...interface{}) {
    zlogx.Info(args...)
}

func Warn(args ...interface{}) {
    zlogx.Warn(args...)
}

func Error(args ...interface{}) {
    zlogx.Error(args...)
}

func Fatal(args ...interface{}) {
    zlogx.Fatal(args...)
}

func WithTrace(ctx context.Context) Logger {
    spanContext := trace.SpanContextFromContext(ctx)
    if spanContext.HasTraceID() {
        return &traceLog{ctx: ctx, traceId: spanContext.TraceID().String()}
    }
    return zlogx
}

func (t *traceLog) Debug(args ...interface{}) {
    zlogx.Debug(t.addTraceId(args)...)
}

func (t *traceLog) Info(args ...interface{}) {
    zlogx.Info(t.addTraceId(args)...)
}

func (t *traceLog) Warn(args ...interface{}) {
    zlogx.Warn(t.addTraceId(args)...)
}

func (t *traceLog) Error(args ...interface{}) {
    zlogx.Error(t.addTraceId(args)...)
}

func (t *traceLog) Fatal(args ...interface{}) {
    zlogx.Fatal(t.addTraceId(args)...)
}

func (t *traceLog) Debugf(format string, args ...interface{}) {
    zlogx.Debugf(t.addTraceIdf(format), args...)
}

func (t *traceLog) Infof(format string, args ...interface{}) {
    zlogx.Infof(t.addTraceIdf(format), args...)
}

func (t *traceLog) Warnf(format string, args ...interface{}) {
    zlogx.Warnf(t.addTraceIdf(format), args...)
}

func (t *traceLog) Errorf(format string, args ...interface{}) {
    zlogx.Errorf(t.addTraceIdf(format), args...)
}

func (t *traceLog) Fatalf(format string, args ...interface{}) {
    zlogx.Fatalf(t.addTraceIdf(format), args...)
}

func (t *traceLog) addTraceId(args []interface{}) []interface{} {
    tmp := make([]interface{}, 0, len(args)+1)
    tmp = append(tmp, fmt.Sprintf("traceid: %s,", t.traceId))
    tmp = append(tmp, args...)
    return tmp
}

func (t *traceLog) addTraceIdf(format string) string {
    return fmt.Sprintf("traceid: %v, ", t.traceId) + format
}

func WithContext(ctx context.Context) (spanId, traceId string) {
    spanContext := trace.SpanContextFromContext(ctx)
    if spanContext.HasSpanID() {
		spanId = spanContext.SpanID().String()
    }
    if spanContext.HasTraceID() {
		traceId = spanContext.TraceID().String()
    }
    fmt.Printf("spanid = %v, traceid = %v\n", spanId, traceId)
    return
}
