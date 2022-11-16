package cpu

import (
    "github.com/shirou/gopsutil/v3/cpu"
    "sync/atomic"
    "time"
)

const (
    interval = time.Millisecond * 500
)

var (
    stats CPU
    usage uint32
)

type CPU interface {
    Usage() (u uint32, e error)
}

type psutilCPU struct {
    interval time.Duration
}

// Usage 获取500ms内的CPU总使用量
func (ps *psutilCPU) Usage() (u uint32, err error) {
    var percents []float64
    percents, err = cpu.Percent(ps.interval, false) // false 取所有的而不是单个核的
    if err == nil {
        u = uint32(percents[0] * 10)
    }
    return
}

func newPsutilCPU(duration time.Duration) CPU {
    return &psutilCPU{interval: duration}
}

func getCpuUsage() {
    var err error
    stats, err = newCgroupCPU()
    if err != nil {
        stats = newPsutilCPU(interval)
    }
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        for range ticker.C {
            // 获取最近500ms内cpu总使用量, 并刷到usage中去
            u, err := stats.Usage()
            // fmt.Println("u = ", u)
            if err == nil && u != 0 {
                // 将最近500ms内的CPU使用量刷到usage中去
                atomic.StoreUint32(&usage, u)
            }
        }
    }()
}

func init() {
    getCpuUsage()
}

type Stat struct {
    Usage uint32 // cpu use ratio.
}

func (s *Stat) ReadStat() {
    s.Usage = atomic.LoadUint32(&usage)
}
