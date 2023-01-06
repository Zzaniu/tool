package set

import (
    "bytes"
    "fmt"
    iset "github.com/Zzaniu/tool/set"
)

var (
    _ iset.ISet[string] = &set[string]{}
)

// set 不保证并发安全
type set[T comparable] struct {
    m map[T]struct{}
}

// Contains 是否包含元素
func (s *set[T]) Contains(key T) bool {
    _, exists := s.m[key]
    return exists
}

// Add 添加元素
func (s *set[T]) Add(key T) bool {
    if s.Contains(key) {
        return false
    }
    s.m[key] = struct{}{}
    return true
}

func (s *set[T]) add(key T) {
    s.m[key] = struct{}{}
}

// Remove 删除元素
func (s *set[T]) Remove(key T) {
    // 如果key不存在，为空操作
    delete(s.m, key)
}

// Len 长度
func (s *set[T]) Len() int {
    return len(s.m)
}

// IsEmpty 是否为空
func (s *set[T]) IsEmpty() bool {
    return s.Len() == 0
}

// Clear 清空
func (s *set[T]) Clear() {
    s.m = make(map[T]struct{})
}

// Elements 所有元素
func (s *set[T]) Elements() []T {
    ret := make([]T, 0, s.Len())
    for key := range s.m {
        ret = append(ret, key)
    }
    return ret
}

func (s *set[T]) String() string {
    var buf bytes.Buffer
    buf.WriteString("set{")
    flag := true
    for k := range s.m {
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
func (s *set[T]) Same(other iset.ISet[T]) bool {
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

// Intersect 交集
func (s *set[T]) Intersect(other iset.ISet[T]) iset.ISet[T] {
    if other == nil || other.Len() == 0 {
        return newISet[T]()
    }
    intersectSet := newISet[T]()
    elements := other.Elements()
    for index := range elements {
        if s.Contains(elements[index]) {
            intersectSet.add(elements[index])
        }
    }
    return intersectSet
}

// Difference 差集
func (s *set[T]) Difference(other iset.ISet[T]) iset.ISet[T] {
    diffSet := newISet[T]()
    if other == nil || other.Len() == 0 {
        for v := range s.m {
            diffSet.add(v)
        }
    } else {
        for v := range s.m {
            if !other.Contains(v) {
                diffSet.add(v)
            }
        }
    }
    return diffSet
}

// Union 并集
func (s *set[T]) Union(other iset.ISet[T]) iset.ISet[T] {
    union := newISet[T]()
    for v := range s.m {
        union.add(v)
    }
    if other == nil {
        return union
    }
    elements := other.Elements()
    for index := range elements {
        union.add(elements[index])
    }
    return union
}

func newISet[T comparable]() *set[T] {
    return &set[T]{m: make(map[T]struct{})}
}

// TNewFromSlice 从切片生成
func TNewFromSlice[T comparable](slice []T) iset.ISet[T] {
    ret := &set[T]{m: make(map[T]struct{}, len(slice))}
    for index := range slice {
        ret.add(slice[index])
    }
    return ret
}

func TNewSet[T comparable]() iset.ISet[T] {
    return newISet[T]()
}
