# Available Rules

- [sql-rows-err](#sql-rows-err)
- [http-response-body-close](#http-response-body-close)
- [context-cancel](#context-cancel)

## SQL Rows Err

Checks calls to [database/sql](https://pkg.go.dev/database/sql) and similar packages, that obtain [Rows](https://pkg.go.dev/database/sql#Rows) calls [Rows.Err()](https://pkg.go.dev/database/sql#Rows.Err) as described by

- [sql.Rows.Next](https://pkg.go.dev/database/sql#Rows.Next)
- [sql.Rows.NextResultSet](https://pkg.go.dev/database/sql#Rows.NextResultSet)

The following code is wrong, as it should check [Rows.Err()](https://pkg.go.dev/database/sql#Rows.Err) after [Rows.Next()](https://pkg.go.dev/database/sql#Rows.Next) returns false.

```go
rows, err := db.Query("select id from tb")
if err != nil {
    // Handle error.
}
for rows.Next() {
    // Handle row.
}
// rows.Err() shoud be checked  here!
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

`uncalled` helps uncover such errors which will result in incomplete data if an error is triggered while processing rows.
This can happen when a connection becomes invalid, this causes [Rows.Next()](https://pkg.go.dev/database/sql#Rows.Next) or [sql.Rows.NextResultSet](https://pkg.go.dev/database/sql#Rows.NextResultSet) to return false without processing all rows.

## HTTP Reponse Body Close

Checks for missing [http](https://pkg.go.dev/net/http) `Reponse.Body.Close()` calls.

```go
resp, err := http.Get("http://example.com/")
if err != nil {
    // Handle error.
}
// defer resp.Body.Close() should be called!

body, err := io.ReadAll(resp.Body)
if err != nil {
    // Handle error.
}
```

# Context Cancel

Checks for missing [context](https://pkg.go.dev/context) `CancelFunc()` calls.

```go
ctx, cancel := context.WithCancel(context.Background())
// defer context() check be called!
```
