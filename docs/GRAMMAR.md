# ImGo EBNF Grammar

The ImGo grammar is a restricted subset of the Go grammar. It removes pointers, direct assignment, and increment/decrement operations.

## 1. Top-Level Structure
```ebnf
SourceFile      = PackageClause ";" { ImportDecl ";" } { TopLevelDecl ";" } .
PackageClause   = "package" PackageName .
ImportDecl      = "import" ( ImportSpec | "(" { ImportSpec ";" } ")" ) .
ImportSpec      = [ "." | PackageName ] ImportPath .
TopLevelDecl    = Declaration | FunctionDecl .
```

## 2. Declarations
ImGo `var` declarations are restricted compared to Go.
```ebnf
Declaration    = ConstDecl | TypeDecl | VarDecl .
ConstDecl      = "const" ( ConstSpec | "(" { ConstSpec ";" } ")" ) .
VarDecl        = "var" ( VarSpec | "(" { VarSpec ";" } ")" ) .
VarSpec        = IdentifierList [ Type ] "=" ExpressionList . (* Must have assignment *)

TypeDecl       = "type" ( TypeSpec | "(" { TypeSpec ";" } ")" ) .
TypeSpec       = Identifier [ "=" ] Type .
```

## 3. Functions and Statements
```ebnf
FunctionDecl   = "func" FunctionName Signature [ FunctionBody ] .
FunctionBody   = Block .

Block          = "{" StatementList "}" .
StatementList  = { Statement ";" } .

Statement      = Declaration | ShortVarDecl | ReturnStmt | 
                 Block | IfStmt | SwitchStmt | ForStmt | ExprStmt .

ShortVarDecl   = IdentifierList ":=" ExpressionList .
ExprStmt       = Expression .
ReturnStmt     = "return" [ ExpressionList ] .
```

## 4. Expressions and Types
```ebnf
Type           = TypeName | StructType | InterfaceType | MapType | SliceType | ArrayType .
TypeName       = Identifier | QualifiedIdent .
MapType        = "map" "[" Type "]" Type .
SliceType      = "[" "]" Type .
ArrayType      = "[" int_lit "]" Type .

(* Pointers are permitted for read-only access (*p) and in type signatures (*T) *)
```

## 5. Control Flow
Standard Go control flow syntax is preserved, but any expressions used within them must be side-effect free.
```ebnf
IfStmt         = "if" [ SimpleStmt ";" ] Expression Block [ "else" ( IfStmt | Block ) ] .
ForStmt        = "for" [ Condition | ForClause | RangeClause ] Block .
```
