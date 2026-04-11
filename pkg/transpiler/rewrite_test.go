package transpiler

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

func TestRewrite(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "True shadowing with mangling",
			input: `package main
func main() {
	x := 5
	f := func() { fmt.Println(x) }
	x := 10
	f()
}`,
			expected: []string{
				"x_1 := 5",
				"fmt.Println(x_1)",
				"x_3 := 10",
			},
		},
		{
			name: "Map literal and indexing",
			input: `package main
func main() {
	m := map[string]int{"a": 1}
	v := m["a"]
	v, ok := m["b"]
}`,
			expected: []string{
				"m_1 := persistent.NewMap[string, int]().Set(\"a\", 1)",
				"v_2 := m_1.Get(\"a\")",
				"v_3, ok_4 := m_1.Lookup(\"b\")",
			},
		},
		{
			name: "Slice literal and appending",
			input: `package main
func main() {
	l := []int{1, 2}
	l := l.Append(3)
	x := l[0]
}`,
			expected: []string{
				"l_1 := persistent.NewList[int]().Append(1).Append(2)",
				"l_2 := l_1.Append(3)",
				"x_3 := l_2.Get(0)",
			},
		},
		{
			name: "If statement with init and else if",
			input: `package main
func main() {
	if x := 5; x > 0 {
		x := 10
		fmt.Println(x)
	} else if y := 2; y > 0 {
        fmt.Println(y)
    } else {
        fmt.Println("else")
    }
}`,
			expected: []string{
				"if x_1 := 5; x_1 > 0",
				"x_2 := 10",
				"else if y_3 := 2; y_3 > 0",
			},
		},
		{
			name: "Range statement",
			input: `package main
func main() {
	l := []int{1, 2}
	for i, v := range l {
		fmt.Println(i, v)
	}
}`,
			expected: []string{
				"for i, v := range l_1",
			},
		},
		{
			name: "SetIn and UpdateIn expansion",
			input: `package main
func main() {
	m := map[string]any{}
	m := m.SetIn("a", "b", 1)
	m := m.UpdateIn("a", "b", func(v any) any { return v })
}`,
			expected: []string{
				"m_2 := m_1.Set(\"a\", m_1.Get(\"a\").Set(\"b\", 1))",
				"m_3 := m_2.Set(\"a\", m_2.Get(\"a\").Update(\"b\", func(v any) any { return v }))",
			},
		},
		{
			name: "Package level var types and non-value specs",
			input: `package main
const Pi = 3.14
type MyInt int
var m map[string]int
var l []int`,
			expected: []string{
				"const Pi = 3.14",
				"type MyInt int",
				"var m persistent.Map[string, int]",
				"var l persistent.List[int]",
			},
		},
        {
            name: "Return statement",
            input: `package main
func f() int {
    x := 5
    return x
}`,
            expected: []string{
                "return x_1",
            },
        },
        {
            name: "Nested blocks and const",
            input: `package main
func main() {
    x := 1
    const y = 2
    {
        x := 2
        fmt.Println(x, y)
    }
    fmt.Println(x)
}`,
            expected: []string{
                "x_1 := 1",
                "const y = 2",
                "x_2 := 2",
                "fmt.Println(x_2, y)",
                "fmt.Println(x_1)",
            },
        },
        {
            name: "Switch and defer and expressions",
            input: `package main
func main() {
    x := 5
    defer fmt.Println(x)
    switch y := (x + 1); y {
    case 6:
        fmt.Println(y)
    default:
        fmt.Println("default")
    }
    s := []int{1, 2, 3}
    s2 := s[1:2:3]
    var a any = s2
    s3 := a.([]int)
}`,
            expected: []string{
                "x_1 := 5",
                "defer fmt.Println(x_1)",
                "switch y_2 := (x_1 + 1); y_2",
                "case 6:",
                "fmt.Println(y_2)",
                "s_3 := persistent.NewList[int]().Append(1).Append(2).Append(3)",
                "s2_4 := s_3[1:2:3]",
                "var a_5 any = s2_4",
                "s3_6 := a_5.(persistent.List[int])",
            },
        },
        {
            name: "Slice and TypeAssert",
            input: `package main
func main() {
    s := []int{1, 2, 3}
    s2 := s[:1]
    s3 := s[1:]
    var a any = s
    s4 := a.(map[string]int)
}`,
            expected: []string{
                "s2_2 := s_1[:1]",
                "s3_3 := s_1[1:]",
                "var a_4 any = s_1",
                "s4_5 := a_4.(persistent.Map[string, int])",
            },
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.im", tt.input, 0)
			if err != nil {
				t.Fatalf("Failed to parse input code: %v", err)
			}

			f = Rewrite(f)

			var buf bytes.Buffer
			printer.Fprint(&buf, fset, f)
			got := buf.String()

			for _, exp := range tt.expected {
				if !strings.Contains(got, exp) {
					t.Errorf("Rewrite() output missing %q. Got:\n%s", exp, got)
				}
			}
		})
	}
}

func TestRewriteEdgeCases(t *testing.T) {
    // Test rewriteBlock(nil)
    rewriteBlock(nil, nil, nil)
    
    // Test rewriteExpr(nil)
    if rewriteExpr(nil, nil, nil, false) != nil {
        t.Errorf("Expected nil for rewriteExpr(nil)")
    }
}
