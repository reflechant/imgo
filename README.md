# 🚀 ImGo

**ImGo** is Go with Clojure-like immutability. While it allows non-pure IO and other side effects (much like Clojure), its core is functional in its approach to data. The main idea is to stay as close to Go as possible while providing the safety and reasoning guarantees that deep immutability provides.

ImGo isn't just about prohibiting mutation; it's about shifting your mental model from "updating state in-place" (place-oriented programming or PLOP as Rich Hickey calls it) to "passing data through a chain of transformations" This results in code that is simpler to reason about, trivial to parallelize, and fundamentally more error-proof.

## 🛠️ How it Works
ImGo is a **transpiler** that generates standard, optimized Go code from `.im` source files. It validates your code against strict immutability rules and lowers functional patterns (like identifier re-binding and recursive nested updates) into efficient Go code that leverages persistent data structures.

## ⚡ Quick Start (For the Impatient)

```bash
# 1. Build the ImGo transpiler
go build -o imgo ./cmd/imgo

# 2. Transpile the examples
./imgo example/

# 3. Run a generated example
go run example/main_imgo_gen.go
```

## 🔍 Differences Between Go and ImGo

### 🚫 Prohibited Operators and Builtins
To ensure deep immutability, the following Go features are strictly prohibited:
*   **Direct Assignment:** The `=` operator is forbidden.
*   **Compound Assignments:** All operators like `+=`, `-=`, `*=`, etc., are forbidden.
*   **Increments/Decrements:** `++` and `--` are forbidden.
*   **Pointers:** Pointer types (`*T`), pointer dereferences (`*p`), and the address-of operator (`&`) are forbidden.
*   **Builtin `delete`:** The `delete(m, k)` builtin is prohibited. Use the `.Delete(k)` or `.DeleteIn(path...)` methods instead.
*   **Local `var`:** Using the `var` keyword inside functions or blocks is prohibited. Use `:=` for all local bindings.

### 🌊 Immutable Data Flow
Instead of mutating objects, you derive new data from existing data.
*   **Identifier Re-binding:** You can "reuse" a name with `:=` to represent the next step in your data's transformation. The old value will always be accessible to those who hold a reference to it. No more pointer-based rug pulling and updating things in place without you knowing.
*   **Closure Safety:** The transpiler ensures that if a function captures a variable, it captures the exact version present at the time of definition, even if the identifier is re-bound later in the flow.

### 📦 Persistent Collections
ImGo replaces Go's built-in mutable maps and slices with high-performance, persistent data structures (HAMT and Vector Tries) that support structural sharing.
*   **Maps:** `map[K]V` is sugar for `persistent.Map[K, V]`.
*   **Lists:** `[]T` is sugar for `persistent.List[T]`.
*   **Methods:** Operations like `.Set(k, v)`, `.Append(v)`, and `.Delete(k)` return a **new** collection, leaving the original instance entirely unchanged.

### ✨ Syntactic Sugar & Convenience
*   **Nil Punning:** `nil` is a valid empty collection. Operations on `nil` collections work gracefully without panicking.
*   **Deep Updates:** Provided methods `SetIn`, `UpdateIn`, and `DeleteIn` handle deeply nested maps, automatically creating intermediate maps as needed.
*   **Native Indexing:** Standard Go indexing `m["key"]` and `l[i]` is desugared to safe `Get` or `Lookup` method calls.

## 📚 Documentation
- **[Language Specification](docs/SPEC.md)**: Formal rules of ImGo.
- **[EBNF Grammar](docs/GRAMMAR.md)**: Formal syntax definition.
- **[Library Reference](docs/REFERENCE.md)**: API for persistent collections.
- **[Getting Started Tutorial](docs/TUTORIAL.md)**: Build your first ImGo program.

---
*Built with ❤️ for the Go community.*
