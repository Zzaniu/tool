package str_set

import (
    "bytes"
    "fmt"
    "github.com/Zzaniu/tool/set"
)

var (
    _ set.Set = &strSet{}
)

// strSet 不保证并发安全
type strSet map[string]struct{}

// Contains 是否包含元素
func (s *strSet) Contains(key string) bool {
    _, exists := (*s)[key]
    return exists
}

// Add 添加元素
func (s *strSet) Add(key string) bool {
    if s.Contains(key) {
        return false
    }
    (*s)[key] = struct{}{}
    return true
}

func (s *strSet) add(key string) {
    (*s)[key] = struct{}{}
}

// Remove 删除元素
func (s *strSet) Remove(key string) {
    // 如果key不存在，为空操作
    delete(*s, key)
}

// Len 长度
func (s *strSet) Len() int {
    return len(*s)
}

// IsEmpty 是否为空
func (s *strSet) IsEmpty() bool {
    return len(*s) == 0
}

// Clear 清空
func (s *strSet) Clear() {
    *s = make(map[string]struct{})
}

// Elements 所有元素
func (s *strSet) Elements() []string {
    ret := make([]string, 0, s.Len())
    for key := range *s {
        ret = append(ret, key)
    }
    return ret
}

func (s *strSet) String() string {
    var buf bytes.Buffer
    buf.WriteString("StrSet{")
    flag := true
    for k := range *s {
        if flag {
            flag = false
        } else {
            buf.WriteString(" ")
        }
        buf.WriteString(fmt.Sprintf("%v", k))
    }
    buf.WriteString("}")
    return buf.String()
}

// Same 是否相同
func (s *strSet) Same(other set.Set) bool {
    if other == nil {
        return false
    }

    if s.Len() != other.Len() {
        return false
    }
    elements := other.Elements()
    for index := range elements {
        if !s.Contains(elements[index]) {
            return false
        }
    }
    return true
}

// Union 并集
func (s *strSet) Union(other set.Set) set.Set {
    union := NewStrSet()
    for v := range *s {
        union.add(v)
    }
    elements := other.Elements()
    for index := range elements {
        union.add(elements[index])
    }
    return union
}

// Difference 差集
func (s *strSet) Difference(other set.Set) set.Set {
    diffSet := NewStrSet()
    if other == nil || other.Len() == 0 {
        for v := range *s {
            diffSet.add(v)
        }
    } else {
        for v := range *s {
            if !other.Contains(v) {
                diffSet.add(v)
            }
        }
    }
    return diffSet
}

// Intersect 交集
func (s *strSet) Intersect(other set.Set) set.Set {
    if other == nil || other.Len() == 0 {
        return NewStrSet()
    }
    intersectSet := NewStrSet()
    elements := other.Elements()
    for index := range elements {
        if s.Contains(elements[index]) {
            intersectSet.add(elements[index])
        }
    }
    return intersectSet
}

// NewFromStrSlice 从切片生成
func NewFromStrSlice(strSlice []string) *strSet {
    ret := make(strSet)
    for index := range strSlice {
		ret.add(strSlice[index])
    }
    return &ret
}

func NewStrSet() *strSet {
    ret := make(strSet)
    return &ret
}
