/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/15 19:50
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
	"errors"
	"fmt"
	"github.com/Zzaniu/tool/dir"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

type (
	Format int

	fileInfo []fs.FileInfo

	FileManageOption struct {
		keepCnt        int           // 保持数量
		cutIntervalDay int           // 切割间隔天数
		cutSize        int64         // 切割大小
		cutInterval    time.Duration // 切割间隔
		fileNameFormat Format
	}
	FileManageOptions func(*FileManageOption)
)

const (
	Timestamp Format = iota + 1
	Date
)

func (f fileInfo) Len() int { return len(f) }

// Less '<' 从小到大排序
func (f fileInfo) Less(i, j int) bool { return f[i].Name() < f[j].Name() }

func (f fileInfo) Swap(i, j int) { f[i], f[j] = f[j], f[i] }

func WithKeepCnt(keepCnt int) FileManageOptions {
	return func(option *FileManageOption) {
		switch {
		case keepCnt < 0:
			keepCnt = 0
		case keepCnt > 0 && keepCnt < 3:
			keepCnt = 3
		default:
		}
		option.keepCnt = keepCnt
	}
}

func WithCutSize(cutSize int64) FileManageOptions {
	return func(option *FileManageOption) {
		switch {
		case cutSize < 0:
			cutSize = 0
		case cutSize > 0 && cutSize < 10:
			cutSize = M10
		default:
		}
		option.cutSize = cutSize
	}
}

func WithCutIntervalDay(cutIntervalDay int) FileManageOptions {
	return func(option *FileManageOption) {
		switch {
		case cutIntervalDay < 0:
			cutIntervalDay = 0
		default:
		}
		option.cutIntervalDay = cutIntervalDay
	}
}

func WithCutIntervalTime(cutInterval time.Duration) FileManageOptions {
	return func(option *FileManageOption) {
		switch {
		case cutInterval < 0:
			cutInterval = 0
		case cutInterval > 0 && cutInterval < Hour:
			cutInterval = Hour
		default:
		}
		option.cutInterval = cutInterval
	}
}

func createLogFile(logFilePath string, format Format) (writerFile *os.File, err error) {
	if len(logFilePath) == 0 {
		return nil, errors.New(fmt.Sprintf("无文件名, logFilePath = %v", logFilePath))
	}
	formatStr := "20060102150405"
	switch format {
	case Date:
		formatStr = "2006-01-02"
	}
	err = dir.MkdirAll(filepath.Dir(logFilePath), 0644)
	if err != nil {
		return
	}
	for i := 0; i < 3; i++ {
		writerFile, err = os.OpenFile(fmt.Sprintf("%v.%v.log", logFilePath, time.Now().Format(formatStr)), FileStandard, 0755)
		if err != nil {
			log.Printf("创建文件失败, err = %v\n", err)
			time.Sleep(time.Millisecond * 1)
			continue
		}
		return
	}
	return
}

func mustCreateLogFile(logFilePath string, format Format) *os.File {
	writerFile, err := createLogFile(logFilePath, format)
	if err != nil {
		log.Fatalf("create file %v failed: %v", logFilePath, err)
	}
	return writerFile
}

func Rename(filePath string) (err error) {
	for i := 0; i < 3; i++ {
		err = os.Rename(fmt.Sprintf("%v.log", filePath), fmt.Sprintf("%v.%v.log", filePath, time.Now().Format("20060102150405")))
		if err != nil {
			log.Printf("重命名失败, err = %v\n", err)
			time.Sleep(time.Millisecond * 1)
			continue
		}
		return
	}
	return
}

func CleanHisLog(filePath string, keepCnt int) {
	dirName := path.Dir(filePath)
	files, err := ioutil.ReadDir(dirName)
	if err != nil {
		log.Printf("读取文件夹失败, err = %v\n", err)
		return
	}

	for i := 0; i < len(files); i++ {
		if files[i].IsDir() {
			if i == 0 {
				files = files[1:]
			} else if i == len(files)-1 {
				files = files[:i]
			} else {
				files = append(files[:i], files[i+1:]...)
			}
		}
	}
	if len(files) < keepCnt {
		return
	}
	sort.Sort(fileInfo(files))
	files = files[:len(files)-keepCnt]
	for _, file := range files {
		err := os.Remove(path.Join(dirName, file.Name()))
		if err != nil {
			log.Printf("删除文件失败, err = %v", err)
		}
	}
}
