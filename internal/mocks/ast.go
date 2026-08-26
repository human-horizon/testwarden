package mocks

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// astImport tracks an import alias and its underlying path.
type astImport struct {
	alias string
	path  string
}

// astFile extracts imports and detects mock-library usage via real AST parsing.
type astFile struct {
	imports     []astImport
	usesMock    bool
	mockFuncs   []string
	mockAssigns []int // line numbers
}

// parseGoAST parses a single Go file and returns its AST-derived structure.
func parseGoAST(path string) (*astFile, error) {
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	out := &astFile{}

	// Walk top-level decls only for performance.
	for _, imp := range src.Imports {
		if imp.Path == nil {
			continue
		}
		pathValue := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		out.imports = append(out.imports, astImport{alias: alias, path: pathValue})
	}

	ast.Inspect(src, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if isMockCall(x.Fun) {
				out.usesMock = true
				out.mockFuncs = append(out.mockFuncs, exprName(x.Fun))
			}
		case *ast.SelectorExpr:
			// Detect mock.Mock instantiation: &mocks.Mock{...} or sqlmock.New(...)
			if isMockType(x) {
				out.usesMock = true
			}
		case *ast.CompositeLit:
			if isMockCompositeLit(x) {
				out.usesMock = true
			}
		case *ast.AssignStmt:
			// _ = mocks.Mock{} — detect assignments referencing mock packages.
			for _, rhs := range x.Rhs {
				if isMockType(rhs) || isMockCompositeLit(rhs) {
					out.usesMock = true
					out.mockAssigns = append(out.mockAssigns, fset.Position(n.Pos()).Line)
				}
			}
		}
		return true
	})

	return out, nil
}

func isMockCall(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); ok {
		return strings.Contains(strings.ToLower(id.Name), "mock")
	}
	return false
}

func isMockType(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(ident.Name), "mock") ||
		strings.Contains(strings.ToLower(sel.Sel.Name), "mock")
}

func isMockCompositeLit(e ast.Expr) bool {
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return false
	}
	return isMockType(cl.Type)
}

func exprName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprName(x.X) + "." + x.Sel.Name
	}
	return ""
}

// realDepForImport returns the configured real dep that matches the import path,
// or empty string.
func realDepForImport(imp astImport, realDeps map[string]bool) string {
	lower := strings.ToLower(imp.path)
	for real := range realDeps {
		if strings.Contains(lower, strings.ToLower(real)) {
			return real
		}
	}
	// Also check alias.
	if imp.alias != "" {
		aliasLower := strings.ToLower(imp.alias)
		for real := range realDeps {
			if strings.Contains(aliasLower, strings.ToLower(real)) {
				return real
			}
		}
	}
	return ""
}

// mockLibForImport returns the first mock library import, or empty.
func mockLibForImport(imp astImport) string {
	return matchMockLib(imp.path)
}

func matchMockLib(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "gomock"):
		return path
	case strings.Contains(lower, "testify/mock"):
		return path
	case strings.Contains(lower, "sqlmock"):
		return path
	case strings.Contains(lower, "mock") && strings.Contains(lower, "/"):
		return path
	}
	return ""
}
