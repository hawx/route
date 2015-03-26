package route_test

import (
	"fmt"
	"log"
	"net/http"

	"github.com/hawx/route"
)

func Example(w http.ResponseWriter, r *http.Request) {
	vars := route.Vars(r)
	fmt.Fprintf(w, "hello, %s!\n", vars["name"])
}

func main() {
	route.Handle("/", http.RedirectHandler("/hello/anon", http.StatusMovedPermanently))
	route.HandleFunc("/hello/:name", Example)

	log.Fatal(http.ListenAndServe(":8080", route.Default))
}
