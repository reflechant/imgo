# ImGo Language Specification (Version 0.1)

ImGo is a persistent functional language built for the Go runtime. It enforces deep immutability while maintaining a syntax familiar to Go developers.

## 1. Source File
An ImGo source file is a text file with the `.im` extension. It follows the same package and import structure as standard Go.

## 2. Purity Rules
ImGo enforces a "Functional Core" through strict prohibitions on features that imply or allow shared mutable state.

### 2.1 Prohibited Operators
The following operators are prohibited:
- **Assignment:** `=`
- **Compound Assignment:** `+=`, `-=`, `*=`, `/=`, `%=`, `&=`, `|=`, `^=`, `<<=`, `>>=`, `&^=`
- **Increment/Decrement:** `++`, `--`

### 2.2 Pointers and References
To ensure physical immutability, all pointer-related operations are forbidden:
- **Pointer Types:** `*T`
- **Pointer Dereference:** `*p`
- **Address-of Operator:** `&x`

### 2.3 Builtin Functions
- **`delete(m, k)`**: Prohibited. Use the `.Delete(k)` or `.DeleteIn(path...)` methods on persistent maps instead.

## 3. Variable Binding and Data Flow
### 3.1 Global Bindings
Package-level variables use the `var` keyword. They follow Go's zero-value initialization rules.
```go
var MaxUsers = 100 // OK
var Counter int    // OK: Default 0
```

### 3.2 Local Bindings
The `var` keyword is **prohibited** inside functions and blocks. All local bindings must use the short variable declaration operator `:=`.
```go
func main() {
    x := 5 // OK
    var y = 10 // ❌ Error: Prohibited inside blocks
}
```

### 3.3 Identifier Re-binding
Deriving new data from existing data using the same identifier is permitted through `:=`. This is desugared via **SSA-style name mangling** in the generated Go code (e.g., `x` -> `x_1`, `x_2`). This mechanism ensures that the language remains lexically scoped and closure-safe, as each binding is physically unique in the generated output.

## 4. Built-in Types
### 4.1 Persistent Collections
- **Maps:** `map[K]V` is desugared to `persistent.Map[K, V]`.
- **Lists:** `[]T` is desugared to `persistent.List[T]`.
- **Nil Punning:** `nil` is a valid empty collection for both maps and lists. Operations on `nil` collections do not panic and return new, populated collections where applicable.

### 4.2 Collection Operations
- **Single-value Access:** `v := m[k]` or `v := l[i]` desugars to a `Get` call that returns the zero-value if the key/index is missing or the collection is `nil`.
- **Context-aware Access:** `v, ok := m[k]` or `v, ok := l[i]` desugars to a `Lookup` call.
- **Deep Access:** Chained indexing (e.g., `m["a"]["b"]`) is safe and handles missing intermediate maps by returning zero-values.
- **Deep Updates:** The methods `.SetIn(path..., value)`, `.UpdateIn(path..., fn)`, and `.DeleteIn(path...)` provide recursive updates for nested maps.
- **Length:** `len(coll)` desugars to `persistent.Len(coll)`.

## 5. Control Flow
ImGo supports standard Go control flow: `if`, `else`, `for`, `range`, `switch`, `defer`, and `return`. Loop-based data transformations should be implemented using recursion-like patterns through identifier re-binding.
