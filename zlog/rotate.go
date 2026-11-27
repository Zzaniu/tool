package zlog

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"
)

const (
    defaultDateLayout      = "2006-01-02"
    defaultTimestampLayout = "20060102150405.000000000"
    legacyTimestampLayout  = "20060102150405"
)

type rotationReason int

const (
    rotateNone rotationReason = iota
    rotateByDate
    rotateBySize
)

// ErrClosed 表示日志 writer 已经关闭。
var ErrClosed = errors.New("zlog: log writer is closed")

type (
    // RotationConfig 是 RotatingWriter 的配置。
    RotationConfig struct {
        FilePath         string
        MaxSize          int64
        KeepCnt          int
        RotateByDate     bool
        DateIntervalDays int
        DateLayout       string
        TimeLayout       string
        Now              func() time.Time
        Location         *time.Location
    }

    // RotatingWriter 是并发安全、按大小或日期轮转的 io.Writer。
    RotatingWriter struct {
        mu          sync.Mutex
        cfg         RotationConfig
        file        *os.File
        size        int64
        lastRotate  time.Time
        sizePrefix  string
        nextSizeSeq int
        closed      bool
    }

    logFileMeta struct {
        name      string
        generated time.Time
        seq       int
    }
)

// NewRotatingWriter 创建日志 writer 并打开当前日志文件。
func NewRotatingWriter(cfg RotationConfig) (*RotatingWriter, error) {
    cfg = normalizeRotationConfig(cfg)
    if cfg.FilePath == "" {
        return nil, errors.New("zlog: empty log file path")
    }

    if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0755); err != nil {
        return nil, fmt.Errorf("zlog: create log dir failed: %w", err)
    }

    now := cfg.Now()
    if cfg.Location == nil {
        cfg.Location = now.Location()
    }
    if cfg.Location == nil {
        cfg.Location = time.Local
    }

    writer := &RotatingWriter{
        cfg:        cfg,
        lastRotate: now,
    }

    file, size, err := writer.openFileLocked(writer.lastRotate, writer.useDateFileName())
    if err != nil {
        return nil, err
    }
    writer.file = file
    writer.size = size
    writer.cleanupLocked()
    return writer, nil
}

func normalizeRotationConfig(cfg RotationConfig) RotationConfig {
    if cfg.DateLayout == "" {
        cfg.DateLayout = defaultDateLayout
    }
    if cfg.TimeLayout == "" {
        cfg.TimeLayout = defaultTimestampLayout
    }
    if cfg.Now == nil {
        cfg.Now = time.Now
    }
    if cfg.MaxSize < 0 {
        cfg.MaxSize = 0
    }
    if cfg.KeepCnt < 0 {
        cfg.KeepCnt = 0
    }
    return cfg
}

// Write 实现 io.Writer。
//
// 切割检查只使用内存中的 size 和 lastRotate，不做逐行 Stat。
func (w *RotatingWriter) Write(p []byte) (int, error) {
    w.mu.Lock()
    defer w.mu.Unlock()

    if w.closed {
        return 0, ErrClosed
    }

    now := w.cfg.Now()
    reason := w.rotationReason(now, len(p))
    if reason != rotateNone {
        if err := w.rotateLocked(now, reason); err != nil {
            // 切割失败时保留旧文件继续写，避免日志直接丢失。
            reportLogError("rotate", err)
        }
    }

    if w.file == nil {
        return 0, ErrClosed
    }

    n, err := w.file.Write(p)
    w.size += int64(n)
    if err != nil {
        reportLogError("write", err)
    }
    return n, err
}

// Close 关闭当前文件，重复调用返回 nil。
func (w *RotatingWriter) Close() error {
    w.mu.Lock()
    defer w.mu.Unlock()

    if w.closed {
        return nil
    }
    w.closed = true
    if w.file == nil {
        return nil
    }

    err := w.file.Close()
    w.file = nil
    if err != nil {
        reportLogError("close", err)
    }
    return err
}

func (w *RotatingWriter) rotationReason(now time.Time, writeSize int) rotationReason {
    if w.cfg.RotateByDate && w.cfg.DateIntervalDays > 0 {
        location := w.logLocation()
        if calendarDaysBetween(w.lastRotate.In(location), now.In(location)) >= w.cfg.DateIntervalDays {
            return rotateByDate
        }
    }
    if w.cfg.MaxSize > 0 && w.size > 0 && w.size+int64(writeSize) > w.cfg.MaxSize {
        return rotateBySize
    }
    return rotateNone
}

func (w *RotatingWriter) rotateLocked(now time.Time, reason rotationReason) error {
    byDate := reason == rotateByDate && w.useDateFileName()
    file, size, err := w.openFileLocked(now, byDate)
    if err != nil {
        return err
    }

    oldFile := w.file
    w.file = file
    w.size = size
    w.lastRotate = now

    if oldFile != nil {
        if err := oldFile.Close(); err != nil {
            reportLogError("close rotated file", err)
        }
    }

    w.cleanupLocked()
    return nil
}

func (w *RotatingWriter) useDateFileName() bool {
    return w.cfg.RotateByDate && w.cfg.MaxSize == 0
}

func (w *RotatingWriter) logLocation() *time.Location {
    if w.cfg.Location != nil {
        return w.cfg.Location
    }
    return time.Local
}

func (w *RotatingWriter) openFileLocked(now time.Time, byDate bool) (*os.File, int64, error) {
    if byDate {
        return w.openDateFileLocked(now)
    }
    return w.openSizeFileLocked(now)
}

func (w *RotatingWriter) openDateFileLocked(now time.Time) (*os.File, int64, error) {
    path := fmt.Sprintf("%s.%s.log", w.cfg.FilePath, now.In(w.logLocation()).Format(w.cfg.DateLayout))
    file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return nil, 0, fmt.Errorf("zlog: open log file failed: %w", err)
    }
    size, err := fileSize(file)
    if err != nil {
        _ = file.Close()
        return nil, 0, err
    }
    return file, size, nil
}

func (w *RotatingWriter) openSizeFileLocked(now time.Time) (*os.File, int64, error) {
    prefix := fmt.Sprintf("%s.%s", w.cfg.FilePath, now.In(w.logLocation()).Format(w.cfg.TimeLayout))
    if w.sizePrefix != prefix {
        w.sizePrefix = prefix
        w.nextSizeSeq = w.scanNextSizeSeq(prefix)
    }

    for {
        seq := w.nextSizeSeq
        if seq < 0 || seq == int(^uint(0)>>1) {
            return nil, 0, errors.New("zlog: log file sequence overflow")
        }

        path := sizeFileName(prefix, seq)
        file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
        if err == nil {
            w.nextSizeSeq = seq + 1
            size, statErr := fileSize(file)
            if statErr != nil {
                _ = file.Close()
                return nil, 0, statErr
            }
            return file, size, nil
        }
        if os.IsExist(err) {
            w.nextSizeSeq = seq + 1
            continue
        }
        return nil, 0, fmt.Errorf("zlog: open log file failed: %w", err)
    }
}

func sizeFileName(prefix string, seq int) string {
    if seq == 0 {
        return prefix + ".log"
    }
    return fmt.Sprintf("%s.%d.log", prefix, seq)
}

// scanNextSizeSeq 扫描同一时间戳前缀的现存文件，返回下一个可分配序号。
//
// 基础文件按序号 0 处理；已存在的最大序号 + 1 作为下一个序号，
// 因此清理产生的低序号空位不会被复用。
func (w *RotatingWriter) scanNextSizeSeq(prefix string) int {
    dirName := filepath.Dir(prefix)
    baseName := filepath.Base(prefix)
    entries, err := os.ReadDir(dirName)
    if err != nil {
        return 0
    }

    maxSeq := -1
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        name := entry.Name()
        if name == baseName+".log" {
            if maxSeq < 0 {
                maxSeq = 0
            }
            continue
        }
        if !strings.HasPrefix(name, baseName+".") || !strings.HasSuffix(name, ".log") {
            continue
        }
        seqStr := strings.TrimSuffix(strings.TrimPrefix(name, baseName+"."), ".log")
        seq, err := strconv.Atoi(seqStr)
        if err != nil || seq <= 0 || strconv.Itoa(seq) != seqStr {
            continue
        }
        if seq > maxSeq {
            maxSeq = seq
        }
    }
    return maxSeq + 1
}

func fileSize(file *os.File) (int64, error) {
    fileInfo, err := file.Stat()
    if err != nil {
        return 0, fmt.Errorf("zlog: stat log file failed: %w", err)
    }
    return fileInfo.Size(), nil
}

// cleanupLocked 只清理当前实例且可解析的历史文件，并保留当前活动文件。
func (w *RotatingWriter) cleanupLocked() {
    activeName := ""
    if w.file != nil {
        activeName = filepath.Base(w.file.Name())
    }

    dateLayouts := make([]string, 0, 1)
    timeLayouts := make([]string, 0, 2)
    if w.cfg.RotateByDate {
        dateLayouts = append(dateLayouts, w.cfg.DateLayout)
    }
    if !w.cfg.RotateByDate || w.cfg.MaxSize > 0 {
        timeLayouts = append(timeLayouts, w.cfg.TimeLayout, legacyTimestampLayout)
    }
    cleanupLogDir(
        filepath.Dir(w.cfg.FilePath),
        filepath.Base(w.cfg.FilePath),
        w.cfg.KeepCnt,
        activeName,
        dateLayouts,
        timeLayouts,
        w.logLocation(),
    )
}

// cleanupLogDir 按文件归属、解析时间和数值序号清理历史文件。
//
// activeName 非空时表示在线清理，保留 max(KeepCnt-1, 0) 个历史文件；
// activeName 为空时表示离线清理，保留 max(KeepCnt, 1) 个文件。
func cleanupLogDir(dirName, baseName string, keepCnt int, activeName string, dateLayouts, timestampLayouts []string, location *time.Location) {
    entries, err := os.ReadDir(dirName)
    if err != nil {
        reportLogError("read log dir", err)
        return
    }

    metas := make([]logFileMeta, 0, len(entries))
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        name := entry.Name()
        if name == activeName {
            continue
        }
        meta, ok := parseLogFile(baseName, name, dateLayouts, timestampLayouts, location)
        if !ok {
            continue
        }
        metas = append(metas, meta)
    }

    sort.Slice(metas, func(i, j int) bool {
        if !metas[i].generated.Equal(metas[j].generated) {
            return metas[i].generated.Before(metas[j].generated)
        }
        if metas[i].seq != metas[j].seq {
            return metas[i].seq < metas[j].seq
        }
        return metas[i].name < metas[j].name
    })

    keep := 0
    if activeName == "" {
        keep = keepCnt
        if keep < 1 {
            keep = 1
        }
    } else {
        keep = keepCnt - 1
        if keep < 0 {
            keep = 0
        }
    }
    if len(metas) <= keep {
        return
    }

    for _, meta := range metas[:len(metas)-keep] {
        if err := os.Remove(filepath.Join(dirName, meta.name)); err != nil {
            reportLogError("remove log file", err)
        }
    }
}

// parseLogFile 严格解析日志文件名，避免把相似前缀文件当成当前实例文件。
func parseLogFile(baseName, name string, dateLayouts, timestampLayouts []string, location *time.Location) (logFileMeta, bool) {
    prefix := baseName + "."
    if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
        return logFileMeta{}, false
    }

    payload := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log")
    if payload == "" {
        return logFileMeta{}, false
    }

    for _, layout := range dateLayouts {
        if layout == "" {
            continue
        }
        if generated, ok := parseTimeLayout(layout, payload, location); ok {
            return logFileMeta{name: name, generated: generated}, true
        }
    }

    for _, layout := range timestampLayouts {
        if layout == "" {
            continue
        }
        if generated, ok := parseTimeLayout(layout, payload, location); ok {
            return logFileMeta{name: name, generated: generated}, true
        }
    }

    dot := strings.LastIndex(payload, ".")
    if dot <= 0 || dot == len(payload)-1 {
        return logFileMeta{}, false
    }

    stem := payload[:dot]
    seqStr := payload[dot+1:]
    seq, err := strconv.Atoi(seqStr)
    if err != nil || seq <= 0 || strconv.Itoa(seq) != seqStr {
        return logFileMeta{}, false
    }

    for _, layout := range timestampLayouts {
        if layout == "" {
            continue
        }
        if generated, ok := parseTimeLayout(layout, stem, location); ok {
            return logFileMeta{name: name, generated: generated, seq: seq}, true
        }
    }
    return logFileMeta{}, false
}

func parseTimeLayout(layout, value string, location *time.Location) (time.Time, bool) {
    if location == nil {
        location = time.Local
    }
    parsed, err := time.ParseInLocation(layout, value, location)
    if err != nil || parsed.Format(layout) != value {
        return time.Time{}, false
    }
    return parsed, true
}

func calendarDaysBetween(from, to time.Time) int {
    fromYear, fromMonth, fromDay := from.Date()
    toYear, toMonth, toDay := to.Date()

    // 只比较日历日期，用 UTC 零点避免夏令时导致的 23/25 小时问题。
    fromDate := time.Date(fromYear, fromMonth, fromDay, 0, 0, 0, 0, time.UTC)
    toDate := time.Date(toYear, toMonth, toDay, 0, 0, 0, 0, time.UTC)
    return int(toDate.Sub(fromDate) / (24 * time.Hour))
}

// reportLogError 将内部错误写到 stderr。
// 这里不能调用 zlog/slog，否则轮转错误可能递归触发日志。
func reportLogError(operation string, err error) {
    fmt.Fprintf(os.Stderr, "zlog: %s failed: %v\n", operation, err)
}
