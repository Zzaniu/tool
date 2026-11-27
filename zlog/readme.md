# zlog 使用与迁移

zlog 使用 `log/slog` 编码日志，通过 `RotatingWriter` 完成文件轮转，并提供包级日志方法、Text/JSON 输出和 TraceID 关联。本文先说明旧文件管理器 API 的迁移，再介绍请求 logger 的复用方式。

## 旧版本迁移到当前版本

本次清理属于破坏性 API 更新，不再保留旧文件管理器的适配层。当前版本使用 `WithRotate*`、`WithJSON`、`WithTrace` 的调用方式保持不变；显式声明旧选项类型的代码需要改成 `Option`。

保留 `InitZlog`、`Close`、`Logger`、日志级别和输出模式，以及 `Info/Infof/Error/Errorf` 等方法。初始化仍然只生效一次，应在启动服务和并发写日志之前完成；初始化失败仍向 stderr 报错并退出。服务正常退出时调用 `zlog.Close()` 关闭文件。日志级别仍是位掩码，`Fatal/Fatalf` 仍在记录后 panic。

### 接口替换表

| 已删除或变化的 API | 迁移方式 |
| --- | --- |
| `WithLogFileManage`、`LogFileManage`、`SetOutput` | 文件输出直接使用 `WithRotate*`；自定义输出改用 `io.Writer + slog.Handler`，见后文 |
| `NewSizeCutMode`、`SizeFileManage`、`WithCutSize` | `WithRotateSize(size)`，文件路径放到 `InitZlog` 第一个参数 |
| `NewDateCutMode`、`DateFileManage`、`WithCutIntervalDay` | `WithRotateDateInterval(days)`；默认已经按 1 个日历日轮转 |
| `WithKeepCnt` | `WithRotateKeep(n)`，注意下文的保留数量语义 |
| `NewTimeFileManage`、`TimeFileManage`、`WithCutIntervalTime` | 当前不提供按固定小时数/Duration 轮转；不能直接等价替换成日期轮转 |
| `LogFileManageOption` | `Option`，例如 `[]zlog.Option` |
| 旧配置结构体 `Option` | 现在 `Option` 是函数选项类型；不再使用 `zlog.Option{}` 或 `func(*zlog.Option)` 构造配置 |
| `FileManageOption/FileManageOptions`、`DateOption/SizeOption/TimeOption` | 删除旧配置变量，直接组合 `WithRotate*` 返回的 `Option` |
| `M10/M20/M50/M100` | 使用字节数表达式，如 `50 << 20` 表示 50 MiB |
| `Hour/Hour6/Hour12/Hour24` | 其他业务代码使用 `time.Hour` 等标准库值；这些值不能传给日期间隔选项 |
| `Format`、`Timestamp`、`Date` | 需要自定义文件名布局时使用 `RotationConfig.DateLayout/TimeLayout`；它们不是 Text/JSON 输出编码 |
| `FmtStandard`、`FileStandard` | 使用方自行选择标准库 `log`/文件打开标志；zlog 输出编码使用 `WithOutputFormat` |
| `Rename` | 不再手动重命名日志；轮转由 `RotatingWriter` 管理 |
| `WithContext(ctx)` | 日志关联使用 `WithTrace(ctx)`；仅提取 ID 时使用 `trace.SpanContextFromContext(ctx)` |

原来的离线清理接口 `CleanHisLog`、`CleanHisLogAt`、`CleanHisLogWithLayouts` 仍可使用，实现在 `cleanup.go`。只能在对应文件没有活动 writer 时离线调用；运行中的 writer 会自动清理。

### Text 输出与 Trace 字段迁移

旧版本 Text 输出大致为：

```text
[zlog] 2026/11/28 10:00:00.123456 C:/code/app.go:10: [INFO] hello
```

当前版本 Text 输出为 TextHandler 格式：

```text
time=2026-11-28T10:00:00.123+08:00 level=INFO source=C:/code/app.go:10 msg=hello
```

主要变化：

- 不再有 `[zlog]` 前缀；
- 级别从消息前的 `[INFO]` 变为 `level=INFO`；
- 时间从头部时间变为 `time=...`，默认 Text 输出的小数秒精度由微秒（6 位）变为毫秒（3 位）；
- 调用位置从裸 `文件:行` 变为 `source=文件:行`；
- 消息从原样拼接变为 `msg=...`，包含空格或特殊字符时可能加引号/转义；
- `Fatal` 级别输出为 `level=FATAL`。

Trace 字段也从旧的消息前缀变为结构化属性：

```text
旧：traceid: 4bf92f3577b34da6a3ce929d0e0e4736, hello
新：... msg=hello trace_id=4bf92f3577b34da6a3ce929d0e0e4736
```

如果日志采集、告警规则或 grep 依赖 `[zlog]`、`[INFO]`、`traceid:` 或旧时间格式，需要同步调整。默认仍是 Text；需要 JSON 时在初始化中传 `WithJSON()` 或 `WithOutputFormat(JSONOutput)`。

JSON 模式下 `source` 默认是对象（包含文件、行号等信息），不是 Text 的 `source=文件:行` 字符串；`trace_id` 也是普通 JSON 字段。JSON 行通常比 Text 更长，按 `MaxSize` 轮转时会更容易达到阈值。

### 离线清理接口变化

`CleanHisLog` 的签名保留，但语义已经收紧：

- 只识别日期、时间戳和数字序号格式的日志文件；
- `app.audit.2026-01-02.log`、`app.notes.log` 等相似前缀文件不参与清理；
- `keepCnt <= 0` 时至少保留最新一个文件；
- 公开接口没有活动文件参数，只适合确认没有活动 writer 的离线清理；
- 运行中的 writer 使用内部带活动文件名的清理逻辑；
- 自定义日期/时间戳布局使用 `CleanHisLogWithLayouts`，自定义时区使用 `CleanHisLogAt` 或前者。

旧代码如果依赖“删除同目录所有相似前缀文件”的行为，需要重新检查调用点。

文件自动清理按当前 writer 模式识别布局：纯日期 writer 主要识别日期文件，纯大小 writer 主要识别时间戳文件，日期与大小组合模式两种都识别；公开 `CleanHisLog` 默认同时识别日期和两种时间戳布局。

### 按大小轮转：迁移前后

旧写法，当前版本已不支持：

```go
zlog.InitZlog("logs/app", zlog.ALLLogLevel, zlog.AllModelLevel,
    zlog.WithLogFileManage(
        zlog.NewSizeCutMode("logs/app",
            zlog.WithCutSize(zlog.M50),
            zlog.WithKeepCnt(10),
        ),
    ),
)
```

新写法：

```go
zlog.InitZlog("logs/app", zlog.ALLLogLevel, zlog.AllModelLevel,
    zlog.WithRotateSize(50 << 20),
    zlog.WithRotateKeep(10),
    zlog.WithJSON(), // 不传时默认文本输出。
)
defer zlog.Close()
```

以前若 manager 的路径与 `InitZlog` 第一个参数不同，应把 manager 实际使用的路径迁移到第一个参数。现在没有第二个文件路径来源。路径表示文件名前缀，例如 `logs/app`，无需自行拼接时间戳和 `.log`。

### 按日期轮转与组合轮转

旧写法，当前版本已不支持：

```go
zlog.InitZlog("logs/app", zlog.ALLLogLevel, zlog.OnlyFile,
    zlog.WithLogFileManage(
        zlog.NewDateCutMode("logs/app",
            zlog.WithCutIntervalDay(2),
            zlog.WithKeepCnt(10),
        ),
    ),
)
```

新写法：

```go
zlog.InitZlog("logs/app", zlog.ALLLogLevel, zlog.OnlyFile,
    zlog.WithRotateDateInterval(2),
    zlog.WithRotateKeep(10),
)
defer zlog.Close()
```

默认文件输出是按日历日期轮转，间隔 1 天，最多保留 10 个文件（包含活动文件）。跨日轮转在下一条日志写入时检查，并非后台定时器主动切割。按 2 个日历日轮转不等于严格经过 48 小时；旧的 6/12/24 小时模式没有直接等价的新选项。

需要同时按大小和日期轮转：

```go
opts := []zlog.Option{
    zlog.WithRotateSize(50 << 20),
    zlog.WithRotateDate(true),
    zlog.WithRotateKeep(10),
    zlog.WithJSON(),
}
zlog.InitZlog("logs/app", zlog.ALLLogLevel, zlog.OnlyFile, opts...)
```

选项按传入顺序生效。`WithRotateSize` 会关闭日期轮转，因此组合模式中把 `WithRotateDate(true)` 或 `WithRotateDateInterval(days)` 放在它后面。多次设置相同配置时以后一次为准。

### 自定义 writer、文件名布局和小时轮转

直接使用过旧 manager 的 `GetFile/IsCut/Close`，或者实现过自定义 `LogFileManage` 的调用方，应迁移为标准 `io.Writer`。使用内置轮转 writer 的完整示例：

```go
package main

import (
    "log"
    "log/slog"

    "github.com/Zzaniu/tool/zlog"
)

func main() {
    writer, err := zlog.NewRotatingWriter(zlog.RotationConfig{
        FilePath:         "logs/app",
        MaxSize:          50 << 20,
        KeepCnt:          10,
        RotateByDate:     true, // 与 MaxSize 同时开启即为组合模式；只按大小轮转时设为 false。
        DateIntervalDays: 1,
        // 也可设置 DateLayout、TimeLayout 和 Location。
    })
    if err != nil {
        log.Fatal(err)
    }
    defer writer.Close()

    logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
        AddSource: true,
        Level:     slog.LevelInfo,
    }))
    logger.Info("服务启动", "component", "api")
}
```

直接使用 `RotationConfig` 时必须显式设置轮转模式和保留数量；它的零值不等于 `InitZlog` 的默认配置。例如 `KeepCnt: 0` 只保留活动文件。

上面的示例同时开启 `MaxSize` 和 `RotateByDate`，属于组合模式，文件名为时间戳格式；如果只需要按大小轮转，应把 `RotateByDate` 设为 `false` 或删除该字段。只按日期轮转时，把 `MaxSize` 设为 `0` 并保留 `RotateByDate: true`。

自定义输出也按这个方式把 writer 交给 `slog.NewTextHandler` 或 `slog.NewJSONHandler`。当前 `InitZlog` 不提供注入自定义 writer 的选项；这个独立 logger 应由业务自行持有和调用，调用全局 `zlog.Info` 不会写入它。

如必须保留固定小时数轮转，应选择或实现支持该模式的 writer，并交给 slog；当前内置 `RotatingWriter` 只提供大小和日历日期轮转。不要将 `WithCutIntervalTime(12*time.Hour)` 机械替换成 `WithRotateDateInterval(1)`。

`zlog` 内部使用 `log/slog`，但不会调用 `slog.SetDefault`。因此直接执行 `slog.Info(...)` 不会自动写入 zlog 的 RotatingWriter；需要使用 zlog 的全局方法，或按上面的方式自行创建并持有独立 logger。

需要从 Context 读取 ID，而不打印旧 `WithContext` 的调试输出：

```go
sc := trace.SpanContextFromContext(ctx)
var spanID, traceID string
if sc.HasSpanID() {
    spanID = sc.SpanID().String()
}
if sc.HasTraceID() {
    traceID = sc.TraceID().String()
}
```

这里的 `trace` 导入路径为 `go.opentelemetry.io/otel/trace`。使用独立 slog logger 时，可以显式附加 `logger.With("trace_id", traceID)`；`slog.InfoContext` 本身不会自动将 TraceID 添加为字段。

### 配置语义和历史日志

- `WithRotateSize` 的单位为字节，非正数关闭大小轮转。该选项同时把 `RotateByDate` 设为 `false`；需要日期轮转时，应在它之后调用 `WithRotateDate(true)` 或 `WithRotateDateInterval(days)`。旧 `WithCutSize` 对 1～9 的输入会改成 10 MiB；新选项不做这个特殊放大。迁移时应明确填写期望字节数。
- `WithRotateKeep(n)` 包含当前活动文件。`n <= 0` 只保留活动文件，`n=1/2` 分别保留 1/2 个文件；旧 `WithKeepCnt` 会把正数 1、2 提高到 3，新选项不再提高。
- `WithRotateDateInterval(0)` 不触发日期轮转；纯日期模式下仍可能使用日期文件名，只是不主动切割。随后调用 `WithRotateDate(true)` 会把非正数间隔恢复为 1 天。
- 文件模式下，`InitZlog` / `NewRotatingWriter` 在成功打开活动文件后会立即执行一次历史清理；不是只等到下一次轮转才清理。启动时可能删除超过 `KeepCnt` 的旧文件。
- `AllModelLevel` 同时写文件和控制台时，先写文件，再写 stdout；stdout 写失败不会阻断文件写入。
- 默认纯日期文件名为 `app.2026-11-28.log`；大小或组合轮转使用纳秒时间戳，重名时增加数值序号。不要继续依赖旧版固定 14 位时间戳的文件名解析方式。
- 仍保留对旧 14 位时间戳文件名的解析，用于历史日志归属判断和清理；这段解析逻辑有实际用途。旧日志只有与当前前缀、模式及可识别布局匹配时才参与清理，不会自动统一改名。
- `OnlyStd` 只使用输出编码配置，不打开文件；轮转选项在该模式下不会创建日志目录。
- 本次删除的是旧 manager、兼容适配器、过时工具函数和未使用字段。下面的 TraceID 与请求 logger 复用方法继续适用。

## 请求日志与 TraceID

本文介绍在 Gin 项目中复用带 TraceID 的 logger，以及通过请求 Context 跨函数复用同一个 logger。以下示例放在业务项目中，不需要修改 zlog 实现。

## 使用前提

- `zlog.WithTrace(ctx)` 从 OpenTelemetry SpanContext 中读取 TraceID，并为日志添加 `trace_id` 字段；它本身不生成 TraceID。
- `ctx` 没有有效 TraceID 时，`WithTrace` 返回全局 logger，日志不会自动出现 `trace_id`。
- `zlog.Info(...)` 等全局函数不读取当前 HTTP 请求上下文。单独执行 `c.Set("trace_id", id)` 也不能让 `WithTrace` 识别该 ID。
- 在 HTTP 服务启动前调用一次 `zlog.InitZlog`。日志级别是位掩码，例如仅设置 `INFO` 不代表同时开启 WARN 和 ERROR；下面示例使用 `ALLLogLevel`。
- 如果只需要日志关联，可以直接生成 ID 并放入 SpanContext，无需初始化 TracerProvider、Exporter 或接入链路采集服务。

## 方式一：在函数内复用 WithTrace 的返回值

函数内需要打印多条日志时，先获取一次 logger，后续使用其 `Info`、`Infof`、`Error`、`Errorf` 等方法。

下面的 `ctx` 应当来自已经注入 TraceID 的请求：

```go
package service

import (
    "context"

    "github.com/Zzaniu/tool/zlog"
)

func GetUser(ctx context.Context, id string) {
    logger := zlog.WithTrace(ctx)

    logger.Infof("开始查询用户 id=%s", id)
    queryUser(ctx, id)
    logger.Infof("查询用户完成 id=%s", id)
}

func queryUser(ctx context.Context, id string) {
    logger := zlog.WithTrace(ctx)

    logger.Infof("开始数据库查询 id=%s", id)
    // 使用 ctx 执行业务查询……
    logger.Infof("数据库查询完成 id=%s", id)
}
```

跨函数继续传递同一个请求的 Context，每个函数取一次 logger 即可。这里会创建两个 logger 包装对象，但它们读取的是同一个 TraceID，不会生成新的 ID。

如果希望连这一次跨函数构造也省掉，可以显式传递 `zlog.Logger`：

```go
package service

import (
    "context"

    "github.com/Zzaniu/tool/zlog"
)

func GetUser(ctx context.Context, id string) {
    logger := zlog.WithTrace(ctx)

    logger.Infof("开始查询用户 id=%s", id)
    queryUser(ctx, logger, id)
}

func queryUser(ctx context.Context, logger zlog.Logger, id string) {
    logger.Infof("开始数据库查询 id=%s", id)
    // 使用 ctx 执行业务查询……
}
```

这两个 service 示例是替代方案，不要把同名函数同时放入同一个包。显式传递 logger 的方式只构造一次，但会增加函数参数。希望函数之间只传 `ctx` 时，可以采用下面的封装。

## 方式二：在请求 Context 中复用 logger

请求入口创建一次 logger 并绑定到 Context，业务层统一通过 `requestlog.FromContext(ctx)` 获取。该封装属于业务项目，并不是 zlog 当前自带的 API。

示例项目结构：

```text
yourapp/
    requestlog/
        requestlog.go
    middleware.go
    main.go
```

下面假设业务项目的 module 为 `example.com/yourapp`。将代码中的 `example.com/yourapp/requestlog` 替换为自己项目的导入路径。业务项目需要能够导入 Gin、当前版本的 zlog 和 OpenTelemetry trace API。

### 1. 定义 requestlog 封装

文件：`requestlog/requestlog.go`。

```go
package requestlog

import (
    "context"

    "github.com/Zzaniu/tool/zlog"
)

// 使用私有 key 类型，避免与其他包的 Context 值冲突。
type loggerKey struct{}

// Bind 在请求入口调用。
// ctx 必须非 nil，且应当已经携带本次请求的 TraceID。
// 调用方需要继续传递返回的新 Context。
func Bind(ctx context.Context) context.Context {
    logger := zlog.WithTrace(ctx)
    return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext 优先返回已经绑定的请求 logger。
func FromContext(ctx context.Context) zlog.Logger {
    if ctx != nil {
        if logger, ok := ctx.Value(loggerKey{}).(zlog.Logger); ok {
            return logger
        }
    }

    // 没经过 Bind 的调用仍然可用，但这里不会自动缓存结果。
    return zlog.WithTrace(ctx)
}
```

### 2. 在 Gin 中间件中生成 TraceID 并绑定 logger

文件：`middleware.go`。

顺序必须是：准备带 TraceID 的 Context，然后 `Bind`，最后写回 `c.Request`。中间件自己的访问日志也复用绑定的 logger。

```go
package main

import (
    "crypto/rand"
    "net/http"
    "time"

    "example.com/yourapp/requestlog"
    "github.com/Zzaniu/tool/zlog"
    "github.com/gin-gonic/gin"
    "go.opentelemetry.io/otel/trace"
)

func TraceIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        ctx := c.Request.Context()
        sc := trace.SpanContextFromContext(ctx)

        // 已有完整的 SpanContext 时沿用，否则生成请求关联 ID。
        if !sc.IsValid() {
            var err error
            sc, err = newLogSpanContext()
            if err != nil {
                zlog.Errorf("生成 trace_id 失败: %v", err)
                c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
                    "message": "internal server error",
                })
                return
            }
            ctx = trace.ContextWithSpanContext(ctx, sc)
        }

        ctx = requestlog.Bind(ctx)
        c.Request = c.Request.WithContext(ctx)
        c.Header("X-Trace-ID", sc.TraceID().String())

        logger := requestlog.FromContext(ctx)

        c.Next()

        logger.Infof(
            "HTTP method=%s path=%s status=%d duration=%s",
            c.Request.Method,
            c.Request.URL.Path,
            c.Writer.Status(),
            time.Since(start),
        )
    }
}

// 只创建日志关联信息，不启动或记录真实的 Span。
// 同时生成 SpanID，使 SpanContext 保持有效；zlog 这里只输出 TraceID。
func newLogSpanContext() (trace.SpanContext, error) {
    for {
        var data [24]byte
        if _, err := rand.Read(data[:]); err != nil {
            return trace.SpanContext{}, err
        }

        var traceID trace.TraceID
        var spanID trace.SpanID
        copy(traceID[:], data[:16])
        copy(spanID[:], data[16:])

        sc := trace.NewSpanContext(trace.SpanContextConfig{
            TraceID: traceID,
            SpanID:  spanID,
        })
        if sc.IsValid() {
            return sc, nil
        }
    }
}
```

这个中间件将 TraceID 回传到响应头 `X-Trace-ID`，方便客户端提供 ID 查询日志。示例不解析请求中的 `X-Trace-ID` 或 `traceparent`；如果项目已经有注入 SpanContext 的中间件，应让它先于本中间件执行。

### 3. Handler、Service、Repository 之间只传 Context

文件：`main.go`。

```go
package main

import (
    "context"
    "net/http"

    "example.com/yourapp/requestlog"
    "github.com/Zzaniu/tool/zlog"
    "github.com/gin-gonic/gin"
)

func main() {
    zlog.InitZlog(
        "",
        zlog.ALLLogLevel,
        zlog.OnlyStd,
        zlog.WithJSON(),
    )
    defer zlog.Close()

    r := gin.New()
    r.Use(TraceIDMiddleware(), gin.Recovery())
    r.GET("/users/:id", getUserHandler)

    if err := r.Run(":8080"); err != nil {
        zlog.Errorf("HTTP 服务启动失败: %v", err)
    }
}

func getUserHandler(c *gin.Context) {
    ctx := c.Request.Context()
    id := c.Param("id")

    requestlog.FromContext(ctx).Info("收到查询请求")
    getUser(ctx, id)

    c.JSON(http.StatusOK, gin.H{"id": id})
}

func getUser(ctx context.Context, id string) {
    logger := requestlog.FromContext(ctx)

    logger.Infof("开始业务处理 id=%s", id)
    queryUser(ctx, id)
    logger.Infof("业务处理完成 id=%s", id)
}

func queryUser(ctx context.Context, id string) {
    requestlog.FromContext(ctx).Infof("开始数据库查询 id=%s", id)
    // 使用 ctx 执行业务查询……
}
```

中间件、Handler 和后续函数获取的是同一个 logger。同一请求的日志带有相同的 `trace_id`，例如以下输出，已省略时间和源码位置：

```json
{"level":"INFO","msg":"收到查询请求","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
{"level":"INFO","msg":"开始业务处理 id=123","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
{"level":"INFO","msg":"开始数据库查询 id=123","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
{"level":"INFO","msg":"业务处理完成 id=123","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
{"level":"INFO","msg":"HTTP method=GET path=/users/123 status=200 duration=2ms","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
```

### 使用约定

- 先注入 TraceID，再调用 `Bind`。如果先绑定没有 TraceID 的 logger，之后给 Context 添加 TraceID，并不会更新已经缓存的 logger。
- 业务日志统一使用 `requestlog.FromContext(ctx)`。当前 `zlog.WithTrace(ctx)` 不读取这里的自定义缓存，直接调用它仍会重新构造 logger。
- 继续向下传递绑定后的 Context，或基于它派生的 Context。中途换成 `context.Background()` 会丢失 TraceID 和缓存。
- 没有经过 `Bind` 时，`FromContext` 只执行回退逻辑，不会修改原 Context 或自动缓存；不能依靠重复调用它来实现懒加载缓存。
- 如果确实切换到了新的 TraceID，需要在新 Context 上重新 `Bind`，再把返回值传下去。
- 不要把某个请求的 logger 保存为所有请求共用的全局变量，否则其他请求会使用那个请求的 TraceID。
- 示例只关联通过上述 logger 输出的日志，不会自动改写 Gin Recovery、标准库或其他依赖自己打印的日志。
- 这种封装使函数参数保持简洁，但也让日志依赖变得隐式。要求显式依赖的项目可以选择传递 `zlog.Logger`。

## 性能说明（历史参考）

有 TraceID 时，当前 `WithTrace` 每次调用都会将 ID 转成字符串，创建带 `trace_id` 字段的 slog logger，并创建 `traceLog` 包装对象。它不会执行磁盘写入，但重复构造会增加内存分配。

以下数据来自 2026-11-28 兼容层清理前的版本，只作为历史参考；当前版本删除了旧字段和适配层，不能直接当作当前实现的精确性能基线。基准环境：Go 1.27.0、Windows/amd64、AMD Ryzen 7 9700X、`GOMAXPROCS=1`，每项运行 300ms、重复 5 次，表中耗时取中位数。测试保留日志编码和源码位置处理，输出到 `io.Discard`，不包含磁盘或终端 I/O；复用场景的首次 logger 构造位于计时之外。

| 调用方式 | 每条耗时 | 每条分配内存 | 每条分配次数 |
| --- | ---: | ---: | ---: |
| Text：复用 logger 后调用 Info | 883 ns | 504 B | 8 |
| Text：每条调用 WithTrace(ctx).Info | 1,183 ns | 888 B | 18 |
| JSON：复用 logger 后调用 Info | 1,129 ns | 640 B | 10 |
| JSON：每条调用 WithTrace(ctx).Info | 1,398 ns | 1,024 B | 20 |

在这次测试中，单独执行有 TraceID 的 `WithTrace(ctx)` 约为 0.22 微秒、384 B、10 次内存分配。即使 DEBUG 关闭，`WithTrace(ctx).Debug(...)` 仍然先完成 logger 构造，再判断日志级别。

这些数据来自清理前版本，是特定环境下的历史微基准，不代表本次删除未使用字段后的精确分配量，也不是实际 HTTP 吞吐量或磁盘日志性能保证。上表没有测量 `requestlog.FromContext` 的查找耗时；该封装命中缓存时只做 Context 取值和类型断言，避免再次构造 logger。函数内多次打印时，也可以先保存为局部变量以减少重复取值。

通常可以从“每个函数取一次 WithTrace”开始；需要统一跨函数复用且保持只有 Context 参数时，再采用请求级绑定方案。
