# ImGo Language Specification (Version 0.1)

ImGo is a persistent functional language built for the Go runtime. It enforces deep immutability while maintaining a syntax familiar to Go developers.

## 1. Source File
An ImGo source file is a text file with the `.im` extension. It follows the same package and import structure as standard Go.

## 2. Purity Rules
ImGo enforces a "Functional Core" through three strict prohibitions:
- **No Mutation:** The assignment operator `=` and compound assignments (`+=`, `*=`, etc.) are prohibited.
- **No Pointers:** Pointer types (`*T`) and the address-of operator (`&x`) are prohibited.
- **No Increments:** The `++` and `--` operators are prohibited.

## 3. Variable Binding
### 3.1 Global Bindings (def)
Package-level variables use `var`. They follow Go's zero-value initialization rules (e.g., `0` for numbers, `""` for strings).
```go
var MaxUsers = 100 // OK
var Counter int    // OK: Default 0
```

### 3.2 Local Bindings (let)
The `var` keyword is prohibited inside functions. All local bindings must use the short variable declaration operator `:=`.
```go
func main() {
    x := 5 // OK
    var y = 10 // ❌ Error: Use := instead
}
```

### 3.3 Shadowing Semantics
Re-binding an identifier with `:=` in the same or nested scope is permitted. This is implemented via **SSA-style name mangling** in the generated Go code. This ensures that closures capture the specific version of a variable they were defined with.

## 4. Built-in Types
### 4.1 Persistent Collections
- **Maps:** `map[K]V` syntax is sugar for `persistent.Map[K, V]`.
- **Slices:** `[]T` syntax is sugar for `persistent.List[T]`.
- **Nil Safety:** A `nil` map or slice is a valid empty collection. Operations like `m[k]` or `m.Set(k, v)` on a `nil` collection will not panic.
- **Deep Reads:** Chained indexing (e.g., `m["a"]["b"]`) is safe. If any intermediate map is missing, the expression returns the zero value of the final type.
- **Deep Writes:** The `.SetIn(k1, k2, ..., v)` and `.UpdateIn(k1, k2, ..., fn)` methods provide infinite-depth, type-safe updates, automatically creating intermediate maps.
- **Index Access:** 
    - `v := m[k]` desugars to `persistent.Get(m, k)` (1 value).
    - `v, ok := m[k]` desugars to `persistent.Lookup(m, k)` (2 values).
- **Length:** `len(coll)` is sugar for `persistent.Len(coll)`.

## 5. Control Flow
ImGo supports standard Go control flow: `if`, `else`, `for`, `range`, `switch`, and `return`. Since mutation is prohibited, loops are primarily used for iterating over collections or implementing recursive-like logic through shadowing.
