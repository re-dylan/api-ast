package main

import (
	"bytes"
	"fmt"
	"go/printer"

	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"

	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/parser"
	"github.com/zeromicro/api-ast/token"
)

func main() {
	goAstPrinter()
	return
	fset := token.NewFileSet() // positions are relative to fset

	src := `
syntax = "v1"

// User for name
type User {
	Name string
}
`

	// Parse src but stop after processing the imports.
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.Trace)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(f.Doc.Text())

	// Print the imports from the file's AST.
	for _, s := range f.Decls {
		a := s.(*ast.GenDecl)

		fmt.Printf("%+v\n", a.Doc.Text())
	}

	// output:
	//
	// "fmt"
	// "time"
}

func goPrinter() {
	fset := gotoken.NewFileSet() // positions are relative to fset

	src := `
package main

type User struct {
Name string
}
`

	// Parse src but stop after processing the imports.
	f, err := goparser.ParseFile(fset, "", src, goparser.ParseComments|goparser.Trace)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(f.Doc.Text())

	var gofmtBuf bytes.Buffer
	printer.Fprint(&gofmtBuf, fset, f)
	fmt.Println(gofmtBuf.String())
}

type User struct {
	Nameas string
}

func goAstPrinter() {
	var u User

	goast.Print(nil, u)
}
