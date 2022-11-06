# go-rowserr

[![Reference](https://pkg.go.dev/badge/github.com/stevenh/go-rowserr.svg)](https://pkg.go.dev/github.com/stevenh/go-rowserr) [![License](https://img.shields.io/badge/License-BSD_2--Clause-blue.svg)](https://opensource.org/licenses/BSD-2-Clause) [![Go Report Card](https://goreportcard.com/badge/github.com/stevenh/go-rowserr)](https://goreportcard.com/report/github.com/stevenh/go-rowserr)

go-rowserr is a static analysis tool for golang which checks for missing [database/sql.Rows.Err()](https://pkg.go.dev/database/sql#Rows.Err) calls.

It is compatible with both standard and generic functions as introduced by [golang](https://go.dev/) version [1.18](https://go.dev/doc/go1.18).

## Install

You can install `rowserr` cmd using `go install` command.

```bash
go install github.com/stevenh/go-rowserr/cmd/rowserr@latest
```

## How to use

You run `rowserr` with [go vet](https://pkg.go.dev/cmd/vet).

```bash
go vet -vettool=$(which rowserr) ./...
# github.com/stevenh/go-rowserr/test
test/bad.go:10:2: rows.Err() must be checked
```

## Analyzer

`rowserr` validates that code which uses [database/sql](https://pkg.go.dev/database/sql) to obtain [sql.Rows](https://pkg.go.dev/database/sql#Rows) calls [Rows.Err()](https://pkg.go.dev/database/sql#Rows.Err) as described by

- [sql.Rows.Next](https://pkg.go.dev/database/sql#Rows.Next)
- [sql.Rows.NextResultSet](https://pkg.go.dev/database/sql#Rows.NextResultSet)

The following code above is wrong, as it should check [Rows.Err()](https://pkg.go.dev/database/sql#Rows.Err) after [Rows.Next()](https://pkg.go.dev/database/sql#Rows.Next) returns false.

```go
rows, err := db.Query("select id from tb")
if err != nil {
    // Handle error.
}
for rows.Next() {
    // Handle row.
}
// rows.Err() check should be here!
```

This is how this code should be written.

```go
rows, err := db.Query("select id from tb")
if err != nil {
    // Handle error.
}
for rows.Next() {
    // Handle row.
}
if err = rows.Err(); err != nil {
    // Handle error.
}
```

`rowserr` helps uncover such errors which will result in incomplete data if an error is triggered while processing rows.
This can happen when a connection becomes invalid, this causes [Rows.Next()](https://pkg.go.dev/database/sql#Rows.Next) or [sql.Rows.NextResultSet](https://pkg.go.dev/database/sql#Rows.NextResultSet) to return false without processing all rows.

## Inspired by

This code was inspired by the following analysers:

- [jingyugao rowserrcheck](https://github.com/jingyugao/rowserrcheck)
- [x/tools httpresponse](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/httpresponse)
