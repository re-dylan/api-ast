package main

import (
	"github.com/zeromicro/api-ast/parser"
	"github.com/zeromicro/api-ast/token"
	"go/ast"
)

func main() {
	fset := token.NewFileSet() // positions are relative to fset

	src := `
/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
 */

// api语法版本
syntax = "v1"

info(
	author: "dylan"
	date: "2022-01-28"
	desc: "api 语法 demo"
)

import "a/b.api"

import(
	"c.api"
	"d/d.api"
)

type (

	User { // User for 注释
		Name string 
	}
)

service foo-api {
  @handler ping
  get /ping

  @doc "foo"
  @handler bar
  post /bar/:id (Foo)
}
`

	// Parse src but stop after processing the imports.
	f, err := parser.ParseFile(fset, "", src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return
	}
	ast.Print(fset, f)
}
