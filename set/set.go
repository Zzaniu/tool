package set

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
