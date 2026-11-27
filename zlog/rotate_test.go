package zlog

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "testing"
    "time"
)

func listLogFiles(t *testing.T, dirName, baseName string) []string {
    t.Helper()

    entries, err := os.ReadDir(dirName)
    if err != nil {
        t.Fatalf("read dir: %v", err)
    }
    prefix := baseName + "."
    names := make([]string, 0)
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        name := entry.Name()
        if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".log") {
            names = append(names, name)
        }
    }
    sort.Strings(names)
    return names
}

func listOwnedLogFiles(t *testing.T, dirName, baseName string) []string {
    t.Helper()

    names := listLogFiles(t, dirName, baseName)
    owned := make([]string, 0, len(names))
    for _, name := range names {
        if _, ok := parseLogFile(baseName, name, []string{defaultDateLayout}, []string{defaultTimestampLayout, legacyTimestampLayout}, time.Local); ok {
            owned = append(owned, name)
        }
    }
    return owned
}

func readLogContent(t *testing.T, dirName, baseName string) string {
    t.Helper()

    var builder strings.Builder
    for _, name := range listLogFiles(t, dirName, baseName) {
        content, err := os.ReadFile(filepath.Join(dirName, name))
        if err != nil {
            t.Fatalf("read log %s: %v", name, err)
        }
        builder.Write(content)
    }
    return builder.String()
}

func TestRotatingWriterSizeRotation(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    fixedNow := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:  path,
        MaxSize:   100,
        KeepCnt:   3,
        TimeLayout: "20060102150405.000000000",
        Now:       func() time.Time { return fixedNow },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    first := strings.Repeat("a", 60)
    second := strings.Repeat("b", 60)
    if _, err := writer.Write([]byte(first)); err != nil {
        t.Fatalf("write first: %v", err)
    }
    if _, err := writer.Write([]byte(second)); err != nil {
        t.Fatalf("write second: %v", err)
    }

    names := listLogFiles(t, dirName, "app")
    if len(names) != 2 {
        t.Fatalf("expected 2 files, got %v", names)
    }

    content := readLogContent(t, dirName, "app")
    if !strings.Contains(content, first) || !strings.Contains(content, second) {
        t.Fatalf("log content missing data: %q", content)
    }
}

func TestRotatingWriterKeepCnt(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
    var tick int

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:  path,
        MaxSize:   10,
        KeepCnt:   2,
        TimeLayout: "20060102150405.000000000",
        Now: func() time.Time {
            tick++
            return now.Add(time.Duration(tick) * time.Second)
        },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    for i := 0; i < 5; i++ {
        if _, err := writer.Write([]byte(strings.Repeat("x", 20))); err != nil {
            t.Fatalf("write %d: %v", i, err)
        }
    }

    names := listLogFiles(t, dirName, "app")
    if len(names) != 2 {
        t.Fatalf("expected 2 files with KeepCnt=2, got %v", names)
    }
}

func TestRotatingWriterKeepZero(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:  path,
        MaxSize:   10,
        KeepCnt:   0,
        TimeLayout: "20060102150405.000000000",
        Now: func() time.Time {
            now = now.Add(time.Second)
            return now
        },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    for i := 0; i < 5; i++ {
        if _, err := writer.Write([]byte(strings.Repeat("x", 20))); err != nil {
            t.Fatalf("write %d: %v", i, err)
        }
    }

    names := listLogFiles(t, dirName, "app")
    if len(names) != 1 {
        t.Fatalf("expected only active file with KeepCnt=0, got %v", names)
    }
}

func TestRotatingWriterDateRotation(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    current := time.Date(2026, 1, 31, 10, 0, 0, 0, time.Local)

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:         path,
        KeepCnt:          3,
        RotateByDate:     true,
        DateIntervalDays: 1,
        Now:              func() time.Time { return current },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    if _, err := writer.Write([]byte("day-one\n")); err != nil {
        t.Fatalf("write day one: %v", err)
    }
    current = current.AddDate(0, 0, 1)
    if _, err := writer.Write([]byte("day-two\n")); err != nil {
        t.Fatalf("write day two: %v", err)
    }

    names := listLogFiles(t, dirName, "app")
    if len(names) != 2 {
        t.Fatalf("expected 2 date files, got %v", names)
    }
    if names[0] != "app.2026-01-31.log" || names[1] != "app.2026-02-01.log" {
        t.Fatalf("unexpected date files: %v", names)
    }
}

func TestCalendarDaysBetween(t *testing.T) {
    tests := []struct {
        from time.Time
        to   time.Time
        want int
    }{
        {
            from: time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local),
            to:   time.Date(2026, 1, 1, 23, 0, 0, 0, time.Local),
            want: 0,
        },
        {
            from: time.Date(2026, 1, 31, 23, 0, 0, 0, time.Local),
            to:   time.Date(2026, 2, 1, 1, 0, 0, 0, time.Local),
            want: 1,
        },
        {
            from: time.Date(2025, 12, 31, 23, 0, 0, 0, time.Local),
            to:   time.Date(2026, 1, 2, 1, 0, 0, 0, time.Local),
            want: 2,
        },
    }

    for _, test := range tests {
        if got := calendarDaysBetween(test.from, test.to); got != test.want {
            t.Fatalf("calendarDaysBetween(%v, %v) = %d, want %d", test.from, test.to, got, test.want)
        }
    }
}

func TestRotatingWriterCleanupIgnoresUnrelated(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)

    unrelatedLog := filepath.Join(dirName, "other.log")
    unrelatedText := filepath.Join(dirName, "app.notes.txt")
    unrelatedAudit := filepath.Join(dirName, "app.audit.2026-01-02.log")
    unrelatedDir := filepath.Join(dirName, "app.keep.log")
    if err := os.WriteFile(unrelatedLog, []byte("other"), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(unrelatedText, []byte("notes"), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(unrelatedAudit, []byte("audit"), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.Mkdir(unrelatedDir, 0755); err != nil {
        t.Fatal(err)
    }

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:  path,
        MaxSize:   10,
        KeepCnt:   1,
        TimeLayout: "20060102150405.000000000",
        Now: func() time.Time {
            now = now.Add(time.Second)
            return now
        },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    for i := 0; i < 4; i++ {
        if _, err := writer.Write([]byte(strings.Repeat("x", 20))); err != nil {
            t.Fatalf("write %d: %v", i, err)
        }
    }

    for _, name := range []string{"other.log", "app.notes.txt", "app.audit.2026-01-02.log"} {
        if _, err := os.Stat(filepath.Join(dirName, name)); err != nil {
            t.Fatalf("unrelated file %s was removed: %v", name, err)
        }
    }
    if info, err := os.Stat(unrelatedDir); err != nil || !info.IsDir() {
        t.Fatalf("unrelated directory was removed: %v", err)
    }

    names := listOwnedLogFiles(t, dirName, "app")
    if len(names) != 1 {
        t.Fatalf("expected only active log file, got %v", names)
    }
}

func TestRotatingWriterConcurrent(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath: path,
        KeepCnt:  10,
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    const goroutines = 8
    const writesPerGoroutine = 100
    var waitGroup sync.WaitGroup
    waitGroup.Add(goroutines)
    for i := 0; i < goroutines; i++ {
        go func(id int) {
            defer waitGroup.Done()
            for j := 0; j < writesPerGoroutine; j++ {
                if _, err := fmt.Fprintf(writer, "g=%d i=%d\n", id, j); err != nil {
                    t.Errorf("write failed: %v", err)
                    return
                }
            }
        }(i)
    }
    waitGroup.Wait()

    content := readLogContent(t, dirName, "app")
    wantLines := goroutines * writesPerGoroutine
    if got := strings.Count(content, "\n"); got != wantLines {
        t.Fatalf("got %d lines, want %d", got, wantLines)
    }
}

func TestRotatingWriterClose(t *testing.T) {
    dirName := t.TempDir()
    writer, err := NewRotatingWriter(RotationConfig{FilePath: filepath.Join(dirName, "app")})
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }

    if err := writer.Close(); err != nil {
        t.Fatalf("first close: %v", err)
    }
    if err := writer.Close(); err != nil {
        t.Fatalf("second close: %v", err)
    }
    if _, err := writer.Write([]byte("after close")); err != ErrClosed {
        t.Fatalf("write after close got %v, want ErrClosed", err)
    }
}

func TestCleanHisLogSafe(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")

    names := []string{
        "app.2020-01-01.log",
        "app.2021-01-01.log",
        "app.20200101000000.log",
        "app.audit.2026-01-02.log",
        "app.notes.log",
        "other.log",
        "app.notes.txt",
    }
    for _, name := range names {
        if err := os.WriteFile(filepath.Join(dirName, name), []byte(name), 0644); err != nil {
            t.Fatal(err)
        }
    }

    CleanHisLog(path, 1)

    if _, err := os.Stat(filepath.Join(dirName, "app.2021-01-01.log")); err != nil {
        t.Fatalf("newest date log missing: %v", err)
    }
    for _, name := range []string{"app.2020-01-01.log", "app.20200101000000.log"} {
        if _, err := os.Stat(filepath.Join(dirName, name)); !os.IsNotExist(err) {
            t.Fatalf("old log %s should be removed, got %v", name, err)
        }
    }
    for _, name := range []string{"app.audit.2026-01-02.log", "app.notes.log", "other.log", "app.notes.txt"} {
        if _, err := os.Stat(filepath.Join(dirName, name)); err != nil {
            t.Fatalf("unrelated file %s was removed: %v", name, err)
        }
    }
}

func BenchmarkRotatingWriterWrite(b *testing.B) {
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
    for i := 0; i < b.N; i++ {
        if _, err := writer.Write(line); err != nil {
            b.Fatalf("write: %v", err)
        }
    }
}

func TestParseLogFile(t *testing.T) {
    dateLayouts := []string{defaultDateLayout}
    timeLayouts := []string{defaultTimestampLayout, legacyTimestampLayout}
    tests := []struct {
        name string
        ok   bool
        seq  int
    }{
        {"app.2026-01-02.log", true, 0},
        {"app.20260102030405.log", true, 0},
        {"app.20260102030405.000000000.log", true, 0},
        {"app.20260102030405.1.log", true, 1},
        {"app.20260102030405.000000000.10.log", true, 10},
        {"app.audit.2026-01-02.log", false, 0},
        {"app.notes.log", false, 0},
        {"app.2020.log", false, 0},
        {"app.2026-01-02.1.log", false, 0},
        {"app.20260102030405.000000000.0.log", false, 0},
        {"app.20260102030405.000000000.01.log", false, 0},
        {"app.20260102030405.000000000.1.extra.log", false, 0},
        {"app..log", false, 0},
        {"other.2026-01-02.log", false, 0},
    }

    for _, test := range tests {
        meta, ok := parseLogFile("app", test.name, dateLayouts, timeLayouts, time.Local)
        if ok != test.ok {
            t.Fatalf("parse %s ok=%v, want %v", test.name, ok, test.ok)
        }
        if ok && meta.seq != test.seq {
            t.Fatalf("parse %s seq=%d, want %d", test.name, meta.seq, test.seq)
        }
    }
}

func TestCleanupLogDirKeepsActiveFile(t *testing.T) {
    dirName := t.TempDir()
    active := "app.2026-01-02.log"
    history := "app.20260103000000.000000000.log"
    audit := "app.audit.2026-01-02.log"

    for _, name := range []string{active, history, audit} {
        if err := os.WriteFile(filepath.Join(dirName, name), []byte(name), 0644); err != nil {
            t.Fatal(err)
        }
    }

    cleanupLogDir(
        dirName,
        "app",
        1,
        active,
        []string{defaultDateLayout},
        []string{defaultTimestampLayout, legacyTimestampLayout},
        time.Local,
    )

    if _, err := os.Stat(filepath.Join(dirName, active)); err != nil {
        t.Fatalf("active file removed: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dirName, history)); !os.IsNotExist(err) {
        t.Fatalf("history file should be removed, got %v", err)
    }
    if _, err := os.Stat(filepath.Join(dirName, audit)); err != nil {
        t.Fatalf("audit file removed: %v", err)
    }
}

func TestRotatingWriterMixedDateSizeOrder(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local)

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:         path,
        MaxSize:          10,
        KeepCnt:          2,
        RotateByDate:     true,
        DateIntervalDays: 1,
        Now:              func() time.Time { return now },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }

    write := func(value string) {
        if _, err := writer.Write([]byte(strings.Repeat(value, 20))); err != nil {
            t.Fatalf("write %s: %v", value, err)
        }
    }
    write("a")
    write("b")

    now = time.Date(2026, 1, 2, 9, 0, 0, 0, time.Local)
    write("c")
    write("d")
    if err := writer.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    names := listOwnedLogFiles(t, dirName, "app")
    if len(names) != 2 {
        t.Fatalf("expected 2 files, got %v", names)
    }
    expected := map[string]bool{
        "app.20260102090000.000000000.log":   true,
        "app.20260102090000.000000000.1.log": true,
    }
    for _, name := range names {
        if !expected[name] {
            t.Fatalf("unexpected retained file %s, all=%v", name, names)
        }
    }

    content := readLogContent(t, dirName, "app")
    if !strings.Contains(content, strings.Repeat("c", 20)) || !strings.Contains(content, strings.Repeat("d", 20)) {
        t.Fatalf("newest combination records missing: %q", content)
    }
    if strings.Contains(content, strings.Repeat("a", 20)) || strings.Contains(content, strings.Repeat("b", 20)) {
        t.Fatalf("older combination records should be removed: %q", content)
    }
}

func TestRotatingWriterSequenceNoReuse(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath: path,
        MaxSize:  2,
        KeepCnt:  2,
        Now:      func() time.Time { return fixed },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }

    for _, value := range []string{"A", "B", "C", "D", "E"} {
        if _, err := writer.Write([]byte(value + "\n")); err != nil {
            t.Fatalf("write %s: %v", value, err)
        }
    }
    if err := writer.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    names := listOwnedLogFiles(t, dirName, "app")
    if len(names) != 2 {
        t.Fatalf("expected 2 files, got %v", names)
    }
    content := readLogContent(t, dirName, "app")
    lines := strings.Fields(content)
    if len(lines) != 2 || lines[0] != "D" || lines[1] != "E" {
        t.Fatalf("expected D,E, got %v (files=%v)", lines, names)
    }
}

func TestRotatingWriterRestartKeepCnt(t *testing.T) {
    t.Run("date", func(t *testing.T) {
        dirName := t.TempDir()
        path := filepath.Join(dirName, "app")
        for i := 0; i < 5; i++ {
            at := time.Date(2026, 1, 1+i, 10, 0, 0, 0, time.Local)
            writer, err := NewRotatingWriter(RotationConfig{
                FilePath:         path,
                KeepCnt:          2,
                RotateByDate:     true,
                DateIntervalDays: 1,
                Now:              func() time.Time { return at },
            })
            if err != nil {
                t.Fatalf("new writer %d: %v", i, err)
            }
            if _, err := writer.Write([]byte("line\n")); err != nil {
                t.Fatalf("write %d: %v", i, err)
            }
            if err := writer.Close(); err != nil {
                t.Fatalf("close %d: %v", i, err)
            }
        }
        names := listOwnedLogFiles(t, dirName, "app")
        if len(names) != 2 {
            t.Fatalf("expected 2 date files, got %v", names)
        }
    })

    t.Run("size", func(t *testing.T) {
        dirName := t.TempDir()
        path := filepath.Join(dirName, "app")
        for i := 0; i < 5; i++ {
            at := time.Date(2026, 1, 1, 10, 0, i, 0, time.Local)
            writer, err := NewRotatingWriter(RotationConfig{
                FilePath: path,
                KeepCnt:  2,
                Now:      func() time.Time { return at },
            })
            if err != nil {
                t.Fatalf("new writer %d: %v", i, err)
            }
            if _, err := writer.Write([]byte("line\n")); err != nil {
                t.Fatalf("write %d: %v", i, err)
            }
            if err := writer.Close(); err != nil {
                t.Fatalf("close %d: %v", i, err)
            }
        }
        names := listOwnedLogFiles(t, dirName, "app")
        if len(names) != 2 {
            t.Fatalf("expected 2 size files, got %v", names)
        }
    })
}

func TestRotatingWriterCleanupModeOwnership(t *testing.T) {
    t.Run("size does not clean date files", func(t *testing.T) {
        dirName := t.TempDir()
        path := filepath.Join(dirName, "app")
        dateFile := filepath.Join(dirName, "app.2026-01-01.log")
        if err := os.WriteFile(dateFile, []byte("date"), 0644); err != nil {
            t.Fatal(err)
        }

        now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
        writer, err := NewRotatingWriter(RotationConfig{
            FilePath: path,
            MaxSize:  10,
            KeepCnt:  1,
            Now:      func() time.Time { return now },
        })
        if err != nil {
            t.Fatalf("new writer: %v", err)
        }
        for i := 0; i < 3; i++ {
            if _, err := writer.Write([]byte(strings.Repeat("x", 20))); err != nil {
                t.Fatalf("write %d: %v", i, err)
            }
        }
        if err := writer.Close(); err != nil {
            t.Fatalf("close: %v", err)
        }
        if _, err := os.Stat(dateFile); err != nil {
            t.Fatalf("date-mode file removed by size writer: %v", err)
        }
    })

    t.Run("date does not clean timestamp files", func(t *testing.T) {
        dirName := t.TempDir()
        path := filepath.Join(dirName, "app")
        timestampFile := filepath.Join(dirName, "app.20260101100000.000000000.log")
        if err := os.WriteFile(timestampFile, []byte("timestamp"), 0644); err != nil {
            t.Fatal(err)
        }

        now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
        writer, err := NewRotatingWriter(RotationConfig{
            FilePath:         path,
            KeepCnt:          1,
            RotateByDate:     true,
            DateIntervalDays: 1,
            Now:              func() time.Time { return now },
        })
        if err != nil {
            t.Fatalf("new writer: %v", err)
        }
        if _, err := writer.Write([]byte("line\n")); err != nil {
            t.Fatalf("write: %v", err)
        }
        if err := writer.Close(); err != nil {
            t.Fatalf("close: %v", err)
        }
        if _, err := os.Stat(timestampFile); err != nil {
            t.Fatalf("size-mode file removed by date writer: %v", err)
        }
    })
}

func TestCleanHisLogKeepZero(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    for _, name := range []string{"app.2020-01-01.log", "app.2021-01-01.log"} {
        if err := os.WriteFile(filepath.Join(dirName, name), []byte(name), 0644); err != nil {
            t.Fatal(err)
        }
    }

    CleanHisLog(path, 0)

    if _, err := os.Stat(filepath.Join(dirName, "app.2021-01-01.log")); err != nil {
        t.Fatalf("newest log should be kept: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dirName, "app.2020-01-01.log")); !os.IsNotExist(err) {
        t.Fatalf("old log should be removed, got %v", err)
    }
}

func TestRotatingWriterConcurrentRotation(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath: path,
        MaxSize:  256,
        KeepCnt:  10000,
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    defer writer.Close()

    const workers = 8
    const writesPerWorker = 100
    var waitGroup sync.WaitGroup
    waitGroup.Add(workers)
    for worker := 0; worker < workers; worker++ {
        go func(worker int) {
            defer waitGroup.Done()
            for index := 0; index < writesPerWorker; index++ {
                if _, err := fmt.Fprintf(writer, "g%02d-i%03d\n", worker, index); err != nil {
                    t.Errorf("write worker=%d index=%d: %v", worker, index, err)
                    return
                }
            }
        }(worker)
    }
    waitGroup.Wait()

    if err := writer.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    content := readLogContent(t, dirName, "app")
    lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
    if len(lines) != workers*writesPerWorker {
        t.Fatalf("got %d lines, want %d", len(lines), workers*writesPerWorker)
    }
    seen := make(map[string]bool, len(lines))
    for _, line := range lines {
        if seen[line] {
            t.Fatalf("duplicate line %q", line)
        }
        seen[line] = true
    }
}

func TestRotatingWriterMixedRestartKeepsNewest(t *testing.T) {
    dirName := t.TempDir()
    now := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
    cfg := RotationConfig{
        FilePath:         filepath.Join(dirName, "app"),
        MaxSize:          2,
        KeepCnt:          2,
        RotateByDate:     true,
        DateIntervalDays: 1,
        Now:              func() time.Time { return now },
    }

    before, err := NewRotatingWriter(cfg)
    if err != nil {
        t.Fatalf("new writer before restart: %v", err)
    }
    for _, value := range []string{"A", "B", "C"} {
        if _, err := before.Write([]byte(value + "\n")); err != nil {
            t.Fatalf("write %s: %v", value, err)
        }
    }
    if err := before.Close(); err != nil {
        t.Fatalf("close before restart: %v", err)
    }

    now = now.Add(time.Hour)
    after, err := NewRotatingWriter(cfg)
    if err != nil {
        t.Fatalf("new writer after restart: %v", err)
    }
    for _, value := range []string{"D", "E"} {
        if _, err := after.Write([]byte(value + "\n")); err != nil {
            t.Fatalf("write %s: %v", value, err)
        }
    }
    if err := after.Close(); err != nil {
        t.Fatalf("close after restart: %v", err)
    }

    records := strings.Fields(readLogContent(t, dirName, "app"))
    sort.Strings(records)
    if got := strings.Join(records, ","); got != "D,E" {
        t.Fatalf("records=%s want=D,E files=%v", got, listOwnedLogFiles(t, dirName, "app"))
    }
}

func TestParseLogFileLocation(t *testing.T) {
    location := time.FixedZone("UTC+8", 8*60*60)
    dateMeta, ok := parseLogFile(
        "app",
        "app.2026-01-02.log",
        []string{defaultDateLayout},
        []string{"20060102150405-0700"},
        location,
    )
    if !ok {
        t.Fatal("date file should parse")
    }
    if got := dateMeta.generated.Format(time.RFC3339); got != "2026-01-02T00:00:00+08:00" {
        t.Fatalf("date generated=%s", got)
    }

    timestampMeta, ok := parseLogFile(
        "app",
        "app.20260102010000+0800.log",
        []string{defaultDateLayout},
        []string{"20060102150405-0700"},
        location,
    )
    if !ok {
        t.Fatal("zoned timestamp should parse")
    }
    if got := timestampMeta.generated.UTC().Format(time.RFC3339); got != "2026-01-01T17:00:00Z" {
        t.Fatalf("timestamp generated=%s", got)
    }
}

func TestCleanupLogDirZonedTimestampKeepsNewest(t *testing.T) {
    dirName := t.TempDir()
    location := time.FixedZone("UTC+8", 8*60*60)
    active := "app.20260102020000+0800.log"
    historyDate := "app.2026-01-02.log"
    historyTimestamp := "app.20260102010000+0800.log"

    for _, name := range []string{active, historyDate, historyTimestamp} {
        if err := os.WriteFile(filepath.Join(dirName, name), []byte(name), 0644); err != nil {
            t.Fatal(err)
        }
    }

    cleanupLogDir(
        dirName,
        "app",
        2,
        active,
        []string{defaultDateLayout},
        []string{"20060102150405-0700"},
        location,
    )

    if _, err := os.Stat(filepath.Join(dirName, historyDate)); !os.IsNotExist(err) {
        t.Fatalf("older date history should be removed, got %v", err)
    }
    if _, err := os.Stat(filepath.Join(dirName, historyTimestamp)); err != nil {
        t.Fatalf("newer timestamp history missing: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dirName, active)); err != nil {
        t.Fatalf("active file missing: %v", err)
    }
}

func TestRotatingWriterExplicitLocationNaming(t *testing.T) {
    dirName := t.TempDir()
    now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
    location := time.FixedZone("UTC+8", 8*60*60)

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:    filepath.Join(dirName, "app"),
        Now:         func() time.Time { return now },
        Location:    location,
        TimeLayout:  defaultTimestampLayout,
        MaxSize:     0,
        RotateByDate: false,
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    if err := writer.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    expected := "app.20260102080000.000000000.log"
    if _, err := os.Stat(filepath.Join(dirName, expected)); err != nil {
        t.Fatalf("expected file named in explicit location: %v; files=%v", err, listLogFiles(t, dirName, "app"))
    }
}

func TestCleanHisLogAtUsesLocation(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    location := time.FixedZone("UTC+8", 8*60*60)
    older := "app.2026-01-02.log"
    newer := "app.20260102010000.log"

    for _, name := range []string{older, newer} {
        if err := os.WriteFile(filepath.Join(dirName, name), []byte(name), 0644); err != nil {
            t.Fatal(err)
        }
    }

    CleanHisLogAt(path, 1, location)

    if _, err := os.Stat(filepath.Join(dirName, older)); !os.IsNotExist(err) {
        t.Fatalf("older date file should be removed, got %v", err)
    }
    if _, err := os.Stat(filepath.Join(dirName, newer)); err != nil {
        t.Fatalf("newer timestamp file missing: %v", err)
    }
}

func TestRotatingWriterDateRotationUsesLocation(t *testing.T) {
    dirName := t.TempDir()
    location := time.FixedZone("UTC+8", 8*60*60)
    current := time.Date(2026, 1, 1, 16, 0, 0, 0, time.UTC) // 2026-01-02 00:00 +08:00

    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:         filepath.Join(dirName, "app"),
        KeepCnt:          3,
        RotateByDate:     true,
        DateIntervalDays: 1,
        Location:         location,
        Now:              func() time.Time { return current },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }

    if _, err := writer.Write([]byte("a\n")); err != nil {
        t.Fatalf("write a: %v", err)
    }
    current = time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC) // same local date
    if _, err := writer.Write([]byte("b\n")); err != nil {
        t.Fatalf("write b: %v", err)
    }
    current = time.Date(2026, 1, 2, 16, 0, 0, 0, time.UTC) // next local date
    if _, err := writer.Write([]byte("c\n")); err != nil {
        t.Fatalf("write c: %v", err)
    }
    if err := writer.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    names := listLogFiles(t, dirName, "app")
    expected := map[string]bool{
        "app.2026-01-02.log": true,
        "app.2026-01-03.log": true,
    }
    if len(names) != 2 {
        t.Fatalf("expected 2 local-date files, got %v", names)
    }
    for _, name := range names {
        if !expected[name] {
            t.Fatalf("unexpected date file %s", name)
        }
    }
}

func TestRotatingWriterZonedCleanupUsesLocation(t *testing.T) {
    dirName := t.TempDir()
    location := time.FixedZone("UTC+8", 8*60*60)
    historyDate := filepath.Join(dirName, "app.2026-01-02.log")
    historyTimestamp := filepath.Join(dirName, "app.20260102010000+0800.log")
    if err := os.WriteFile(historyDate, []byte("A\n"), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(historyTimestamp, []byte("B\n"), 0644); err != nil {
        t.Fatal(err)
    }

    now := time.Date(2026, 1, 2, 2, 0, 0, 0, location)
    writer, err := NewRotatingWriter(RotationConfig{
        FilePath:         filepath.Join(dirName, "app"),
        MaxSize:          2,
        KeepCnt:          2,
        RotateByDate:     true,
        DateIntervalDays: 1,
        TimeLayout:       "20060102150405-0700",
        Location:         location,
        Now:              func() time.Time { return now },
    })
    if err != nil {
        t.Fatalf("new writer: %v", err)
    }
    if _, err := writer.Write([]byte("C\n")); err != nil {
        t.Fatalf("write C: %v", err)
    }
    if err := writer.Close(); err != nil {
        t.Fatalf("close: %v", err)
    }

    records := strings.Fields(readLogContent(t, dirName, "app"))
    sort.Strings(records)
    if got := strings.Join(records, ","); got != "B,C" {
        t.Fatalf("records=%s want=B,C files=%v", got, listLogFiles(t, dirName, "app"))
    }
    if _, err := os.Stat(historyDate); !os.IsNotExist(err) {
        t.Fatalf("older date A should be removed, got %v", err)
    }
}

func TestCleanHisLogWithLayoutsUsesLocation(t *testing.T) {
    dirName := t.TempDir()
    path := filepath.Join(dirName, "app")
    location := time.FixedZone("UTC+8", 8*60*60)
    older := "app.2026-01-02.log"
    newer := "app.20260102010000+0800.log"

    for _, name := range []string{older, newer} {
        if err := os.WriteFile(filepath.Join(dirName, name), []byte(name), 0644); err != nil {
            t.Fatal(err)
        }
    }

    CleanHisLogWithLayouts(path, 1, defaultDateLayout, "20060102150405-0700", location)

    if _, err := os.Stat(filepath.Join(dirName, older)); !os.IsNotExist(err) {
        t.Fatalf("older date file should be removed, got %v", err)
    }
    if _, err := os.Stat(filepath.Join(dirName, newer)); err != nil {
        t.Fatalf("newer zoned timestamp file missing: %v", err)
    }
}
