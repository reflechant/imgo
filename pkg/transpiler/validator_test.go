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
			name: "Invalid pointer type in signature",
			code: `package main
func MyFunc(p *int) {}`,
			wantErr: "pointers are prohibited",
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
			name: "Invalid address-of (&)",

			code: `package main
func main() {
	x := 5
	p := &x
}`,
			wantErr: "address-of (&) is prohibited",
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
