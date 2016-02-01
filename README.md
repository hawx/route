# route [![docs](http://godoc.org/hawx.me/code/route?status.svg)](http://godoc.org/hawx.me/code/route)

A HTTP request router.

- Uses standard `http.Handler`s instead of a custom type.
- Parameters, both single path segment "named" parameters and greedy
  "catch-all" parameters.
- Allows overlapping route registrations, that is both `/user/create` and
  `/user/:name` may be registered.
- Corrects trailing slashes and redirects paths with superfluous elements
  (e.g. `../`, `/./` and `//`).
- A custom Not Found handler can be assigned.

## parameters

A *named parameter* has the form `:name` where "name" is the key used to
retrieve it from the map. Named parameters only match a single path segment:

```
Path: /blog/:category/:post

Requests:
 /blog/go/request-routers            match: category="go", post="request-routers"
 /blog/go/request-routers/           redirect to /blog/go/request-routers
 /blog/go/                           no match
 /blog/go/request-routers/comments   no match
```

A *catch-all parameter* has the form `*name` where "name" is the key used to
retrieve it from the map. Catch-all parameters match everything including the
preceeding "/", so must always be at the end of the pattern.

```
Path: /files/*filepath

Requests:
 /files/                             match: filepath=""
 /files/LICENSE                      match: filepath="LICENSE"
 /files/templates/article.html       match: filepath="templates/article.html"
 /files                              match: filepath=""
```

Parameters can be retrieved in handlers by calling `route.Vars(*http.Request)
map[string]string` with the current request:

``` golang
import (
  "fmt"
  "net/http"

  "hawx.me/code/route"
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
