# AST FOR API


## GenDecl
包含了 import 和 type 2中类型，根据 Tok 进行判断。

token.IMPORT
```go-zero
import (
	"path/a.api"
)
```

or

token.TYPE
```go-zero
type (
	User {
	 	Name string `json:"name"`
    }
)
```


## ImportSpec
```go-zero
import (
	"path/a.api"
)
```



## TypeSpec
```go-zero
type (
	User {
	 	Name string `json:"name"`
    }
)
```

## SyntaxSpec
```go-zero
syntax = "v1"
```

## InfoType
```go-zero
info(
    author: "dylan"
    date:   "2020-01-08"
    desc:   "api语法示例及语法说明"
)
```

## Service

其中有2部分，分别为 AtServer & ServiceApi
```go-zero
// service block
@server(
    jwt:   Auth
    group: foo
)
service foo-api{
    @doc "foo"
    @handler foo
    post /foo (Foo) returns (Bar)
}
```