/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/3 11:52
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

package timex

import "time"

// Use the long enough past time as start time, in case timex.Now() - lastTime equals 0.
var initTime = time.Now().AddDate(-1, -1, -1)

// Now returns a relative time duration since initTime, which is not important.
// The caller only needs to care about the relative value.
func Now() time.Duration {
    return time.Since(initTime)
}

// Since returns a diff since given d.
func Since(d time.Duration) time.Duration {
    return time.Since(initTime) - d
}

// Time returns current time, the same as time.Now().
func Time() time.Time {
    return initTime.Add(Now())
}
