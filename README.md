# route [![docs](http://godoc.org/github.com/hawx/route?status.png)](http://godoc.org/github.com/hawx/route)

This is a fork of [github.com/julienschmidt/httprouter][httprouter], with the
following aims/changes:

- Use standard `http.Handler` types instead of a custom `Handle` type. This
  meant using [github.com/gorilla/mux][gorilla/mux] style match retrieval. That
  is, instead of passing the parameters into the handler they are retrieved
  using a helper function, `route.Vars` here, from the `http.Request`.

- Remove HTTP method matching.

- Use `http.ServeMux` method names, so we have `route.Handle(http.Handler)` and
  `route.HandleFunc(http.HandlerFunc)` as the only two ways of registering
  handlers.

- Use a `map[string]string` for parameter matches instead of a custom `Params`
  type. This means we loose the strict ordering, but that is rarely required.

- Allow registering to a "default" router, as in the `net/http` package.

- Remove static file server, just use `http.FileServer`.

## features

- **Trailing slash correction:** Auto redirects the client if a trailing slash
  is missing, or one is given but not required. Of course it only does so, if
  the new path has a handler.

- **Path auto-correction:** Removes superfluous elements (like `../` or `//`). Also makes case-insensitive lookups.

- **Parameters:** Define routes with parameters in.

- **Catch panics:** Assign a `http.Handler` to `route.PanicHandler` to deal with
  panics during request handling.

- **Not Found handler:** Assign a `http.Handler` to `route.NotFoundHandler` to
  perform custom actions when not found, otherwise uses `http.NotFound`.

## parameters

A *named parameter* has the form `:name` where "name" is the key used to
retrieve it from the map. Named parameters only match a single path segment:

```
Pattern: /user/:user

 /user/gordon              match (user=gordon)
 /user/you                 match (user=you)
 /user/gordon/profile      no match
 /user/                    no match
```

**Note:** Since this router has only explicit matches, you can not register
static routes and parameters for the same path segment. For example you can not
register the patterns `/user/new` and `/user/:user` at the same time.

A *catch-all parameter* has the form `*name` where "name" is the key used to
retrieve it from the map. Catch-all parameters match everything including the
preceeding "/", so must always be at the end of the pattern.

```
Pattern: /src/*filepath

 /src/                     match (filepath="/")
 /src/somefile.go          match (filepath="/somefile.go")
 /src/subdir/somefile.go   match (filepath="/subdir/somefile.go")
```

Parameters can be retrieved in handlers by calling `route.Vars(*http.Request)
map[string]string` with the current request:

``` golang
import (
  "fmt"
  "net/http"

  "github.com/hawx/route"
)

func greetingHandler(w http.ResponseWriter, r *http.Request)) {
  vars := route.Vars(r)
  fmt.Fprintf(w, "Hi %s", vars["name"])
}

func main() {
  route.HandleFunc("/greet/:name", greetingHandler)
  http.ListenAndServe(":8080", route.Default)
}
```


[httprouter]: https://github.com/julienschmidt/httprouter
[gorilla/mux]: https://github.com/gorilla/mux
