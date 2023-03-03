package set

// Set
// Deprecated: 将丢弃, 请直接使用 set.Set 和 sync_set.SyncSet
type Set interface {
    Contains(string) bool
    Add(string) bool
    Remove(string)
    Len() int
    IsEmpty() bool
    Clear()
    Elements() []string
    String() string
    Same(Set) bool      // 是否相同, 指所包含的元素是否都一致
    Intersect(Set) Set  // 交集
    Difference(Set) Set // 差集
    Union(Set) Set      // 并集
}

type ISet[T comparable] interface {
    Contains(T) bool
    Add(T) bool
    Remove(T)
    Len() int
    IsEmpty() bool
    Clear()
    Elements() []T
    Iter(func(key T) error) error
    String() string
    Same(ISet[T]) bool          // 是否相同, 指所包含的元素是否都一致
    Intersect(ISet[T]) ISet[T]  // 交集
    Difference(ISet[T]) ISet[T] // 差集
    Union(ISet[T]) ISet[T]      // 并集
}
