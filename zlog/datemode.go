/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/15 15:25
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
    DateFileManage struct {
        sync.Mutex
        filePath       string
        logWriteFile   *os.File
        prevCutTime    time.Time
        cutIntervalDay int
        keepCnt        int // 保留日志个数
        fileNameFormat Format
    }

    DateOption func(*DateFileManage)
)

// NewDateCutMode 创建文件日期管理器, 默认1天1切
func NewDateCutMode(filePath string, options ...FileManageOptions) *DateFileManage {
    opt := FileManageOption{
        cutIntervalDay: 1,
        keepCnt:        10,
        fileNameFormat: Date,
    }

    for _, o := range options {
        o(&opt)
    }

    dateFileManage := &DateFileManage{
        prevCutTime:    time.Now(),
        cutIntervalDay: opt.cutIntervalDay,
        filePath:       filePath,
        keepCnt:        opt.keepCnt,
        fileNameFormat: opt.fileNameFormat,
    }

    return dateFileManage
}

func (d *DateFileManage) FilePath() string {
    return d.filePath
}

func (d *DateFileManage) GetFile() *os.File {
    d.Lock()
    defer d.Unlock()
    if d.logWriteFile == nil {
        d.logWriteFile = mustCreateLogFile(d.filePath, d.fileNameFormat)
    }
    return d.logWriteFile
}

func (d *DateFileManage) Close() error {
    if d.logWriteFile != nil {
        return d.logWriteFile.Close()
    }
    return nil
}

func (d *DateFileManage) IsCut(f SetOutput) error {
    if d.cutIntervalDay == 0 {
        return nil
    }
    now := time.Now()
    year, month, day := now.Date()
    prevYear, prevMonth, prevDay := d.prevCutTime.Date()
    if day-prevDay >= d.cutIntervalDay || month != prevMonth || year != prevYear {
        d.Lock()
        defer d.Unlock()
        prevYear, prevMonth, prevDay := d.prevCutTime.Date()
        if day-prevDay >= d.cutIntervalDay || year != prevYear || month != prevMonth {
            // _ = d.Close()
            // _ = Rename(d.filePath)

            writer, err := createLogFile(d.filePath, d.fileNameFormat)
            if err != nil {
                return err
            }

            _ = d.Close()
            d.logWriteFile = writer
            d.prevCutTime = now
			f(writer)
			CleanHisLog(d.filePath, d.keepCnt)
		}
    }
    return nil
}
