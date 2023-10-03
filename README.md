# go-uncalled

[![Reference](https://pkg.go.dev/badge/github.com/stevenh/go-uncalled.svg)](https://pkg.go.dev/github.com/stevenh/go-uncalled) [![License](https://img.shields.io/badge/License-BSD_2--Clause-blue.svg)](https://opensource.org/licenses/BSD-2-Clause) [![Go Report Card](https://goreportcard.com/badge/github.com/stevenh/go-uncalled)](https://goreportcard.com/report/github.com/stevenh/go-uncalled)

go-uncalled is a static analysis tool for golang which checks for missing calls.

It is compatible with both standard and generic functions as introduced by [golang](https://go.dev/) version [1.18](https://go.dev/doc/go1.18).

## Install

You can install the `uncalled` cmd using `go install` command.

```bash
go install github.com/stevenh/go-uncalled/cmd/uncalled@latest
```

## How to use

You run `uncalled` with [go vet](https://pkg.go.dev/cmd/vet).

```bash
go vet -vettool=$(which uncalled) ./...
# github.com/stevenh/go-uncalled/test
test/bad.go:10:2: rows.Err() must be called
```

Or run it directly.

```bash
uncalled ./...
# github.com/stevenh/go-uncalled/test
test/bad.go:10:2: rows.Err() must be called
```

See [command line](#command-line) options for more details.

## Analyzer

`uncalled` validates that code to ensure expected calls are made.

## Command line

`uncalled` supports the following command line options

- `-config <file>` - configures the [YAML](https://yaml.org/) file to read the configuration from. (default: [embedded .uncalled.yaml](pkg/uncalled/.uncalled.yaml)).
- `-version` - prints `uncalled` version information and exits.
- `-verbose [level]` - configures `uncalled` logging level, without a level it increments, with a level it sets (default: `info`)

## Rule Configuration

Each rule is defined by the following common configuration.

- name: `string` name of this rule.
- disabled: `bool` disable this rule.
- category: `string` category to log failures with.
- packages: `[]string` list of package import paths that if present will trigger this rule to be processed.
- results: `[]object` list of results that methods return that if matched will trigger this rule to be processed.
  - type: `string` name of the type relative to the package.
  - pointer: `bool` if true this type is a pointer type.
  - expect: `object` the details to expect when performing checks.
    - call: `string` the method that should be called on the returned type, blank if this is a direct function call.
    - args: `[]string` the list of arguments that the call takes.

Example

```yaml
rules:
  # Checks for missing sql Rows.Err() calls.
  - name: sql-rows-err
    disabled: false
    category: sql
    packages:
      - database/sql
      - github.com/jmoiron/sqlx
    results:
      - type: .Rows
        pointer: true
        expect:
          call: .Err
          args: []
      - type: error
        pointer: false
```

You can find more info in the [available rules](RULES.md#available-rules).

## Inspired by

This code was inspired by the following analysers:

- [jingyugao rowserrcheck](https://github.com/jingyugao/rowserrcheck)
- [x/tools httpresponse](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/httpresponse)
