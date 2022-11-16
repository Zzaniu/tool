package trans

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

const (
	MaxUint63 = uint64(1 << 63)
	Salt      = "!DZbcw#@1RAh4JMrTxc6O@5Zq6fPCEjSWkjPX1^iK6amuDu0UkvMJXsd&N8Rfuvr"
)

// GenCode 生成一个指定位数的随机数
func GenCode(n int) string {
	var sn int32 = 10
	for i := 1; i < n; i++ {
		sn *= 10
	}
	if sn > 1000000 {
		sn = 100000
	}
	num := rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(sn)
	str := "%0" + fmt.Sprintf("%vv", n)
	return fmt.Sprintf(str, num) // 这里面的04v:是保留4位， "%06v"; 这里面前面的04v是和后面的1000相对应的
}

// GetLogInfo 获取调用此方法 所在的文件、方法名、行号
func GetLogInfo(skip int) string {
	pc, file, line, ok := runtime.Caller(skip)
	if ok {
		fn := runtime.FuncForPC(pc).Name()
		fns := strings.Split(fn, ".")
		fn = fns[len(fns)-1]

		return fmt.Sprintf("%v %v():%v", file, fn, line)
	}
	return ""
}

func MapString2Interface(m map[string]string) map[string]interface{} {
	ret := make(map[string]interface{}, len(m))
	for k, v := range m {
		ret[k] = interface{}(v)
	}
	return ret
}

// MustAtoi string to int
func MustAtoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}

// IpToInt64 将ip string 转成 负的int64 (最高位置1)
func IpToInt64(ip string) int64 {
	var ipInt uint64
	ips := strings.Split(ip, ".")
	ipInt = MaxUint63
	ipInt |= uint64(MustAtoi(ips[0]) << 24)
	ipInt |= uint64(MustAtoi(ips[1]) << 16)
	ipInt |= uint64(MustAtoi(ips[2]) << 8)
	ipInt |= uint64(MustAtoi(ips[3]))
	return int64(ipInt)
}

// Int64ToIP 将一个最高位为1的int64转成ip string
func Int64ToIP(ipInt64 int64) string {
	ipUint64 := uint64(ipInt64) & (MaxUint63 - 1)
	ipUint64Slice := make([]string, 4)
	ipUint64Slice[0] = strconv.FormatUint(ipUint64>>24, 10)
	ipUint64Slice[1] = strconv.FormatUint((ipUint64&(255<<16))>>16, 10)
	ipUint64Slice[2] = strconv.FormatUint((ipUint64&(255<<8))>>8, 10)
	ipUint64Slice[3] = strconv.FormatUint(ipUint64&255, 10)
	return strings.Join(ipUint64Slice, ".")
}

// EncryptMd5 md5加密
func EncryptMd5(str string) (res string) {
	m5 := md5.New()
	m5.Write([]byte(Salt))
	m5.Write(*(*[]byte)(unsafe.Pointer(&str))) // 这样不会拷贝
	hash := m5.Sum(nil)
	res = fmt.Sprintf("%x", hash) // 将[]byte转成16进制
	return
}
