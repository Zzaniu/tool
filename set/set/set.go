package set

import (
    "bytes"
    "fmt"
)

// Set 不保证并发安全
type Set[T comparable] struct {
    M map[T]struct{}
}

// Contains 是否包含元素
func (s *Set[T]) Contains(key T) bool {
    _, exists := s.M[key]
    return exists
}

// Add 添加元素
func (s *Set[T]) Add(key T) bool {
    if s.Contains(key) {
        return false
    }
    s.M[key] = struct{}{}
    return true
}

func (s *Set[T]) add(key T) {
    s.M[key] = struct{}{}
}

// Remove 删除元素
func (s *Set[T]) Remove(key T) {
    // 如果key不存在，为空操作
    delete(s.M, key)
}

// Len 长度
func (s *Set[T]) Len() int {
    return len(s.M)
}

// IsEmpty 是否为空
func (s *Set[T]) IsEmpty() bool {
    return s.Len() == 0
}

// Clear 清空
func (s *Set[T]) Clear() {
    s.M = make(map[T]struct{})
}

// Elements 所有元素
func (s *Set[T]) Elements() []T {
    ret := make([]T, 0, s.Len())
    for key := range s.M {
        ret = append(ret, key)
    }
    return ret
}

func (s *Set[T]) Iter(fn func(key T) error) error {
    for key := range s.M {
        if err := fn(key); err != nil {
            return err
        }
    }
    return nil
}

func (s *Set[T]) String() string {
    var buf bytes.Buffer
    buf.WriteString("Set{")
    flag := true
    for k := range s.M {
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
    if other == nil || other.M == nil {
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
    if other == nil || other.Len() == 0 {
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
        for v := range s.M {
            diffSet.add(v)
        }
    } else {
        for v := range s.M {
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
    for v := range s.M {
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

// NewFromSlice 从切片生成
func NewFromSlice[T comparable](slice []T) *Set[T] {
    ret := &Set[T]{M: make(map[T]struct{}, len(slice))}
    for index := range slice {
        ret.add(slice[index])
    }
    return ret
}

func NewSet[T comparable]() *Set[T] {
    return &Set[T]{M: make(map[T]struct{})}
}

func NewSetWithLength[T comparable](length int) *Set[T] {
    return &Set[T]{M: make(map[T]struct{}, length)}
}
