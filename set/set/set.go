package set

import (
    "bytes"
    "fmt"
)

// Set 不保证并发安全
type Set[T comparable] struct {
    m map[T]struct{}
}

// Contains 是否包含元素
func (s *Set[T]) Contains(key T) bool {
    _, exists := s.m[key]
    return exists
}

// Add 添加元素
func (s *Set[T]) Add(key T) bool {
    if s.Contains(key) {
        return false
    }
    s.m[key] = struct{}{}
    return true
}

func (s *Set[T]) add(key T) {
    s.m[key] = struct{}{}
}

// Remove 删除元素
func (s *Set[T]) Remove(key T) {
    // 如果key不存在，为空操作
    delete(s.m, key)
}

// Len 长度
func (s *Set[T]) Len() int {
    return len(s.m)
}

// IsEmpty 是否为空
func (s *Set[T]) IsEmpty() bool {
    return s.Len() == 0
}

// Clear 清空
func (s *Set[T]) Clear() {
    s.m = make(map[T]struct{})
}

// Elements 所有元素
func (s *Set[T]) Elements() []T {
    ret := make([]T, 0, s.Len())
    for key := range s.m {
        ret = append(ret, key)
    }
    return ret
}

func (s *Set[T]) String() string {
    var buf bytes.Buffer
    buf.WriteString("StrSet{")
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
func (s *Set[T]) Same(other *Set[T]) bool {
    if other.m == nil {
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
func (s *Set[T]) Intersect(other *Set[T]) *Set[T] {
    if other.m == nil || other.Len() == 0 {
        return NewSet[T]()
    }
    intersectSet := NewSet[T]()
    elements := other.Elements()
    for index := range elements {
        if s.Contains(elements[index]) {
            intersectSet.add(elements[index])
        }
    }
    return intersectSet
}

// Difference 差集
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
    diffSet := NewSet[T]()
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
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
    union := NewSet[T]()
    for v := range s.m {
        union.add(v)
    }
    elements := other.Elements()
    for index := range elements {
        union.add(elements[index])
    }
    return union
}

// NewFromStrSlice 从切片生成
func NewFromStrSlice[T comparable](slice []T) *Set[T] {
    ret := &Set[T]{m: make(map[T]struct{}, len(slice))}
    for index := range slice {
        ret.add(slice[index])
    }
    return ret
}

func NewSet[T comparable]() *Set[T] {
    return &Set[T]{m: make(map[T]struct{})}
}
