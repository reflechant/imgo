# Getting Started with ImGo

ImGo is a functional language that transpiles to Go. This guide will help you set up your first ImGo project.

## 1. Installation
Ensure you have a recent version of Go (1.23 or newer) installed.

```bash
# Clone the project and build the imgo tool
git clone https://github.com/rg/imgo.git
cd imgo
go build -o imgo ./cmd/imgo
```

## 2. Your First ImGo Program
Create a file named `hello.im`. Note the `.im` extension.

```go
package main

import "fmt"

func main() {
    // 1. Immutable State through Shadowing
    x := 5
    x := x + 10
    fmt.Printf("x is %d\n", x) // Prints 15

    // 2. Persistent Collections
    m := map[string]int{"a": 1, "b": 2}
    m := m.Set("c", 3) // Re-bind 'm' to a new map version
    fmt.Printf("Map size: %d\n", len(m)) // Prints 3

    // 3. Iteration
    fmt.Print("Elements: ")
    for k, v := range m.All() {
        fmt.Printf("%s:%d ", k, v)
    }
    fmt.Println()
}
```

## 3. Transpilation and Execution
Use the `imgo` CLI to generate standard Go code, then run it.

```bash
./imgo hello.im
go run hello_imgo_gen.go
```

## 4. Key Differences from Go
If you try to use standard Go mutation, ImGo will stop you:

```go
// ❌ Error: assignment (=) is prohibited
x = 10 

// ❌ Error: pointers are prohibited
p := &x 

// ❌ Error: mutation (++) is prohibited
x++ 
```

By enforcing these rules, ImGo ensures that your logic is free of data races and hidden side effects, while still being as fast as the Go runtime allows.
