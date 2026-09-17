package ir_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
	"honnef.co/go/tools/go/ir"
)

func BenchmarkSSASingleBlockLocals(b *testing.B) {
	for _, size := range []int{64, 256, 1024} {
		b.Run(fmt.Sprintf("locals=%d", size), func(b *testing.B) {
			var src strings.Builder
			src.WriteString("package bench\nfunc f(a int) int {\n\ttotal := 0\n")
			for i := range size {
				fmt.Fprintf(&src, "\tx%d := a\n\ttotal += x%d\n", i, i)
			}
			src.WriteString("\treturn total\n}\n")

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "bench.go", src.String(), parser.SkipObjectResolution)
			if err != nil {
				b.Fatal(err)
			}
			pkg := types.NewPackage("example.com/bench", "bench")
			info := &types.Info{
				Types:        make(map[ast.Expr]types.TypeAndValue),
				Defs:         make(map[*ast.Ident]types.Object),
				Uses:         make(map[*ast.Ident]types.Object),
				Implicits:    make(map[ast.Node]types.Object),
				Scopes:       make(map[ast.Node]*types.Scope),
				Selections:   make(map[*ast.SelectorExpr]*types.Selection),
				Instances:    make(map[*ast.Ident]types.Instance),
				FileVersions: make(map[*ast.File]string),
			}
			if err := types.NewChecker(&types.Config{}, fset, pkg, info).Files([]*ast.File{file}); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				prog := ir.NewProgram(fset, 0)
				irpkg := prog.CreatePackage(pkg, []*ast.File{file}, info, true)
				irpkg.Build()
			}
		})
	}
}

func BenchmarkSSA(b *testing.B) {
	cfg := &packages.Config{
		Mode:  packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "std")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prog := ir.NewProgram(pkgs[0].Fset, ir.GlobalDebug)
		seen := map[*packages.Package]struct{}{}
		var create func(pkg *packages.Package)
		create = func(pkg *packages.Package) {
			if _, ok := seen[pkg]; ok {
				return
			}
			seen[pkg] = struct{}{}
			prog.CreatePackage(pkg.Types, pkg.Syntax, pkg.TypesInfo, true)
			for _, imp := range pkg.Imports {
				create(imp)
			}
		}
		for _, pkg := range pkgs {
			create(pkg)
		}
		prog.Build()
	}
}
