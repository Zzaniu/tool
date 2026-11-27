package zlog

// Option 配置 InitZlog，使用 WithRotate* 或 WithOutputFormat 等函数创建。
type Option func(*options)

type options struct {
    rotation     RotationConfig
    outputFormat OutputFormat
}

// resolveOptions 只解析配置，不打开文件；OnlyStd 也可以使用。
func resolveOptions(logFilePath string, setters []Option) options {
    option := options{
        rotation: RotationConfig{
            FilePath:         logFilePath,
            KeepCnt:          10,
            RotateByDate:     true,
            DateIntervalDays: 1,
        },
    }
    for _, setter := range setters {
        setter(&option)
    }
    option.rotation = normalizeRotationConfig(option.rotation)
    return option
}

// WithRotateSize 设置轮转大小，单位为字节；非正数关闭大小轮转。
//
// 调用该选项会关闭日期轮转；如果需要同时按日期和大小轮转，
// 请在它之后调用 WithRotateDate(true)。
func WithRotateSize(size int64) Option {
    return func(o *options) {
        o.rotation.MaxSize = size
        o.rotation.RotateByDate = false
    }
}

// WithRotateKeep 设置保留文件总数，包含当前活动文件。
// 非正数只保留活动文件，不表示无限保留。
func WithRotateKeep(keepCnt int) Option {
    return func(o *options) {
        o.rotation.KeepCnt = keepCnt
    }
}

// WithRotateDate 启用或关闭日期轮转。
// 启用时，非正数的日期间隔恢复为 1 天。
func WithRotateDate(enabled bool) Option {
    return func(o *options) {
        o.rotation.RotateByDate = enabled
        if enabled && o.rotation.DateIntervalDays <= 0 {
            o.rotation.DateIntervalDays = 1
        }
    }
}

// WithRotateDateInterval 设置日历日期间隔，并选择日期轮转。
// days 为非正数时不触发日期轮转。
func WithRotateDateInterval(days int) Option {
    return func(o *options) {
        o.rotation.RotateByDate = true
        o.rotation.DateIntervalDays = days
    }
}

// WithOutputFormat 设置日志输出编码，默认 TextOutput。
func WithOutputFormat(format OutputFormat) Option {
    return func(o *options) {
        o.outputFormat = format
    }
}

// WithJSON 是 WithOutputFormat(JSONOutput) 的简写。
func WithJSON() Option {
    return WithOutputFormat(JSONOutput)
}
