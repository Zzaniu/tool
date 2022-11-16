package sync_str_set

import (
    "bytes"
    "fmt"
    "github.com/Zzaniu/tool/set"
    "sync"
)

// syncStrSet 保证并发安全
type syncStrSet struct {
    sync.RWMutex
    m map[string]struct{}
}

func (s *syncStrSet) Contains(key string) bool {
    s.RLock()
    defer s.RUnlock()
    return s.contains(key)
}

func (s *syncStrSet) contains(key string) bool {
    if s == nil {
        return false
    }
    _, exists := s.m[key]
    return exists
}

func (s *syncStrSet) Add(key string) bool {
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

func (s *syncStrSet) Remove(key string) {
    if !s.Contains(key) {
        return
    }
    s.Lock()
    defer s.Unlock()
    // 如果key不存在，为空操作，所以这里不再判断也没关系
    delete(s.m, key)
}

func (s *syncStrSet) Len() int {
    s.RLock()
    defer s.RUnlock()
    if s == nil {
        return 0
    }
    return len(s.m)
}

func (s *syncStrSet) IsEmpty() bool {
    s.RLock()
    defer s.RUnlock()
    return s.isEmpty()
}

func (s *syncStrSet) isEmpty() bool {
    if s == nil {
        return true
    }
    return len(s.m) == 0
}

func (s *syncStrSet) Clear() {
    s.Lock()
    defer s.Unlock()
    if s.isEmpty() {
        return
    }
    s.m = make(map[string]struct{})
}

func (s *syncStrSet) Elements() []string {
    s.RLock()
    defer s.RUnlock()
    if s.isEmpty() {
        return []string{}
    }
    snapshot := make([]string, 0, len(s.m))
    for key := range s.m {
        snapshot = append(snapshot, key)
    }
    return snapshot
}

func (s *syncStrSet) String() string {
    s.RLock()
    defer s.RUnlock()
    if s == nil {
        return "nil"
    }
    var buf bytes.Buffer
    buf.WriteString("SyncStrSet{")
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

func (s *syncStrSet) rawContainer() map[string]struct{} {
    return s.m
}

// Same 是否相同, 指所包含的元素是否都一致.
func (s *syncStrSet) Same(other set.Set) bool {
    otherSet, ok := other.(*syncStrSet)
    if !ok {
        panic("should be *syncStrSet")
    }
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
func (s *syncStrSet) Intersect(other set.Set) set.Set {
    otherSet, ok := other.(*syncStrSet)
    if !ok {
        panic("should be *syncStrSet")
    }
    s.RLock()
    defer s.RUnlock()
    otherSet.RLock()
    defer otherSet.RUnlock()

    if s == nil || len(s.m) == 0 || otherSet == nil || len(otherSet.m) == 0 {
        return NewSyncStrSet()
    }
    intersectSet := NewSyncStrSet()
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
func (s *syncStrSet) Difference(other set.Set) set.Set {
    otherSet, ok := other.(*syncStrSet)
    if !ok {
        panic("should be *syncStrSet")
    }
    s.RLock()
    defer s.RUnlock()
    otherSet.RLock()
    defer otherSet.RUnlock()

    diffSet := NewSyncStrSet()
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

// Union 并集. 不保证 other 并发安全
func (s *syncStrSet) Union(other set.Set) set.Set {
    otherSet, ok := other.(*syncStrSet)
    if !ok {
        panic("should be *syncStrSet")
    }
    s.RLock()
    defer s.RUnlock()
    otherSet.RLock()
    defer otherSet.RUnlock()

    union := NewSyncStrSet()
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

// NewFromStrSlice 从切片生成
func NewFromStrSlice(strSlice []string) *syncStrSet {
    ret := &syncStrSet{m: make(map[string]struct{})}
    for index := range strSlice {
		ret.Add(strSlice[index])
    }
    return ret
}

func NewSyncStrSet() *syncStrSet {
    return &syncStrSet{m: make(map[string]struct{})}
}
