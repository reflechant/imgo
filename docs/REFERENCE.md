# ImGo Standard Library Reference

The `persistent` package is at the heart of ImGo's functional semantics. It provides the high-performance, immutable data structures that back ImGo's maps and slices.

## 1. Package `persistent`

### 1.1 `Map[K comparable, V any]`
A persistent Hash Array Mapped Trie (HAMT) based on structural sharing.

- **`NewMap[K comparable, V any]() Map[K, V]`**
  Initializes an empty map.
- **`(m Map) Set(k K, v V) Map[K, V]`**
  Returns a new map with the key `k` set to value `v`.
- **`(m Map) Delete(k K) Map[K, V]`**
  Returns a new map with the key `k` removed.
- **`(m Map) Update(k K, fn func(V) V) Map[K, V]`**
  Returns a new map by applying a transformation function to the value at key `k`.
- **`(m Map) SetIn(path... K, v V) Map`**
  Performs a deep update, automatically creating intermediate maps as needed.
- **`(m Map) UpdateIn(path... K, fn func(V) V) Map`**
  Performs a deep update by applying `fn` to the leaf value.
- **`(m Map) DeleteIn(path... K) Map`**
  Performs a deep removal of a key in a nested map.
- **`(m Map) Lookup(k K) (V, bool)`**
  Look up a value by key. Returns the zero value and `false` if the key or map is `nil`.
- **`(m Map) Get(k K) V`**
  Returns the value or the zero value. Chainable for deep reads.
- **`(m Map) Len() int`**
  Returns the number of elements in the map.
- **`(m Map) All() iter.Seq2[K, V]`**
  Returns a Go 1.23 iterator for use in `for...range` loops.

### 1.2 Deep Map Operations & Convenience
While most ImGo features are driven by the necessity of immutability, the **Deep Map API** (`SetIn`, `UpdateIn`, `DeleteIn`) is a deliberate convenience feature inspired by Clojure's `assoc-in`, `update-in`, and `dissoc-in`.

These operations allow you to transform nested data structures without the tedious "nil-checking" boilerplate required in standard Go. If any intermediate map in the path is missing or `nil`, ImGo handles it gracefully:
- **`SetIn`**: Creates intermediate maps automatically.
- **`UpdateIn`**: Passes the zero-value to the update function if the path is missing.
- **`DeleteIn`**: Does nothing if the path doesn't exist, safely returning the original structure.
- **Chained Indexing**: `m["a"]["b"]["c"]` desugars to safe deep reads that return zero-values instead of panicking on intermediate `nil` maps.

### 1.3 `List[T any]`
A persistent bit-partitioned vector trie supporting efficient access and updates.

- **`NewList[T any]() List[T]`**
  Initializes an empty list.
- **`(l List) Append(v T) List[T]`**
  Returns a new list with the value appended at the end.
- **`(l List) Get(i int) T`**
  Returns the value at index `i`.
- **`(l List) Lookup(i int) (T, bool)`**
  Look up a value by index. Returns `false` if out of bounds.
- **`(l List) Set(i int, v T) List[T]`**
  Returns a new list with the value at index `i` replaced.
- **`(l List) Len() int`**
  Returns the number of elements in the list.
- **`(l List) All() iter.Seq[T]`**
  Returns a Go 1.23 iterator for use in `for...range` loops.

## 2. Identifier Re-binding (Shadowing)
In ImGo, names are not "variables" in the mutable sense; they are labels for values in a data flow.

### 2.1 The Flow Pattern
When you write `x := x + 1`, you are not incrementing a memory location. You are creating a new binding `x` that is derived from the previous value of `x`.

### 2.2 SSA Mangling & Closure Safety
To maintain strict immutability at the machine level, the transpiler uses **Static Single Assignment (SSA) mangling**.
```go
// ImGo (.im)
x := 10
f := func() { fmt.Println(x) }
x := 20
f() // Prints 10
```
Is transpiled to:
```go
// Go (_imgo_gen.go)
x_1 := 10
f_2 := func() { fmt.Println(x_1) }
x_3 := 20
f_2() // Still refers to x_1
```
This ensures that closures are always "pure" and free from data races caused by later re-bindings in the same scope.

## 3. Global Keywords and Semantics
ImGo leverages Go's built-in functions with modified functional semantics:
- **`len(c)`**: Desugars to `persistent.Len(c)`.
- **`make(T, ...)`**: Supported as sugar for `NewMap` or `NewList`. Capacity hints are currently ignored as persistent collections grow tree-nodes instead of contiguous buffers.
- **`delete(m, k)`**: Supported for persistent maps. Lowers to `.Delete(k)`.
- **`append(s, ...)`**: Supported for persistent lists. Lowers to `.Append(v)`.
- **`cap(c)`, `clear(c)`, `copy(dst, src)`, `new(T)`**: **PROHIBITED**.
- **Pointers**: `*T`, `*p`, and `&x` are **PERMITTED** for type signatures and expressions. However, using pointers for in-place mutation is **PROHIBITED**.
- **Assignment**: `=` is **PROHIBITED**. Use `:=` for shadowing.
