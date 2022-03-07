// Package route is a HTTP request router.
//
// The router matches incoming requests by path to registered handlers. It
// should feel familiar to users of the net/http package.
//
// The registered path may contain parameters, of which there are two types.
//
// Named
//
// Named parameters match single path segments. They match until the next '/' or
// the path end:
//
//  Path: /blog/:category/:post
//
//  Requests:
//   /blog/go/request-routers            match: category="go", post="request-routers"
//   /blog/go/request-routers/           redirect to /blog/go/request-routers
//   /blog/go/                           no match
//   /blog/go/request-routers/comments   no match
//
// Catch-all
//
// Catch-all parameters match anything until the path end. Since they match
// anything until the end, catch-all paramerters must always be the final path
// element.
//
//  Path: /files/*filepath
//
//  Requests:
//   /files/                             match: filepath=""
//   /files/LICENSE                      match: filepath="LICENSE"
//   /files/templates/article.html       match: filepath="templates/article.html"
//   /files                              match: filepath=""
//
// The value of parameters is saved as a map[string]string against the
// request. To retrieve the parameters for a request use the Vars function:
//
//   vars := route.Vars(r)
//
package route

import (
	"context"
	"net/http"
	"sync"
)

type Handler interface {
	http.Handler
	ServeErrorHTTP(w http.ResponseWriter, r *http.Request) error
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		panic(err)
	}
}

func (h HandlerFunc) ServeErrorHTTP(w http.ResponseWriter, r *http.Request) error {
	return h(w, r)
}

type nilErrorHandler struct {
	http.Handler
}

func (h nilErrorHandler) ServeErrorHTTP(w http.ResponseWriter, r *http.Request) error {
	h.ServeHTTP(w, r)
	return nil
}

// Router is a http.Handler which can be used to dispatch requests to different
// handler functions via configurable routes
type Router struct {
	// NotFoundHandler is called when no matching route is found. By default it is
	// set to http.NotFoundHandler().
	NotFoundHandler http.Handler

	// ErrorHandler is called if an error is raised by any handler.
	ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

	mu   sync.RWMutex
	tree *treeLookup
}

// Default is the router instance used by the Handle and HandleFunc functions.
var Default = New()

// Handle registers the handler for the given path to the Default router.
func Handle(path string, handler http.Handler) {
	Default.Handle(path, handler)
}

// HandleFunc registers the handler function for the given path to the Default
// router.
func HandleFunc(path string, handler HandlerFunc) {
	Default.HandleFunc(path, handler)
}

// Make sure the Router conforms with the http.Handler interface
var _ http.Handler = New()

// New returns an initialized Router.
func New() *Router {
	return &Router{
		NotFoundHandler: http.NotFoundHandler(),
		ErrorHandler:    func(w http.ResponseWriter, r *http.Request, err error) {},
		tree:            newLookup(),
	}
}

// Handle registers the handler for the given path to the router.
func (r *Router) Handle(path string, handle http.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if path[0] != '/' {
		panic("path must begin with '/'")
	}

	switch v := handle.(type) {
	case Handler:
		r.tree.Add(path, v)
	case http.Handler:
		r.tree.Add(path, nilErrorHandler{v})
	}
}

// HandleFunc registers the handler function (either `func(http.ResponseWriter,
// *http.Request)` or `func(http.ResponseWriter, *http.Request) error`) for the
// given path to the Default router.
func (r *Router) HandleFunc(path string, handler interface{}) {
	switch v := handler.(type) {
	case func(http.ResponseWriter, *http.Request) error:
		r.Handle(path, HandlerFunc(v))
	case func(http.ResponseWriter, *http.Request):
		r.Handle(path, http.HandlerFunc(v))
	default:
		panic("tried to register unhandleable func type with HandleFunc")
	}
}

// ServeHTTP dispatches the request to appropriate handler, if none can be found
// NotFoundHandler is used.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.EscapedPath()

	if req.Method != "CONNECT" {
		if cleanpath := cleanPath(path); cleanpath != path {
			url := *req.URL
			url.Path = cleanpath
			http.RedirectHandler(url.String(), http.StatusMovedPermanently).ServeHTTP(w, req)
			return
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if handle, ps := r.tree.Get(path); handle != nil {
		req = req.WithContext(context.WithValue(req.Context(), varsKey{}, ps))
		err := handle.ServeErrorHTTP(w, req)
		if err != nil {
			r.ErrorHandler(w, req, err)
		}
		return
	}

	r.NotFoundHandler.ServeHTTP(w, req)
}

type varsKey struct{}

// Vars retrieves the parameter matches for the given request.
func Vars(r *http.Request) map[string]string {
	if rv := r.Context().Value(varsKey{}); rv != nil {
		return rv.(map[string]string)
	}

	return nil
}
