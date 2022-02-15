package parser

import (
	"fmt"
	"github.com/zeromicro/api-ast/token"
)

func ExampleParseFile() {
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
	User {
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
	f, err := ParseFile(fset, "", src, AllErrors|ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(f.Syntax.Name.Value)

	// Print the imports from the file's AST.
	for _, s := range f.Imports {
		fmt.Println(s.Path.Value)
	}

	// output:
	//
	// "v1"
	// "a/b.api"
	// "c.api"
	// "d/d.api"
}
