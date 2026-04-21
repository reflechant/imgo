package transpiler

import (
	"fmt"
	"go/token"
	"strings"
)

// Error codes are stable strings so editor integrations (Stage 4 LSP)
// can route quick-fixes on them.
const (
	CodeDisallowedAssignment = "E001"
	CodeDisallowedIncDec     = "E002"
	CodeDisallowedVar        = "E003"
	CodeDisallowedBuiltin    = "E010"
	CodeDisallowedChanType   = "E020"
	CodeDisallowedGoStmt     = "E021"
	CodeDisallowedChanOp     = "E022"
	CodeDisallowedSelectStmt = "E023"
	CodeDisallowedFullSlice  = "E030"
)

// Diagnostic is a single validation error carrying enough position
// information to point a user at the offending source.
type Diagnostic struct {
	Pos     token.Position
	Code    string
	Message string
}

// Error implements the error interface.
func (d Diagnostic) Error() string {
	return fmt.Sprintf("%s: error[%s] %s", d.Pos, d.Code, d.Message)
}

// Diagnostics is a list of Diagnostic that implements the error interface.
// An empty Diagnostics should be returned as nil by callers (see asError).
type Diagnostics []Diagnostic

// Error implements the error interface, joining all diagnostics into a single string.
func (ds Diagnostics) Error() string {
	var b strings.Builder
	for i, d := range ds {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(d.Error())
	}
	return b.String()
}

// asError returns nil if ds is empty, otherwise ds as an error.
func (ds Diagnostics) asError() error {
	if len(ds) == 0 {
		return nil
	}
	return ds
}
