# ImGo Standard Library Reference

The `persistent` package is at the heart of ImGo's functional semantics. It provides the high-performance, immutable data structures that back ImGo's maps and slices.

## 1. Package `persistent`

### 1.1 `Map[K comparable, V any]`
A persistent Hash Array Mapped Trie (HAMT) based on Bagwell's "Ideal Hash Trees."

- **`NewMap[K comparable, V any]() Map[K, V]`**
  Initializes an empty map.
- **`Assoc(m, k, v) Map[K, V]`**
  Functional entry point for `.Set()`. Handles `nil` maps gracefully.
- **`Update(m, k, fn) Map[K, V]`**
  Functional entry point for applying a transformation function `func(V) V` to a key.
- **`SetIn(k1, k2, ..., v) Map`**
  (Transpiler Sugar) Performs a deep update, creating intermediate maps as needed. Infinite depth.
- **`UpdateIn(k1, k2, ..., fn) Map`**
  (Transpiler Sugar) Performs a deep update by applying `fn` to the leaf value. Infinite depth.
- **`Lookup(m, k) (V, bool)`**
  Look up a value by key. Returns the zero value and `false` if the key or map is `nil`.
- **`Get(m, k) V`**
  Returns the value or the zero value. Chainable for deep reads.
- **`Len() int`**
  Returns the number of elements in the map.
- **`All() iter.Seq2[K, V]`**
  Returns a Go 1.23 iterator for use in `for...range` loops.

### 1.2 `List[T any]`
A bit-partitioned vector trie supporting O(log32 n) access and updates.

- **`NewList[T any]() List[T]`**
  Initializes an empty list.
- **`Append(v T) List[T]`**
  Returns a new list with the value appended at the end.
- **`Get(i int) T`**
  Returns the value at index `i`. Panics if index is out of bounds.
- **`Len() int`**
  Returns the number of elements in the list.
- **`Slice(low, high int) List[T]`**
  Returns a persistent sub-slice in constant time.
- **`All() iter.Seq[T]`**
  Returns a Go 1.23 iterator for use in `for...range` loops.

## 2. Global Keywords
ImGo leverages Go's keywords with modified semantics:
- **`len(c)`**: Invokes `c.Len()` if `c` is a persistent collection.
- **`make()`**: Not supported for maps or slices. Use literals or `NewX()` functions instead.
- **`copy()`**: Not supported. Use `.Slice()` or assignment for structural sharing.
