package str_set

import (
    "fmt"
    "testing"
)

func TestSet(t *testing.T) {
    x1 := []string{"1", "2", "4"}
    x2 := []string{"1", "2", "5"}
    s1 := NewFromStrSlice(x1)
    s2 := NewFromStrSlice(x2)

    fmt.Println("s1.Difference(s2).String() = ", s1.Difference(s2).String())
    fmt.Println("s1.Intersect(s2).String() = ", s1.Intersect(s2).String())
    fmt.Println("s1.Union(s2).String() = ", s1.Union(s2).String())
    fmt.Println("s2.Difference(s1).String() = ", s2.Difference(s1).String())
    fmt.Println("s2.Intersect(s1).String() = ", s2.Intersect(s1).String())
    fmt.Println("s2.Union(s1).String() = ", s2.Union(s1).String())
}

// go test .
// go test -v -run TestSet .
