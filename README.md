# 🚀 ImGo

**ImGo** is Go with Clojure-like immutability. While it allows non-pure IO and other side effects (much like Clojure), its core is functional in its approach to data. The main idea is to stay as close to Go as possible while providing the safety and reasoning guarantees that deep immutability provides.

ImGo isn't just about prohibiting mutation; it's about shifting your mental model from "updating state in-place" to "transforming data through a chain of functions." This results in code that is simpler to reason about, trivial to parallelize, and fundamentally more error-proof.

## 🛠️ How it Works
ImGo is a **transpiler** that generates Go code from `.im` source files. It validates your ImGo code against strict immutability rules and lowers functional patterns (like identifier re-binding and recursive nested updates) into efficient Go code that leverages persistent data structures.

## ⚡ Quick Start (For the Impatient)

```bash
# 1. Build the ImGo transpiler
go build -o imgo ./cmd/imgo

# 2. Transpile the examples (you can point ImGo to a directory or to a single file)
# BTW, check out docs/tutorial.im
./imgo pkg/transpiler/testdata

# 3. Run a generated example
go run pkg/transpiler/testdata/list_imgo_gen.go
```

## 🔍 Semantic Divergence from Go
While ImGo code looks like standard Go, its functional core leads to several behavioral and syntactical differences.

### 🚫 Prohibitions
*   **No mutation in place:** Operators `=`, `+=`, `++`, `--`, etc., are strictly forbidden.
*   **Restricted pointers:** `*T`, `*p`, and `&x` are permitted for type signatures and expressions, but in-place mutation via pointers is strictly forbidden.
*   **Builtins:** `cap`, `clear`, `close`, `copy`, `delete`, and `new` are prohibited. `append` and `len` are supported.

### 🌊 Immutable Data
In order to fully comprehend the concepts behind ImGo it's highly recommended to watch the talk "Value of Values" by Rich Hickey. Another good talk of his is "Simple made easy".

In ImGo you can not update data in place like you can "change the value of a variable" in Go.

ImGo, like Clojure, decouples the notions of "identity" and "value". Identity is what you're used to call "variable name". It's a name, bound to a certain value. 

Values are immutable, they **never** change. You can safely store an ImGo value or a pointer to it knowing that it cannot be changed by anyone from the outside.

In ImGo you can "reassign" a value using Go syntax `:=`.

This is correct ImGo:
```go
x := 2
f := func() { return x }
x := 3
```

Because `x` is an identity and when you refer to it by name you get the **current** state which is an immutable value function `f` will always return `2` no matter what you do.


### 📦 Persistent Collections
Like Go, Imgo has maps, arrays and slices. The difference is they are immutable just like strings in Go. It's achieved by building them on top of a library (github.com/benbjohnson/immutable) of persistent data structures that allow us to efficiently store all the history of changes for as long as it's needed.

*   **Safe Indexing:** `m[k]` and `l[i]` never panic. They return zero-values on miss or out-of-bounds.
*   **The `make` Discrepancy:** `make([]int, 10)` returns an **empty list (length 0)**, not a list of 10 zeros. `make` is purely for collection initialization in ImGo.
*   **Nil Safety:** `nil` is a valid empty collection. `len(nilMap)` is 0, and `nilMap.Set(k, v)` returns a new map with one entry.

## 📚 Documentation
- **[Language Specification](docs/SPEC.md)**: Formal rules and detailed nuances.
- **[EBNF Grammar](docs/GRAMMAR.md)**: Formal syntax definition.
- **[Library Reference](docs/REFERENCE.md)**: API for persistent collections.
- **[Getting Started Tutorial](docs/TUTORIAL.md)**: Build your first ImGo program.

---
*Built with ❤️ for the Go community.*

## 🤖 AI Disclosure
This project is developed and maintained with the assistance of AI models, including Claude 4.6, 4.7, and Gemini 3.
