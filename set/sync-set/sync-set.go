package sync_set

import (
    "bytes"
    "fmt"
    "sync"
)

// SyncSet 保证并发安全
type SyncSet[T comparable] struct {
    sync.RWMutex
    m map[T]struct{}
}

func (s *SyncSet[T]) Contains(key T) bool {
    s.RLock()
    defer s.RUnlock()
    return s.contains(key)
}

func (s *SyncSet[T]) contains(key T) bool {
    if s == nil {
        return false
    }
    _, exists := s.m[key]
    return exists
}

func (s *SyncSet[T]) Add(key T) bool {
    if s.Contains(key) {
        return false
    }
    s.Lock()
    defer s.Unlock()
    if _, exists := s.m[key]; exists {
        return false
    }
    s.m[key] = struct{}{}
    return true
}

func (s *SyncSet[T]) Remove(key T) {
    if !s.Contains(key) {
        return
    }
    s.Lock()
    defer s.Unlock()
    // 如果key不存在，为空操作，所以这里不再判断也没关系
    delete(s.m, key)
}

func (s *SyncSet[T]) Len() int {
    s.RLock()
    defer s.RUnlock()
    if s == nil {
        return 0
    }
    return len(s.m)
}

func (s *SyncSet[T]) IsEmpty() bool {
    s.RLock()
    defer s.RUnlock()
    return s.isEmpty()
}

func (s *SyncSet[T]) isEmpty() bool {
    if s == nil {
        return true
    }
    return len(s.m) == 0
}

func (s *SyncSet[T]) Clear() {
    s.Lock()
    defer s.Unlock()
    if s.isEmpty() {
        return
    }
    s.m = make(map[T]struct{})
}

func (s *SyncSet[T]) Elements() []T {
    s.RLock()
    defer s.RUnlock()
    if s.isEmpty() {
        return []T{}
    }
    snapshot := make([]T, 0, len(s.m))
    for key := range s.m {
        snapshot = append(snapshot, key)
    }
    return snapshot
}

func (s *SyncSet[T]) Iter(fn func(key T) error) error {
    s.RLock()
    defer s.RUnlock()
    for key := range s.m {
        if err := fn(key); err != nil {
            return err
        }
    }
    return nil
}

func (s *SyncSet[T]) String() string {
    s.RLock()
    defer s.RUnlock()
    if s == nil {
        return "nil"
    }
    var buf bytes.Buffer
    buf.WriteString("SyncSet{")
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

func (s *SyncSet[T]) rawContainer() map[T]struct{} {
    return s.m
}

// Same 是否相同, 指所包含的元素是否都一致.
func (s *SyncSet[T]) Same(otherSet *SyncSet[T]) bool {
    s.RLock()
    defer s.RUnlock()
    otherSet.RLock()
    defer otherSet.RUnlock()

    if s == nil || otherSet == nil {
        return false
    }
    otherLength := len(otherSet.m)
    if otherLength == 0 || len(s.m) != otherLength {
        return false
    }
    for key := range s.m {
        if _, exists := otherSet.m[key]; !exists {
            return false
        }
    }
    return true
}

// Intersect 交集.
func (s *SyncSet[T]) Intersect(otherSet *SyncSet[T]) *SyncSet[T] {
    s.RLock()
    defer s.RUnlock()
    otherSet.RLock()
    defer otherSet.RUnlock()

    if s == nil || len(s.m) == 0 || otherSet == nil || len(otherSet.m) == 0 {
        return NewSyncSet[T]()
    }
    intersectSet := NewSyncSet[T]()
    if len(s.m) > len(otherSet.m) {
        for key := range otherSet.m {
            if s.contains(key) {
                intersectSet.m[key] = struct{}{}
            }
        }
    } else {
        for key := range s.m {
            if otherSet.contains(key) {
                intersectSet.m[key] = struct{}{}
            }
        }
    }
    return intersectSet
}

// Difference 差集.
func (s *SyncSet[T]) Difference(otherSet *SyncSet[T]) *SyncSet[T] {
    s.RLock()
    defer s.RUnlock()
    otherSet.RLock()
    defer otherSet.RUnlock()

    diffSet := NewSyncSet[T]()
    if s == nil || len(s.m) == 0 {
        return diffSet
    }
    if otherSet == nil || len(otherSet.m) == 0 {
        for key := range s.m {
            diffSet.m[key] = struct{}{}
        }
    } else {
        for key := range s.m {
            if !otherSet.contains(key) {
                diffSet.m[key] = struct{}{}
            }
        }
    }
    return diffSet
}

// Union 并集
func (s *SyncSet[T]) Union(otherSet *SyncSet[T]) *SyncSet[T] {
    s.RLock()
    defer s.RUnlock()
    otherSet.RLock()
    defer otherSet.RUnlock()

    union := NewSyncSet[T]()
    if s != nil && len(s.m) > 0 {
        for key := range s.m {
            union.m[key] = struct{}{}
        }
    }

    if otherSet != nil && len(otherSet.m) > 0 {
        for key := range otherSet.m {
            union.m[key] = struct{}{}
        }
    }
    return union
}

// NewFromSlice 从切片生成
func NewFromSlice[T comparable](slice []T) *SyncSet[T] {
    ret := &SyncSet[T]{m: make(map[T]struct{})}
    for index := range slice {
        ret.Add(slice[index])
    }
    return ret
}

func NewSyncSet[T comparable]() *SyncSet[T] {
    return &SyncSet[T]{m: make(map[T]struct{})}
}
