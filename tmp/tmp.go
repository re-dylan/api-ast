package main

import "go/ast"

type U struct {
	Name string
}

func main() {
	u := &U{Name: "dylan"}
	ast.Print(nil, u)
}
