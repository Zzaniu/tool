package zlog

import (
    "fmt"
    "log"
    "log/slog"
    "os"
    "path/filepath"
    "testing"
)

func BenchmarkLoggerInfofParallel(b *testing.B) {
    logger := newZlog(filepath.Join(b.TempDir(), "app"), ALLLogLevel, nil)
    defer logger.Close()

    b.ResetTimer()
    b.SetBytes(int64(len("benchmark line 000000")))
    b.RunParallel(func(pb *testing.PB) {
        index := 0
        for pb.Next() {
            logger.Infof("benchmark line %d", index)
            index++
        }
    })
}

func BenchmarkLoggerInfofSizeParallel(b *testing.B) {
    logger := newZlog(
        filepath.Join(b.TempDir(), "app"),
        ALLLogLevel,
        []Option{WithRotateSize(1 << 30), WithRotateKeep(10)},
    )
    defer logger.Close()

    b.ResetTimer()
    b.SetBytes(int64(len("benchmark line 000000")))
    b.RunParallel(func(pb *testing.PB) {
        index := 0
        for pb.Next() {
            logger.Infof("benchmark line %d", index)
            index++
        }
    })
}

func BenchmarkRotatingWriterWriteParallel(b *testing.B) {
    writer, err := NewRotatingWriter(RotationConfig{
        FilePath: filepath.Join(b.TempDir(), "app"),
    })
    if err != nil {
        b.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    line := []byte("2026-01-02T03:04:05.000000Z level=INFO msg=benchmark\n")
    b.SetBytes(int64(len(line)))
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            if _, err := writer.Write(line); err != nil {
                b.Errorf("write: %v", err)
                return
            }
        }
    })
}

func BenchmarkRotatingWriterRotation(b *testing.B) {
    writer, err := NewRotatingWriter(RotationConfig{
        FilePath: filepath.Join(b.TempDir(), "app"),
        MaxSize:  64 * 1024,
        KeepCnt:  10,
    })
    if err != nil {
        b.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    line := []byte("2026-01-02T03:04:05.000000Z level=INFO msg=benchmark\n")
    b.SetBytes(int64(len(line)))
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        if _, err := writer.Write(line); err != nil {
            b.Fatalf("write: %v", err)
        }
    }
}

func BenchmarkLoggerInfofSizeRotation(b *testing.B) {
    logger := newZlog(
        filepath.Join(b.TempDir(), "app"),
        ALLLogLevel,
        []Option{WithRotateSize(64 << 10), WithRotateKeep(10)},
    )
    defer logger.Close()

    b.ResetTimer()
    b.SetBytes(int64(len("benchmark line 000000")))
    for i := 0; i < b.N; i++ {
        logger.Infof("benchmark line %d", i)
    }
}

func BenchmarkSlogTextFile(b *testing.B) {
    file, err := os.OpenFile(filepath.Join(b.TempDir(), "slog.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        b.Fatal(err)
    }
    defer file.Close()

    logger := slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
        AddSource: true,
        Level:     slog.LevelDebug,
    }))

    b.ResetTimer()
    b.SetBytes(int64(len("benchmark line 000000")))
    for i := 0; i < b.N; i++ {
        logger.Info(fmt.Sprintf("benchmark line %d", i))
    }
}

func BenchmarkStdLogFile(b *testing.B) {
    file, err := os.OpenFile(filepath.Join(b.TempDir(), "std.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        b.Fatal(err)
    }
    defer file.Close()

    logger := log.New(file, "", log.Llongfile|log.Ldate|log.Lmicroseconds)

    b.ResetTimer()
    b.SetBytes(int64(len("benchmark line 000000")))
    for i := 0; i < b.N; i++ {
        if err := logger.Output(2, fmt.Sprintf("benchmark line %d", i)); err != nil {
            b.Fatalf("output: %v", err)
        }
    }
}

func BenchmarkLoggerInfofJSON(b *testing.B) {
    logger := newZlog(
        filepath.Join(b.TempDir(), "app"),
        ALLLogLevel,
        []Option{WithJSON()},
    )
    defer logger.Close()

    b.ResetTimer()
    b.SetBytes(int64(len("benchmark line 000000")))
    for i := 0; i < b.N; i++ {
        logger.Infof("benchmark line %d", i)
    }
}
