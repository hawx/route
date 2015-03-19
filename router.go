// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Package route is a trie based high performance HTTP request router.
//
// A trivial example is:
//
//  package main
//
//  import (
//      "fmt"
//      "github.com/hawx/route"
//      "net/http"
//      "log"
//  )
//
//  func Index(w http.ResponseWriter, r *http.Request) {
//      fmt.Fprint(w, "Welcome!\n")
//  }
//
//  func Hello(w http.ResponseWriter, r *http.Request) {
//      vars := route.Vars(r)
//      fmt.Fprintf(w, "hello, %s!\n", vars["name"])
//  }
//
//  func main() {
//      route.HandleFunc("/", Index)
//      route.HandleFunc("/hello/:name", Hello)
//
//      log.Fatal(http.ListenAndServe(":8080", route.Default))
//  }
//
// The router matches incoming requests by the path. If a handle is registered
// for this path the router delegates the request to that function.
//
// The registered path, against which the router matches incoming requests, can
// contain two types of parameters:
//
//  Syntax    Type
//  :name     named parameter
//  *name     catch-all parameter
//
// Named parameters are dynamic path segments. They match anything until the
// next '/' or the path end:
//
//  Path: /blog/:category/:post
//
//  Requests:
//   /blog/go/request-routers            match: category="go", post="request-routers"
//   /blog/go/request-routers/           no match, but the router would redirect
//   /blog/go/                           no match
//   /blog/go/request-routers/comments   no match
//
// Catch-all parameters match anything until the path end, including the
// directory index (the '/' before the catch-all). Since they match anything
// until the end, catch-all paramerters must always be the final path element.
//
//  Path: /files/*filepath
//
//  Requests:
//   /files/                             match: filepath="/"
//   /files/LICENSE                      match: filepath="/LICENSE"
//   /files/templates/article.html       match: filepath="/templates/article.html"
//   /files                              no match, but the router would redirect
//
// The value of parameters is saved as a map[string]string. To retrieve the
// parameters for a request use the route.Vars function:
//
//   vars := route.Vars(r)
package route

import (
	"net/http"

	"github.com/gorilla/context"
)

// Router is a http.Handler which can be used to dispatch requests to different
// handler functions via configurable routes
type Router struct {
	tree *node

	// Configurable http.Handler which is called when no matching route is
	// found. If it is not set, http.NotFound is used.
	NotFound http.Handler

	// Function to handle panics recovered from http handlers. It should be used
	// to generate a error page and return the http error code 500 (Internal
	// Server Error). The handler can be used to keep your server from crashing
	// because of unrecovered panics.
	PanicHandler func(http.ResponseWriter, *http.Request, interface{})
}

// Default is the router instance used by the Handle and HandleFunc functions.
var Default = New()

// Handle registers the handler for the given path to the Default router.
func Handle(path string, handler http.Handler) {
	Default.Handle(path, handler)
}

// HandleFunc registers the handler function for the given path to the Default
// router.
func HandleFunc(path string, handler http.HandlerFunc) {
	Default.HandleFunc(path, handler)
}

// Make sure the Router conforms with the http.Handler interface
var _ http.Handler = New()

// New returns a new initialized Router.
func New() *Router {
	return &Router{}
}

// Handler registers a new http.Handler with the given path.
func (r *Router) Handle(path string, handle http.Handler) {
	if path[0] != '/' {
		panic("path must begin with '/'")
	}

	if r.tree == nil {
		r.tree = new(node)
	}

	root := r.tree
	if root == nil {
		root = new(node)
		r.tree = root
	}

	root.addRoute(path, handle)
}

// HandleFunc is an adapter which allows the usage of an http.HandlerFunc as a
// handler.
func (r *Router) HandleFunc(path string, handler http.HandlerFunc) {
	r.Handle(path, handler)
}

func (r *Router) recv(w http.ResponseWriter, req *http.Request) {
	if rcv := recover(); rcv != nil {
		r.PanicHandler(w, req, rcv)
	}
}

// ServeHTTP makes the Router implement the http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.PanicHandler != nil {
		defer r.recv(w, req)
	}

	if root := r.tree; root != nil {
		path := req.URL.Path

		if handle, ps, tsr := root.getValue(path); handle != nil {
			setVars(req, ps)
			defer context.Clear(req)
			handle.ServeHTTP(w, req)
			return
		} else if req.Method != "CONNECT" && path != "/" {
			code := 301 // Permanent redirect, request with GET method
			if req.Method != "GET" {
				// Temporary redirect, request with same method
				// As of Go 1.3, Go does not support status code 308.
				code = 307
			}

			if tsr {
				if len(path) > 1 && path[len(path)-1] == '/' {
					req.URL.Path = path[:len(path)-1]
				} else {
					req.URL.Path = path + "/"
				}
				http.Redirect(w, req, req.URL.String(), code)
				return
			}

			// Try to fix the request path
			fixedPath, found := root.findCaseInsensitivePath(
				cleanPath(path),
				true,
			)
			if found {
				req.URL.Path = string(fixedPath)
				http.Redirect(w, req, req.URL.String(), code)
				return
			}
		}
	}

	// Handle 404
	if r.NotFound != nil {
		r.NotFound(w, req)
	} else {
		http.NotFound(w, req)
	}
}

const varsKey = "__github.com/hawx/route:Vars__"

// Vars retrieves the parameter matches for the given request.
func Vars(r *http.Request) map[string]string {
	if rv := context.Get(r, varsKey); rv != nil {
		return rv.(map[string]string)
	}
	return nil
}

func setVars(r *http.Request, val interface{}) {
	context.Set(r, varsKey, val)
}
