package route

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type registeredHandler struct {
	val string
}

func (r registeredHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func register(lookup *treeLookup, route string) http.Handler {
	handler := registeredHandler{route}
	lookup.Add(route, handler)
	return handler
}

func registerRoutes(lookup *treeLookup, routes []string) map[string]http.Handler {
	handlers := map[string]http.Handler{}
	for _, route := range routes {
		handlers[route] = register(lookup, route)
	}
	return handlers
}

type lookupExpectation struct {
	requestPath     string
	expectedHandler http.Handler
	expectedParams  map[string]string
}

func checkExpectations(t *testing.T, lookup *treeLookup, expectations []lookupExpectation) {
	for _, expectation := range expectations {
		found, pars := lookup.Get(expectation.requestPath)

		assert.Equal(t, expectation.expectedHandler, found)
		assert.Equal(t, expectation.expectedParams, pars)
	}
}

func checkPanics(t *testing.T, testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
		if recv == nil {
			t.Fatal("expected panic")
		}
	}()

	testFunc()
	return
}

func TestLookupRegisterRouteWithoutLeadingSlash(t *testing.T) {
	lookup := newLookup()

	checkPanics(t, func() {
		lookup.Add("file", registeredHandler{""})
	})
}

func TestLookupRegisterRouteWithTrailingSlash(t *testing.T) {
	lookup := newLookup()

	checkPanics(t, func() {
		lookup.Add("/file/", registeredHandler{""})
	})
}

func TestLookupRegisterGreedyParameterWithPathAfter(t *testing.T) {
	lookup := newLookup()

	checkPanics(t, func() {
		lookup.Add("/file/*path/cool", registeredHandler{""})
	})
}

func TestLookupRegisterGreedyParameterWithSameName(t *testing.T) {
	lookup := newLookup()

	lookup.Add("/file/*path", registeredHandler{"yay"})
	checkPanics(t, func() {
		lookup.Add("/file/*path", registeredHandler{""})
	})
}

func TestLookupRegisterNamedParameterWithDifferentNames(t *testing.T) {
	lookup := newLookup()

	lookup.Add("/file/:path", registeredHandler{"yay"})
	checkPanics(t, func() {
		lookup.Add("/file/:notpath", registeredHandler{""})
	})
}

func TestLookupRegisterNamedParameterWithEmptyName(t *testing.T) {
	lookup := newLookup()

	routes := []string{
		"/user/:",
		"/cmd/:/and",
		"/src/*",
	}
	for _, route := range routes {
		checkPanics(t, func() {
			register(lookup, route)
		})
	}
}

func TestLookupExactRoutes(t *testing.T) {
	lookup := newLookup()

	handlers := registerRoutes(lookup, []string{
		"/",
		"/a",
		"/a/b",
		"/a/b/c",
		"/a/d",
	})

	checkExpectations(t, lookup, []lookupExpectation{
		{"/", handlers["/"], map[string]string{}},
		{"/a", handlers["/a"], map[string]string{}},
		{"/a/b", handlers["/a/b"], map[string]string{}},
		{"/a/b/c", handlers["/a/b/c"], map[string]string{}},
		{"/a/d", handlers["/a/d"], map[string]string{}},
		{"/q", nil, map[string]string{}},

		// with trailing slash
		{"/a/", handlers["/a"], map[string]string{}},
		{"/a/b/", handlers["/a/b"], map[string]string{}},
		{"/a/b/c/", handlers["/a/b/c"], map[string]string{}},
		{"/a/d/", handlers["/a/d"], map[string]string{}},
		{"/q/", nil, map[string]string{}},
	})
}

func TestLookupParameter(t *testing.T) {
	lookup := newLookup()

	handlers := registerRoutes(lookup, []string{
		"/user/:name/create",
		"/user/:name",
		"/user/me",
	})

	expectations := []lookupExpectation{
		{"/user/me", handlers["/user/me"], map[string]string{}},                                  // exact match
		{"/user/john", handlers["/user/:name"], map[string]string{"name": "john"}},               // param match
		{"/user/john/create", handlers["/user/:name/create"], map[string]string{"name": "john"}}, // subpath match
		{"/user/me/create", handlers["/user/:name/create"], map[string]string{"name": "me"}},     // match deepest

		// with trailing slash
		{"/user/me/", handlers["/user/me"], map[string]string{}},                                  // exact match
		{"/user/john/", handlers["/user/:name"], map[string]string{"name": "john"}},               // param match
		{"/user/john/create/", handlers["/user/:name/create"], map[string]string{"name": "john"}}, // subpath match
		{"/user/me/create/", handlers["/user/:name/create"], map[string]string{"name": "me"}},     // match deepest
	}

	checkExpectations(t, lookup, expectations)
}

func TestLookupGreedyParameter(t *testing.T) {
	lookup := newLookup()

	handlers := registerRoutes(lookup, []string{
		"/file/this-exact-file",
		"/file/:dir/:file",
		"/file/:dir/*path",
	})

	expectations := []lookupExpectation{
		{"/file/this-exact-file", handlers["/file/this-exact-file"], map[string]string{}},                                 // exact matches
		{"/file/cool/thing.txt", handlers["/file/:dir/:file"], map[string]string{"dir": "cool", "file": "thing.txt"}},     // parameter match
		{"/file/cool/no/this.txt", handlers["/file/:dir/*path"], map[string]string{"dir": "cool", "path": "no/this.txt"}}, // greedy match
		{"/file/img", handlers["/file/:dir/*path"], map[string]string{"dir": "img", "path": ""}},                          // empty greedy match

		// with trailing slash
		{"/file/this-exact-file/", handlers["/file/this-exact-file"], map[string]string{}},                                 // exact matches
		{"/file/cool/thing.txt/", handlers["/file/:dir/:file"], map[string]string{"dir": "cool", "file": "thing.txt"}},     // parameter match
		{"/file/cool/no/this.txt/", handlers["/file/:dir/*path"], map[string]string{"dir": "cool", "path": "no/this.txt"}}, // greedy match
		{"/file/img/", handlers["/file/:dir/*path"], map[string]string{"dir": "img", "path": ""}},                          // empty greedy match
	}

	checkExpectations(t, lookup, expectations)
}

func TestLookupPriorities(t *testing.T) {
	lookup := newLookup()

	handlers := registerRoutes(lookup, []string{"/file/cool.txt", "/file/:name", "/file/*path"})

	expectations := []lookupExpectation{
		{"/file/cool.txt", handlers["/file/cool.txt"], map[string]string{}},
		{"/file/notcool.txt", handlers["/file/:name"], map[string]string{"name": "notcool.txt"}},
		{"/file/notcool.txt/yep", handlers["/file/*path"], map[string]string{"path": "notcool.txt/yep"}},
	}

	checkExpectations(t, lookup, expectations)
}

type route struct {
	method, path string
}

var (
	gplusAPI = []route{
		// People
		{"GET", "/people/:userId"},
		{"GET", "/people"},
		{"GET", "/activities/:activityId/people/:collection"},
		{"GET", "/people/:userId/people/:collection"},
		{"GET", "/people/:userId/openIdConnect"},

		// Activities
		{"GET", "/people/:userId/activities/:collection"},
		{"GET", "/activities/:activityId"},
		{"GET", "/activities"},

		// Comments
		{"GET", "/activities/:activityId/comments"},
		{"GET", "/comments/:commentId"},

		// Moments
		{"POST", "/people/:userId/moments/:collection"},
		{"GET", "/people/:userId/moments/:collection"},
		{"DELETE", "/moments/:id"},
	}

	parseAPI = []route{
		// Objects
		{"POST", "/1/classes/:className"},
		{"GET", "/1/classes/:className/:objectId"},
		{"PUT", "/1/classes/:className/:objectId"},
		{"GET", "/1/classes/:className"},
		{"DELETE", "/1/classes/:className/:objectId"},

		// Users
		{"POST", "/1/users"},
		{"GET", "/1/login"},
		{"GET", "/1/users/:objectId"},
		{"PUT", "/1/users/:objectId"},
		{"GET", "/1/users"},
		{"DELETE", "/1/users/:objectId"},
		{"POST", "/1/requestPasswordReset"},

		// Roles
		{"POST", "/1/roles"},
		{"GET", "/1/roles/:objectId"},
		{"PUT", "/1/roles/:objectId"},
		{"GET", "/1/roles"},
		{"DELETE", "/1/roles/:objectId"},

		// Files
		{"POST", "/1/files/:fileName"},

		// Analytics
		{"POST", "/1/events/:eventName"},

		// Push Notifications
		{"POST", "/1/push"},

		// Installations
		{"POST", "/1/installations"},
		{"GET", "/1/installations/:objectId"},
		{"PUT", "/1/installations/:objectId"},
		{"GET", "/1/installations"},
		{"DELETE", "/1/installations/:objectId"},

		// Cloud Functions
		{"POST", "/1/functions"},
	}
)

func Benchmark_GPlusStatic(b *testing.B) {
	benchRoute(b, gplusAPI, "/people")
}

func Benchmark_GPlusParam(b *testing.B) {
	benchRoute(b, gplusAPI, "/people/118051310819094153327")
}

func Benchmark_GPlus2Params(b *testing.B) {
	benchRoute(b, gplusAPI, "/people/118051310819094153327/activities/123456789")
}

func Benchmark_GPlusAll(b *testing.B) {
	benchRoutes(b, gplusAPI)
}

func Benchmark_ParseStatic(b *testing.B) {
	benchRoute(b, parseAPI, "/1/users")
}

func Benchmark_ParseParam(b *testing.B) {
	benchRoute(b, parseAPI, "/1/classes/go")
}

func Benchmark_Parse2Params(b *testing.B) {
	benchRoute(b, parseAPI, "/1/classes/go/123456789")
}

func Benchmark_ParseAll(b *testing.B) {
	benchRoutes(b, parseAPI)
}

func benchRoute(b *testing.B, routes []route, url string) {
	router := New()

	for _, route := range routes {
		router.Handle(route.path, registeredHandler{route.path})
	}

	r, _ := http.NewRequest("GET", url, nil)
	w := new(mockResponseWriter)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, r)
	}
}

func benchRoutes(b *testing.B, routes []route) {
	router := New()

	for _, route := range routes {
		router.Handle(route.path, registeredHandler{route.path})
	}

	w := new(mockResponseWriter)
	r, _ := http.NewRequest("GET", "/", nil)
	u := r.URL
	rq := u.RawQuery

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, route := range routes {
			r.Method = route.method
			r.RequestURI = route.path
			u.Path = route.path
			u.RawQuery = rq
			router.ServeHTTP(w, r)
		}
	}
}

type mockResponseWriter struct{}

func (m *mockResponseWriter) Header() (h http.Header) {
	return http.Header{}
}

func (m *mockResponseWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (m *mockResponseWriter) WriteString(s string) (n int, err error) {
	return len(s), nil
}

func (m *mockResponseWriter) WriteHeader(int) {}
