package route_test

import (
	"fmt"
	"log"
	"net/http"

	"hawx.me/code/route"
)

func Hello(w http.ResponseWriter, r *http.Request) error {
	vars := route.Vars(r)
	fmt.Fprintf(w, "hello, %s!\n", vars["name"])
	return nil
}

func Example() {
	route.Handle("/", http.RedirectHandler("/hello/anon", http.StatusMovedPermanently))
	route.HandleFunc("/hello/:name", Hello)

	log.Fatal(http.ListenAndServe(":8080", route.Default))
}
