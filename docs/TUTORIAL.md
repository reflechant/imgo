# Learn ImGo in Y Minutes

ImGo is "Go with Clojure-like immutability". It looks like Go, runs on Go, but treats data as values that never change in-place. This guide focuses on the key differences between ImGo and standard Go.

## 1. Variable Binding & Shadowing

In ImGo, the `var` keyword is prohibited inside functions. All local bindings must use the short variable declaration operator `:=`. Because data is immutable, you don't "update" a variable; you shadow it to represent a new state in time.

```go
func main() {
    x := 10
    // x = 20 is PROHIBITED
    
    // Shadowing creates a new binding
    x := x + 5 
    
    // This is safe for closures!
    captured := func() { fmt.Println(x) } // captures x=15
    
    x := 100
    captured() // Prints 15
    fmt.Println(x) // Prints 100
}
```

## 2. Persistent Maps

The standard `map[K]V` syntax is automatically transpiled to a high-performance, persistent Hash Array Mapped Trie (HAMT).

```go
m := map[string]int{"a": 1, "b": 2}

// m.Set returns a NEW map. 'm' remains unchanged.
m1 := m.Set("c", 3)

// Standard indexing works as expected:
val := m1["a"]       // Desugars to m1.Get("a")
val, ok := m1["c"]   // Desugars to m1.Lookup("c")
```

## 3. Persistent Lists

The standard `[]T` (slice) syntax is transpiled to a persistent Vector Trie.

```go
l := []int{10, 20}

// The 'append' builtin is prohibited. Use .Append()
l1 := l.Append(30) // Returns new list [10, 20, 30]

fmt.Println(l1[2]) // Prints 30
```

## 4. Deep Convenience (Clojure-style)

ImGo provides powerful tools for working with nested structures without the "nil-check hell" of standard Go.

```go
db := map[string]map[string]int{
    "users": map[string]int{"id_1": 30},
}

// 1. Safe Deep Read: Returns zero-value if any part of the path is missing.
age := db["users"]["id_1"] // 30
missing := db["unknown"]["path"] // 0 (Does not panic!)

// 2. Deep Write (SetIn): Creates intermediate maps automatically.
db1 := db.SetIn("users", "id_2", 25)

// 3. Deep Update (UpdateIn): Transforms a value deep inside.
db2 := db1.UpdateIn("users", "id_1", func(old int) int {
    return old + 1
})
```

## 5. Nil Punning

`nil` is a valid empty collection. Operations on `nil` collections are safe and return a new, non-nil persistent collection.

```go

// If m is nil, .Set still works and returns a new map with 1 element.
m1 := m.Set("recovered", 42)

```

## 6. Builtin Deviations

- **`make`**: Returns empty collections. Capacity hints are ignored. For lists, `make([]T, 10)` returns a list of length **0**, not 10.
- **`len`**: Works normally but is desugared to `persistent.Len()`.
- **`range`**: Works normally but is desugared to `m.All()` or `l.All()`.
- **Prohibited**: `append`, `cap`, `clear`, `close`, `copy`, `delete`, `new`.

## 7. The Philosophy

ImGo shifts the mental model from **mutable state** to **data flow**. Instead of modifying objects in place, you pass values through transformations, binding each result to a name (or the same name) as you go. This eliminates whole classes of concurrency bugs and makes code significantly easier to reason about.
