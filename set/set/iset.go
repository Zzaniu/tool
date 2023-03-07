package set

import (
    "bytes"
    "fmt"
    iset "github.com/Zzaniu/tool/set"
)

var (
    _ iset.ISet[string] = &TSet[string]{}
)

// TSet 不保证并发安全
type TSet[T comparable] struct {
    M map[T]struct{}
}

// Contains 是否包含元素
func (s *TSet[T]) Contains(key T) bool {
    _, exists := s.M[key]
    return exists
}

// Add 添加元素
func (s *TSet[T]) Add(key T) bool {
    if s.Contains(key) {
        return false
    }
    s.M[key] = struct{}{}
    return true
}

func (s *TSet[T]) add(key T) {
    s.M[key] = struct{}{}
}

// Remove 删除元素
func (s *TSet[T]) Remove(key T) {
    // 如果key不存在，为空操作
    delete(s.M, key)
}

// Len 长度
func (s *TSet[T]) Len() int {
    return len(s.M)
}

// IsEmpty 是否为空
func (s *TSet[T]) IsEmpty() bool {
    return s.Len() == 0
}

// Clear 清空
func (s *TSet[T]) Clear() {
    s.M = make(map[T]struct{})
}

// Elements 所有元素
func (s *TSet[T]) Elements() []T {
    ret := make([]T, 0, s.Len())
    for key := range s.M {
        ret = append(ret, key)
    }
    return ret
}

func (s *TSet[T]) Iter(fn func(key T) error) error {
    for key := range s.M {
        if err := fn(key); err != nil {
            return err
        }
    }
    return nil
}

func (s *TSet[T]) String() string {
    var buf bytes.Buffer
    buf.WriteString("TSet{")
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
func (s *TSet[T]) Same(other iset.ISet[T]) bool {
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
func (s *TSet[T]) Intersect(other iset.ISet[T]) iset.ISet[T] {
    if other == nil || other.Len() == 0 {
        return newTSet[T]()
    }
    intersectSet := newTSet[T]()
    elements := other.Elements()
    for index := range elements {
        if s.Contains(elements[index]) {
            intersectSet.add(elements[index])
        }
    }
    return intersectSet
}

// Difference 差集
func (s *TSet[T]) Difference(other iset.ISet[T]) iset.ISet[T] {
    diffSet := newTSet[T]()
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
func (s *TSet[T]) Union(other iset.ISet[T]) iset.ISet[T] {
    union := newTSet[T]()
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

func newTSet[T comparable]() *TSet[T] {
    return &TSet[T]{M: make(map[T]struct{})}
}

// TNewFromSlice 从切片生成
func TNewFromSlice[T comparable](slice []T) iset.ISet[T] {
    ret := &TSet[T]{M: make(map[T]struct{}, len(slice))}
    for index := range slice {
        ret.add(slice[index])
    }
    return ret
}

func TNewSet[T comparable]() iset.ISet[T] {
    return newTSet[T]()
}
