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

### 2.3 Prohibited Builtin Functions
The following Go builtin functions are prohibited because they imply in-place mutation or handle pointers:
- **`append(s, ...)`**: Prohibited. Use `s.Append(v)` on persistent lists instead.
- **`cap(c)`**: Prohibited. Persistent collections do not have a separate capacity concept; use `len(c)`.
- **`clear(c)`**: Prohibited. Performs in-place mutation of maps and slices.
- **`close(c)`**: Prohibited. Mutates channel state.
- **`copy(dst, src)`**: Prohibited. Performs in-place mutation of the destination.
- **`delete(m, k)`**: Prohibited. Use the `.Delete(k)` or `.DeleteIn(path...)` methods instead.
- **`new(T)`**: Prohibited. Returns a pointer.

### 2.4 Supported Builtin Functions
- **`len(c)`**: Supported. Desugars to `persistent.Len(c)`.
- **`make(T, ...)`**: Supported as syntactic sugar (see [Section 6: Transparency & Nuances](#6-transparency--nuances)).
- **`panic`, `recover`**: Supported for standard error handling.
- **`print`, `println`**: Supported for debugging and logging.
- **`complex`, `real`, `imag`**: Supported for complex number math.
- **`max`, `min`**: Supported for pure comparison.

## 3. Variable Binding and Data Flow
### 3.1 Global Bindings
Package-level variables use the `var` keyword. They follow Go's zero-value initialization rules. Note that because `=` is prohibited, package-level variables are effectively immutable once the program starts.

### 3.2 Local Bindings
The `var` keyword is **prohibited** inside functions and blocks. All local bindings must use the short variable declaration operator `:=`.

### 3.3 Identifier Re-binding
Deriving new data from existing data using the same identifier is permitted through `:=`. This is desugared via **SSA-style name mangling** in the generated Go code (e.g., `x` -> `x_1`, `x_2`). This ensures that closures capture the specific version of a variable they were defined with, preserving lexical scope safety.

## 4. Built-in Types
### 4.1 Persistent Collections
- **Maps:** `map[K]V` is desugared to `persistent.Map[K, V]`.
- **Lists:** `[]T` is desugared to `persistent.List[T]`.
- **Nil Punning:** `nil` is a valid empty collection for both maps and lists. Operations on `nil` collections do not panic.

### 4.2 Collection Operations
- **Single-value Access:** `v := m[k]` or `v := l[i]` desugars to a `Get` call.
- **Context-aware Access:** `v, ok := m[k]` or `v, ok := l[i]` desugars to a `Lookup` call.
- **Deep Access:** Chained indexing (e.g., `m["a"]["b"]`) is safe and handles missing intermediate maps by returning zero-values.
- **Deep Updates:** The methods `.SetIn(path..., value)`, `.UpdateIn(path..., fn)`, and `.DeleteIn(path...)` provide recursive updates for nested maps.

## 5. Control Flow
ImGo supports standard Go control flow: `if`, `else`, `for`, `range`, `switch`, `defer`, and `return`. Loop-based data transformations should be implemented using recursion-like patterns through identifier re-binding.

## 6. Semantic Divergence from Go
While ImGo syntax mirrors standard Go, its functional nature leads to different runtime behavior in specific scenarios.

### 6.1 The `make` Builtin
The `make` function in ImGo is purely for initializing empty collections.
- **Maps**: `make(map[K]V, hint)` ignores the capacity hint. It always returns an empty persistent map.
- **Lists**: `make([]T, len, cap)` ignores the `len` and `cap` arguments. Unlike standard Go, **it returns an empty list (length 0)**, not a list pre-filled with zero-values.

### 6.2 Indexing Behavior
Standard Go indexing (`s[i]`) panics if the index is out of bounds. ImGo indexing is **safe by default**:
- **`v := s[i]`**: If `i` is out of bounds or the collection is `nil`, it returns the **zero value** of the element type instead of panicking.
- **`v, ok := s[i]`**: The `ok` value will be `false` for out-of-bounds or `nil` access.

### 6.3 Physical Immutability
Every operation that "changes" a collection returns a brand-new header structure. While structural sharing makes this efficient, it means that no two parts of your program can ever share a "mutable" view of the same data.
