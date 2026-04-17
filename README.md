# 🚀 ImGo

**ImGo** is Go with Clojure-like immutability. While it allows non-pure IO and other side effects (much like Clojure), its core is functional in its approach to data. The main idea is to stay as close to Go as possible while providing the safety and reasoning guarantees that deep immutability provides.

ImGo isn't just about prohibiting mutation; it's about shifting your mental model from "updating state in-place" to "transforming data through a flow." This results in code that is simpler to reason about, trivial to parallelize, and fundamentally more error-proof.

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

## 🔍 Semantic Divergence from Go
While ImGo code looks like standard Go, its functional core leads to several behavioral differences.

### 🚫 Prohibitions
*   **No Mutation:** Operators `=`, `+=`, `++`, `--`, etc., are strictly forbidden.
*   **Restricted Pointers:** `*T`, `*p`, and `&x` are permitted for type signatures and expressions, but in-place mutation via pointers is strictly forbidden.
*   **Builtins:** `append`, `cap`, `clear`, `close`, `copy`, `delete`, and `new` are prohibited.

### 🌊 Immutable Data Flow
*   **Re-binding:** Use `:=` to label the next step in your data's transformation.
*   **Closure Safety:** Captures are "snapshotted" at the time of function definition via SSA mangling.

### 📦 Persistent Collections
*   **Safe Indexing:** `m[k]` and `l[i]` never panic. They return zero-values on miss or out-of-bounds.
*   **The `make` Discrepancy:** `make([]int, 10)` returns an **empty list (length 0)**, not a list of 10 zeros. `make` is purely for collection initialization in ImGo.
*   **Nil Safety:** `nil` is a valid empty collection. `len(nilMap)` is 0, and `nilMap.Set(k, v)` returns a new map with one entry.

## 📚 Documentation
- **[Language Specification](docs/SPEC.md)**: Formal rules and detailed nuances.
- **[EBNF Grammar](docs/GRAMMAR.md)**: Formal syntax definition.
- **[Library Reference](docs/REFERENCE.md)**: API for persistent collections.
- **[Getting Started Tutorial](docs/TUTORIAL.md)**: Build your first ImGo program.

---
*Built with ❤️ for the functional programming community.*
