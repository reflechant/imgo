package transpiler

import (
	"errors"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Parallel()
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
			name: "Allowed delete builtin (ImGo value-update form)",
			code: `package main
func main() {
    m := map[string]int{"a": 1}
    m1 := delete(m, "a")
    println(m1)
}`,
			wantErr: "",
		},
		{
			name: "Allowed append builtin",
			code: `package main
func main() {
    s := []int{1}
    s2 := append(s, 2)
    _ := s2
}`,
			wantErr: "",
		},
		{
			name: "Prohibited append on array",
			code: `package main
func main() {
    a := [2]int{1, 2}
    l := append(a, 3)
    _ := l
}`,
			wantErr: "builtin 'append' is prohibited on fixed-size arrays",
		},
		{
			name: "Prohibited delete on array",
			code: `package main
func main() {
    a := [2]int{1, 2}
    delete(a, 0)
}`,
			wantErr: "builtin 'delete' is prohibited on fixed-size arrays",
		},
		{
			name: "Prohibited set on array",
			code: `package main
func main() {
    a := [2]int{1, 2}
    a2 := set(a, 0, 10)
    _ := a2
}`,
			wantErr: "builtin 'set' is prohibited on fixed-size arrays",
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
		{
			name: "Prohibited RangeStmt two-value mutation =",
			code: `package main
func main() {
    l := []int{1, 2}
    var i, v int
    for i, v = range l {
        _ = i
        _ = v
    }
}`,
			wantErr: "mutation operator = is prohibited",
		},
		{
			name: "Prohibited chan type",
			code: `package main
type C chan int`,
			wantErr: "channel types are prohibited",
		},
		{
			name: "Prohibited go statement",
			code: `package main
func main() {
    go func() {}()
}`,
			wantErr: "'go' statement is prohibited",
		},
		{
			name: "Prohibited chan send",
			code: `package main
func main() {
    var c chan int
    c <- 1
}`,
			wantErr: "channel send '<-' is prohibited",
		},
		{
			name: "Prohibited chan receive",
			code: `package main
func main() {
    var c chan int
    <-c
}`,
			wantErr: "channel receive '<-' is prohibited",
		},
		{
			name: "Prohibited select statement",
			code: `package main
func main() {
    select {}
}`,
			wantErr: "'select' statement is prohibited",
		},
		{
			name: "Prohibited three-index slice",
			code: `package main
func main() {
    s := []int{1, 2, 3}
    _ = s[0:2:3]
}`,
			wantErr: "three-index slice",
		},
		{
			name: "Valid two-index slice",
			code: `package main
func main() {
    s := []int{1, 2, 3}
    a := s[1:2]
    b := s[:2]
    c := s[1:]
    d := s[:]
    println(a, b, c, d)
}`,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.im", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse test code: %v", err)
			}

			info := typeCheck(fset, f)
			err = Validate(fset, f, info)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidateAccumulatesDiagnostics(t *testing.T) {
	t.Parallel()
	code := `package main
func main() {
	x := 5
	x = 10
	x++
	y := new(int)
	println(y)
}`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.im", code, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	verr := Validate(fset, f, nil)
	if verr == nil {
		t.Fatal("expected diagnostics, got nil")
	}
	var ds Diagnostics
	if !errors.As(verr, &ds) {
		t.Fatalf("expected Diagnostics, got %T", verr)
	}
	if len(ds) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d: %v", len(ds), ds)
	}

	wantCodes := []string{CodeDisallowedAssignment, CodeDisallowedIncDec, CodeDisallowedBuiltin}
	for i, d := range ds {
		if d.Code != wantCodes[i] {
			t.Errorf("ds[%d].Code = %q, want %q", i, d.Code, wantCodes[i])
		}
		if d.Pos.Filename != "test.im" {
			t.Errorf("ds[%d].Pos.Filename = %q, want test.im", i, d.Pos.Filename)
		}
		if d.Pos.Line == 0 || d.Pos.Column == 0 {
			t.Errorf("ds[%d] missing position: %+v", i, d.Pos)
		}
	}
}

func TestDiagnosticFormat(t *testing.T) {
	t.Parallel()
	code := `package main
func main() {
	x := 5
	x = 10
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.im", code, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	verr := Validate(fset, f, nil)
	if verr == nil {
		t.Fatal("expected error, got nil")
	}
	msg := verr.Error()
	want := "test.im:4:2: error[E001] mutation operator = is prohibited"
	if !strings.Contains(msg, want) {
		t.Errorf("Error() = %q, want substring %q", msg, want)
	}
}

func TestEmptyDiagnostics(t *testing.T) {
	t.Parallel()
	var ds Diagnostics
	if got := ds.Error(); got != "" {
		t.Errorf("empty Diagnostics.Error() = %q, want empty", got)
	}
}
