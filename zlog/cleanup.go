package zlog

import (
    "path/filepath"
    "time"
)

// CleanHisLog 离线清理历史日志。
//
// 该接口没有活动文件参数，只适用于调用方确认没有活动 writer 的离线清理；
// 库内部轮转使用带活动文件名的 cleanupLogDir。
// 无时区文件名按 time.Local 解释；自定义时区请使用 CleanHisLogAt。
func CleanHisLog(filePath string, keepCnt int) {
    CleanHisLogAt(filePath, keepCnt, time.Local)
}

// CleanHisLogAt 按指定时区离线清理历史日志。
// location 为 nil 时使用 time.Local。
func CleanHisLogAt(filePath string, keepCnt int, location *time.Location) {
    CleanHisLogWithLayouts(filePath, keepCnt, defaultDateLayout, defaultTimestampLayout, location)
}

// CleanHisLogWithLayouts 按指定日期/时间戳布局和时区离线清理历史日志。
// 空布局使用默认布局；location 为 nil 时使用 time.Local。
func CleanHisLogWithLayouts(filePath string, keepCnt int, dateLayout, timeLayout string, location *time.Location) {
    if filePath == "" {
        return
    }
    if location == nil {
        location = time.Local
    }
    if dateLayout == "" {
        dateLayout = defaultDateLayout
    }
    if timeLayout == "" {
        timeLayout = defaultTimestampLayout
    }

    cleanupLogDir(
        filepath.Dir(filePath),
        filepath.Base(filePath),
        keepCnt,
        "",
        []string{dateLayout},
        []string{timeLayout, legacyTimestampLayout},
        location,
    )
}
