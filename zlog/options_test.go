package zlog

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestRotationOptions(t *testing.T) {
    tests := []struct {
        name    string
        setters []Option
        size    int64
        keep    int
        byDate  bool
        days    int
    }{
        {"defaults", nil, 0, 10, true, 1},
        {"size only", []Option{WithRotateSize(64)}, 64, 10, false, 1},
        {"date then size", []Option{WithRotateDate(true), WithRotateSize(64)}, 64, 10, false, 1},
        {"size then date", []Option{WithRotateSize(64), WithRotateDate(true)}, 64, 10, true, 1},
        {"keep then size", []Option{WithRotateKeep(2), WithRotateSize(64)}, 64, 2, false, 1},
        {"size then keep", []Option{WithRotateSize(64), WithRotateKeep(2)}, 64, 2, false, 1},
        {"last keep wins", []Option{WithRotateKeep(3), WithRotateKeep(1)}, 0, 1, true, 1},
        {"negative size and keep", []Option{WithRotateSize(-1), WithRotateKeep(-1)}, 0, 0, false, 1},
        {"date interval", []Option{WithRotateDateInterval(3)}, 0, 10, true, 3},
        {"zero date interval", []Option{WithRotateDateInterval(0)}, 0, 10, true, 0},
        {"negative date interval", []Option{WithRotateDateInterval(-1)}, 0, 10, true, -1},
        {"reenable date", []Option{WithRotateDateInterval(0), WithRotateDate(true)}, 0, 10, true, 1},
    }

    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            cfg := resolveOptions("logs/app", test.setters).rotation
            if cfg.FilePath != "logs/app" || cfg.MaxSize != test.size || cfg.KeepCnt != test.keep ||
                cfg.RotateByDate != test.byDate || cfg.DateIntervalDays != test.days {
                t.Fatalf("unexpected configuration: %+v", cfg)
            }
        })
    }
}

// 初始化由全局 sync.Once 控制，使用独立进程验证公开入口，避免重置共享状态。
func TestInitZlogModes(t *testing.T) {
    modes := []struct {
        name   string
        mode   ModelLevel
        stdout bool
        file   bool
    }{
        {"stdout", OnlyStd, true, false},
        {"file", OnlyFile, false, true},
        {"both", AllModelLevel, true, true},
    }
    for _, mode := range modes {
        for _, format := range []OutputFormat{TextOutput, JSONOutput} {
            t.Run(fmt.Sprintf("%s/format=%d", mode.name, format), func(t *testing.T) {
                dirName := t.TempDir()
                parent := filepath.Join(dirName, "logs")
                path := filepath.Join(parent, "app")
                if !mode.file {
                    // 此路径不可能创建日志文件；OnlyStd 必须忽略文件输出配置。
                    if err := os.WriteFile(parent, []byte("blocker"), 0644); err != nil {
                        t.Fatal(err)
                    }
                }

                cmd := exec.Command(os.Args[0], "-test.run=^TestInitZlogProcess$")
                cmd.Env = append(os.Environ(),
                    fmt.Sprintf("ZLOG_TEST_INIT_MODE=%d", mode.mode),
                    fmt.Sprintf("ZLOG_TEST_INIT_FORMAT=%d", format),
                    "ZLOG_TEST_INIT_PATH="+path,
                )
                output, err := cmd.CombinedOutput()
                if err != nil {
                    t.Fatalf("initialization process: %v\n%s", err, output)
                }

                containsLog := strings.Contains(string(output), "init-api-check")
                if containsLog != mode.stdout {
                    t.Fatalf("stdout=%v, want %v: %s", containsLog, mode.stdout, output)
                }
                if mode.stdout {
                    assertInitRecord(t, string(output), format)
                }
                if mode.file {
                    files, err := filepath.Glob(path + ".*.log")
                    if err != nil || len(files) != 1 {
                        t.Fatalf("log files=%v, error=%v", files, err)
                    }
                    content, err := os.ReadFile(files[0])
                    if err != nil {
                        t.Fatal(err)
                    }
                    assertInitRecord(t, string(content), format)
                }
            })
        }
    }
}

func TestInitZlogProcess(t *testing.T) {
    rawMode := os.Getenv("ZLOG_TEST_INIT_MODE")
    if rawMode == "" {
        t.Skip("subprocess helper")
    }
    var mode ModelLevel
    var format OutputFormat
    if _, err := fmt.Sscan(rawMode, &mode); err != nil {
        t.Fatal(err)
    }
    if _, err := fmt.Sscan(os.Getenv("ZLOG_TEST_INIT_FORMAT"), &format); err != nil {
        t.Fatal(err)
    }

    InitZlog(os.Getenv("ZLOG_TEST_INIT_PATH"), INFO, mode,
        WithRotateSize(256),
        WithRotateKeep(2),
        WithOutputFormat(format),
    )
    // 重复初始化不应替换第一个 logger。
    InitZlog("", DEBUG, OnlyStd)
    Info("init-api-check")
    if err := Close(); err != nil {
        t.Fatal(err)
    }
}

func assertInitRecord(t *testing.T, output string, format OutputFormat) {
    t.Helper()
    count := 0
    for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
        if !strings.Contains(line, "init-api-check") {
            continue
        }
        count++
        if format == JSONOutput {
            var record map[string]any
            if err := json.Unmarshal([]byte(line), &record); err != nil {
                t.Fatalf("invalid JSON: %v, line=%q", err, line)
            }
            if record["msg"] != "init-api-check" || record["level"] != "INFO" {
                t.Fatalf("unexpected record: %v", record)
            }
        } else if !strings.Contains(line, "level=INFO") {
            t.Fatalf("unexpected text log: %q", line)
        }
    }
    if count != 1 {
        t.Fatalf("got %d records, want 1: %s", count, output)
    }
}
