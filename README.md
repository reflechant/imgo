# 🚀 ImGo (Immutable Go)

**ImGo** is a persistent functional language transpiled to Go. It brings Clojure-inspired safety, deep immutability, and structural sharing to the Go ecosystem.

"ImGo" stands for **Immutable Go**, and it's also a pun: *In my gorgeous opinion*, this is how high-performance concurrency should be written.

## ✨ The ImGo Philosophy

- **No Mutation:** Standard assignment (`=`) is prohibited.
- **No Pointers:** Address-of (`&`) and pointer types (`*T`) are forbidden.
- **Shadowing is First-Class:** Use `:=` to create new versions of your state. `var` is prohibited inside functions.
- **SSA-style Immutability:** The transpiler mangles shadowed names (e.g., `x_1`, `x_2`) so that closures capture the specific version of a variable they were defined with, exactly like Clojure's `let` bindings.
- **Package Globals (def):** Use `var` for package-level declarations. They follow Go's zero-value initialization rules.
- **Persistent by Default:** Uses high-performance Hash Array Mapped Tries (HAMT) and Vector Tries for collections.

## 🛠️ How It Works

### 1. The Source (`.im` files)
Write your logic in `.im` files. ImGo enforces a strict functional core:

```go
package logic

import "github.com/rg/imgo/pkg/persistent"

func main() {
    m := map[string]int{"a": 1}
    m := m.Set("b", 2) // Shadowing! This is a new 'm'.
    
    // x = 10 // ❌ COMPILE ERROR: Assignment is forbidden.
    // p := &m // ❌ COMPILE ERROR: Pointers are forbidden.
}
```

### 2. The Transpilation
The `imgo` tool parses your `.im` files, validates the AST against the ImGo rules, and generates optimized, standard Go code (`_imgo_gen.go`).

## 📚 Documentation
- **[Language Specification](docs/SPEC.md)**: Formal rules of ImGo.
- **[EBNF Grammar](docs/GRAMMAR.md)**: Formal syntax definition.
- **[Library Reference](docs/REFERENCE.md)**: API for persistent collections.
- **[Getting Started Tutorial](docs/TUTORIAL.md)**: Build your first ImGo program.

## 🗺️ Phase 1 Goals (Completed)
- [x] **Project Rename:** Transitioned from Immugo to ImGo.
- [x] **New File Extension:** Support for `.im` source files.
- [x] **Mutation Stripping:** Strict rejection of `=`, `++`, and `--`.
- [x] **Pointer Stripping:** Strict rejection of `*` and `&`.
- [x] **Shadowing Lowering:** Automatic conversion of shadowing `:=` into Go-compatible assignments.

## 📦 Installation & Usage

```bash
# Build the ImGo transpiler
go build -o imgo ./cmd/imgo

# Transpile your project
./imgo ./path/to/your/files.im

# Run the generated Go code
go run ./path/to/your/files_imgo_gen.go
```

---
*Built with ❤️ for the functional programming community.*
