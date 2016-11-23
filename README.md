trie-mux
====
A minimal tree based url path router (or mux) for Go.

[![Build Status](http://img.shields.io/travis/teambition/trie-mux.svg?style=flat-square)](https://travis-ci.org/teambition/trie-mux)
[![Coverage Status](http://img.shields.io/coveralls/teambition/trie-mux.svg?style=flat-square)](https://coveralls.io/r/teambition/trie-mux)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/teambition/trie-mux/master/LICENSE)
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/teambition/trie-mux)

## JavaScript Version

https://github.com/zensh/route-trie

## Features:

1. Support regexp (Trie)
2. Fixed path automatic redirection (Trie)
3. Trailing slash automatic redirection (Trie)
4. Automatic handle `405 Method Not Allowed` (Mux)
5. Automatic handle `501 Not Implemented` (Mux)
6. Automatic handle `OPTIONS` method (Mux)
7. Best Performance

## Implementations

### Gear: gear.Router

https://github.com/teambition/gear/blob/master/router.go

```go
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/teambition/gear"
)

func main() {
	app := gear.New()

	router := gear.NewRouter()
	router.Get("/", func(ctx *gear.Context) error {
		return ctx.HTML(200, "<h1>Hello, Gear!</h1>")
	})
	router.Get("/view/:view", func(ctx *gear.Context) error {
		view := ctx.Param("view")
		if view == "" {
			return &gear.Error{Code: 400, Msg: "Invalid view"}
		}
		return ctx.HTML(200, "View: "+view)
	})

	app.UseHandler(router)
	srv := app.Start(":3000")
	defer srv.Close()

	res, _ := http.Get("http://" + srv.Addr().String() + "/view/users")
	body, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()

	fmt.Println(res.StatusCode, string(body))
	// Output: 200 View: users
}
```

### trie-mux: mux.Mux

https://github.com/teambition/trie-mux/blob/master/mux/mux.go

```go
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/teambition/trie-mux/mux"
)

func main() {
	router := mux.New()
	router.Get("/", func(w http.ResponseWriter, _ *http.Request, _ mux.Params) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("<h1>Hello, Gear!</h1>"))
	})

	router.Get("/view/:view", func(w http.ResponseWriter, _ *http.Request, params mux.Params) {
		view := params["view"]
		if view == "" {
			http.Error(w, "Invalid view", 400)
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(200)
			w.Write([]byte("View: " + view))
		}
	})

	// srv := http.Server{Addr: ":3000", Handler: router}
	// srv.ListenAndServe()
	srv := httptest.NewServer(router)
	defer srv.Close()

	res, _ := http.Get(srv.URL + "/view/users")
	body, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()

	fmt.Println(res.StatusCode, string(body))
	// Output: 200 View: users
}
```

## Pattern Rule

The defined pattern can contain three types of parameters:

| Syntax | Description |
|--------|------|
| `:name` | named parameter |
| `:name*` | named with catch-all parameter |
| `:name(regexp)` | named with regexp parameter |


Named parameters are dynamic path segments. They match anything until the next '/' or the path end:

Defined: `/api/:type/:ID`
```
/api/user/123             match: type="user", ID="123"
/api/user                 no match
/api/user/123/comments    no match
```

Named with catch-all parameters match anything until the path end, including the directory index (the '/' before the catch-all). Since they match anything until the end, catch-all parameters must always be the final path element.

Defined: `/files/:filepath*`
```
/files                           no match
/files/LICENSE                   match: filepath="LICENSE"
/files/templates/article.html    match: filepath="templates/article.html"
```

Named with regexp parameters match anything using regexp until the next '/' or the path end:

Defined: `/api/:type/:ID(^\\d+$)`
```
/api/user/123             match: type="user", ID="123"
/api/user                 no match
/api/user/abc             no match
/api/user/123/comments    no match
```

The value of parameters is saved on the `Matched.Params`. Retrieve the value of a parameter by name:
```
type := matched.Params("type")
id   := matched.Params("ID")
```

## Documentation

https://godoc.org/github.com/teambition/trie-mux

## Bench

```bash
go test -bench=. ./bench
```

```
GithubAPI Routes: 203
   trie-mux: 85408 Bytes
   HttpRouter: 37464 Bytes
testing: warning: no tests to run
BenchmarkTrieMux-4      	    2000	    809379 ns/op	 1095099 B/op	    3177 allocs/op
BenchmarkHttpRouter-4   	    2000	    670166 ns/op	 1030812 B/op	    2604 allocs/op
PASS
ok  	github.com/teambition/trie-mux/bench	3.163s
```

## License
trie-mux is licensed under the [MIT](https://github.com/teambition/trie-mux/blob/master/LICENSE) license.
Copyright &copy; 2016 [Teambition](https://www.teambition.com).