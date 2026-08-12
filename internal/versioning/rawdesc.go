package versioning

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// rawDescFromPBGo extracts the file descriptor bytes from protoc-gen-go rawDesc const (Go AST, not .proto).
func rawDescFromPBGo(path string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if name == nil || !strings.Contains(name.Name, "proto_rawDesc") {
					continue
				}
				if i >= len(vs.Values) {
					continue
				}
				val, err := evalStringExpr(vs.Values[i])
				if err != nil {
					return nil, err
				}
				return []byte(val), nil
			}
		}
	}
	return nil, fmt.Errorf("rawDesc const not found in %s", path)
}

func evalStringExpr(expr ast.Expr) (string, error) {
	var eval func(ast.Expr) (constant.Value, error)
	eval = func(e ast.Expr) (constant.Value, error) {
		switch x := e.(type) {
		case *ast.BinaryExpr:
			if x.Op != token.ADD {
				return nil, fmt.Errorf("unsupported expr op %s", x.Op)
			}
			l, err := eval(x.X)
			if err != nil {
				return nil, err
			}
			var r constant.Value
			r, err = eval(x.Y)
			if err != nil {
				return nil, err
			}
			return constant.BinaryOp(l, token.ADD, r), nil
		case *ast.BasicLit:
			if x.Kind != token.STRING {
				return nil, fmt.Errorf("expected string literal")
			}
			s, err := strconv.Unquote(x.Value)
			if err != nil {
				return nil, err
			}
			return constant.MakeString(s), nil
		case *ast.ParenExpr:
			return eval(x.X)
		default:
			return nil, fmt.Errorf("unsupported expr %T", e)
		}
	}
	v, err := eval(expr)
	if err != nil {
		return "", err
	}
	if v.Kind() != constant.String {
		return "", fmt.Errorf("expected string constant")
	}
	return constant.StringVal(v), nil
}
