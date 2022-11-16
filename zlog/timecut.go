/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/15 15:27
Desc   :

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
    "os"
    "sync"
    "time"
)

type (
    TimeFileManage struct {
        sync.Mutex
        filePath       string
        logWriteFile   *os.File
        prevCutTime    time.Time
        cutInterval    time.Duration
        keepCnt        int // 保留日志个数
        fileNameFormat Format
    }

    TimeOption func(*TimeFileManage)
)

const (
    Hour   = time.Hour
    Hour6  = 6 * Hour
    Hour12 = 2 * Hour6
    Hour24 = 2 * Hour12
)

// NewTimeFileManage 创建文件大小管理器, 默认20M切割
func NewTimeFileManage(filePath string, options ...FileManageOptions) *TimeFileManage {
    opt := FileManageOption{
        cutInterval:    Hour12,
        keepCnt:        10,
        fileNameFormat: Timestamp,
    }

    for _, o := range options {
        o(&opt)
    }

    timeFileManage := &TimeFileManage{
        filePath:       filePath,
        prevCutTime:    time.Now(),
        cutInterval:    opt.cutInterval,
        keepCnt:        opt.keepCnt,
        fileNameFormat: opt.fileNameFormat,
    }

    return timeFileManage
}

func (t *TimeFileManage) FilePath() string {
    return t.filePath
}

func (t *TimeFileManage) GetFile() *os.File {
    t.Lock()
    defer t.Unlock()
    if t.logWriteFile == nil {
        t.logWriteFile = mustCreateLogFile(t.filePath, t.fileNameFormat)
    }
    return t.logWriteFile
}

func (t *TimeFileManage) Close() error {
    if t.logWriteFile != nil {
        return t.logWriteFile.Close()
    }
    return nil
}

func (t *TimeFileManage) IsCut(f SetOutput) error {
    if t.cutInterval == 0 {
        return nil
    }
    now := time.Now()
    // TODO 这里要注意新创建的名字可能是一样的
    if now.Sub(t.prevCutTime) > t.cutInterval {
        t.Lock()
        defer t.Unlock()
        if now.Sub(t.prevCutTime) >= t.cutInterval {
            // _ = t.Close()
            // _ = Rename(t.filePath)

            writer, err := createLogFile(t.filePath, t.fileNameFormat)
            if err != nil {
                return err
            }

            _ = t.Close()
            t.logWriteFile = writer
            t.prevCutTime = now
            f(writer)
            CleanHisLog(t.filePath, t.keepCnt)
        }
    }
    return nil
}
