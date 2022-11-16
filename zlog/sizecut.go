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
    "golang.org/x/xerrors"
    "os"
    "sync"
    "time"
)

type (
    SizeFileManage struct {
        sync.Mutex
        filePath       string
        logWriteFile   *os.File
        cutSize        int64
        prevCutTime    time.Time
        keepCnt        int    // 保留日志个数
        fileNameFormat Format // 保留日志个数
    }

    SizeOption func(*SizeFileManage)
)

const (
    M10  = 1024 * 1024 * 10
    M20  = 2 * M10
    M50  = 5 * M10
    M100 = 2 * M50
)

// NewSizeCutMode 创建文件大小管理器, 默认20M切割
func NewSizeCutMode(filePath string, options ...FileManageOptions) *SizeFileManage {
    opt := FileManageOption{
        cutSize:        M50,
        keepCnt:        10,
        fileNameFormat: Timestamp,
    }

    for _, o := range options {
        o(&opt)
    }

    sizeFileManage := &SizeFileManage{
        cutSize:        opt.cutSize,
        filePath:       filePath,
        prevCutTime:    time.Now(),
        keepCnt:        opt.keepCnt,
        fileNameFormat: opt.fileNameFormat,
    }

    return sizeFileManage
}

func (s *SizeFileManage) FilePath() string {
    return s.filePath
}

func (s *SizeFileManage) GetFile() *os.File {
    s.Lock()
    defer s.Unlock()
    if s.logWriteFile == nil {
        s.logWriteFile = mustCreateLogFile(s.filePath, s.fileNameFormat)
    }
    return s.logWriteFile
}

func (s *SizeFileManage) Close() error {
    if s.logWriteFile != nil {
        return s.logWriteFile.Close()
    }
    return nil
}

func (s *SizeFileManage) IsCut(f SetOutput) error {
    if s.cutSize == 0 {
        return nil
    }
    fileInfo, err := s.logWriteFile.Stat()
    if err != nil {
        return xerrors.Errorf("获取文件%v信息失败, err = %w", s.logWriteFile.Name(), err)
    }
    now := time.Now()
    if fileInfo.Size() >= s.cutSize && now.Unix() != s.prevCutTime.Unix() {
        s.Lock()
        defer s.Unlock()
        fileInfo, err = s.logWriteFile.Stat()
        if err != nil {
            return xerrors.Errorf("获取文件信息失败, err = %w", err)
        }
        if fileInfo.Size() > s.cutSize {
            // _ = s.Close()
            // _ = Rename(s.filePath)

            writer, err := createLogFile(s.filePath, s.fileNameFormat)
            if err != nil {
                if writer != nil {
                    _ = writer.Close()
                }
                return xerrors.Errorf("创建文件失败, err = %w", err)
            }
            _ = s.Close()
            s.logWriteFile = writer
			s.prevCutTime = now
			f(writer)
			CleanHisLog(s.filePath, s.keepCnt)
		}
    }
    return nil
}
