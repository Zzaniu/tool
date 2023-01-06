/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/11/16 14:23
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

package set

import (
    "fmt"
    "testing"
)

func TestSet(t *testing.T) {
    x1 := []string{"1", "2", "4"}
    x2 := []string{"1", "2", "5"}
    s1 := NewFromSlice[string](x1)
    s2 := NewFromSlice[string](x2)

    fmt.Println("s1.Same(s2) = ", s1.Same(s2))
    fmt.Println("s1.Difference(s2).String() = ", s1.Difference(s2).String())
    fmt.Println("s1.Intersect(s2).String() = ", s1.Intersect(s2).String())
    fmt.Println("s1.Union(s2).String() = ", s1.Union(s2).String())
    fmt.Println("s2.Difference(s1).String() = ", s2.Difference(s1).String())
    fmt.Println("s2.Intersect(s1).String() = ", s2.Intersect(s1).String())
    fmt.Println("s2.Union(s1).String() = ", s2.Union(s1).String())
}

// go test .
// go test -v -run TestSet .
