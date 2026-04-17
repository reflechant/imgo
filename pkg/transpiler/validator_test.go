package transpiler

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr string
	}{
		{
			name: "Valid shadowing",
			code: `package main
func main() {
	x := 5
	x := 10
}`,
			wantErr: "",
		},
		{
			name: "Invalid assignment =",
			code: `package main
func main() {
	x := 5
	x = 10
}`,
			wantErr: "mutation operator = is prohibited",
		},
		{
			name: "Invalid compound assignment +=",
			code: `package main
func main() {
	x := 5
	x += 1
}`,
			wantErr: "mutation operator += is prohibited",
		},
		{
			name: "Invalid compound assignment -=",
			code: `package main
func main() {
	x := 5
	x -= 1
}`,
			wantErr: "mutation operator -= is prohibited",
		},
		{
			name: "Invalid compound assignment *=",
			code: `package main
func main() {
	x := 5
	x *= 2
}`,
			wantErr: "mutation operator *= is prohibited",
		},
		{
			name: "Invalid compound assignment /=",
			code: `package main
func main() {
	x := 10
	x /= 2
}`,
			wantErr: "mutation operator /= is prohibited",
		},
		{
			name: "Invalid compound assignment %=",
			code: `package main
func main() {
	x := 10
	x %= 3
}`,
			wantErr: "mutation operator %= is prohibited",
		},
		{
			name: "Invalid compound assignment &=",
			code: `package main
func main() {
	x := 1
	x &= 0
}`,
			wantErr: "mutation operator &= is prohibited",
		},
		{
			name: "Invalid compound assignment |=",
			code: `package main
func main() {
	x := 1
	x |= 0
}`,
			wantErr: "mutation operator |= is prohibited",
		},
		{
			name: "Invalid compound assignment ^=",
			code: `package main
func main() {
	x := 1
	x ^= 1
}`,
			wantErr: "mutation operator ^= is prohibited",
		},
		{
			name: "Invalid compound assignment <<=",
			code: `package main
func main() {
	x := 1
	x <<= 1
}`,
			wantErr: "mutation operator <<= is prohibited",
		},
		{
			name: "Invalid compound assignment >>=",
			code: `package main
func main() {
	x := 1
	x >>= 1
}`,
			wantErr: "mutation operator >>= is prohibited",
		},
		{
			name: "Invalid compound assignment &^=",
			code: `package main
func main() {
	x := 1
	x &^= 1
}`,
			wantErr: "mutation operator &^= is prohibited",
		},
		{
			name: "Invalid increment",
			code: `package main
func main() {
	x := 5
	x++
}`,
			wantErr: "mutation (++, --) is prohibited",
		},
		{
			name: "Valid pointer type in signature",
			code: `package main
func MyFunc(p *int) {}`,
			wantErr: "",
		},
		{
			name: "Valid address-of (&)",
			code: `package main
func main() {
	x := 5
	p := &x
}`,
			wantErr: "",
		},
		{
			name: "Valid dereference",
			code: `package main
func main() {
	x := 5
	p := &x
	y := *p
	println(y)
}`,
			wantErr: "",
		},
		{
			name: "Invalid pointer mutation",
			code: `package main
func main() {
	x := 5
	p := &x
	*p = 10
}`,
			wantErr: "mutation operator = is prohibited",
		},
		{
			name: "Invalid field mutation",
			code: `package main
type S struct { F int }
func main() {
	s := &S{F: 1}
	s.F = 2
}`,
			wantErr: "mutation operator = is prohibited",
		},
		{
			name: "Valid package var with init",
			code: `package main
var MyVal = 10`,
			wantErr: "",
		},
		{
			name: "Valid package var without init (Zero-value)",
			code: `package main
var MyVal int`,
			wantErr: "",
		},
		{
			name: "Invalid local var declaration",
			code: `package main
func main() {
	var x = 5
}`,
			wantErr: "'var' is prohibited inside blocks",
		},
		{
			name: "Prohibited delete builtin",
			code: `package main
func main() {
    m := map[string]int{"a": 1}
    delete(m, "a")
}`,
			wantErr: "'delete' builtin is prohibited",
		},
		{
			name: "Prohibited append builtin",
			code: `package main
func main() {
    s := []int{1}
    s2 := append(s, 2)
}`,
			wantErr: "builtin 'append' is prohibited",
		},
		{
			name: "Prohibited cap builtin",
			code: `package main
func main() {
    s := []int{1}
    c := cap(s)
}`,
			wantErr: "builtin 'cap' is prohibited",
		},
		{
			name: "Prohibited new builtin",
			code: `package main
func main() {
    x := new(int)
}`,
			wantErr: "builtin 'new' is prohibited",
		},
		{
			name: "Prohibited clear builtin",
			code: `package main
func main() {
    m := map[string]int{"a": 1}
    clear(m)
}`,
			wantErr: "builtin 'clear' is prohibited",
		},
		{
			name: "Prohibited close builtin",
			code: `package main
func main() {
    c := make(chan int)
    close(c)
}`,
			wantErr: "builtin 'close' is prohibited",
		},
		{
			name: "Prohibited copy builtin",
			code: `package main
func main() {
    s1 := []int{1}
    s2 := []int{2}
    copy(s1, s2)
}`,
			wantErr: "builtin 'copy' is prohibited",
		},
		{
			name: "Prohibited RangeStmt mutation =",
			code: `package main
func main() {
    l := []int{1, 2}
    var i int
    for i = range l {
        _ = i
    }
}`,
			wantErr: "mutation operator = is prohibited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.im", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse test code: %v", err)
			}

			err = Validate(fset, f)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				} else if !contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
