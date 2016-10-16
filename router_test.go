package route

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type recordingHandler struct {
	Vars map[string]string
	Used bool
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Used = true
	h.Vars = Vars(r)
}

func TestRouter(t *testing.T) {
	router := New()

	handler := &recordingHandler{}
	router.Handle("/user/:name", handler)

	r, _ := http.NewRequest("GET", "/user/gopher", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.True(t, handler.Used)
	assert.Equal(t, map[string]string{"name": "gopher"}, handler.Vars)
}

func TestRouterRegisterWithHandleFunc(t *testing.T) {
	router := New()
	router.HandleFunc("/HandlerFunc", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(418)
	})

	r, _ := http.NewRequest("GET", "/HandlerFunc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, 418, w.Code)
}

func TestRouterWithEncodedPath(t *testing.T) {
	arg := ""

	router := New()
	router.HandleFunc("/Handle+%2B/:arg", func(w http.ResponseWriter, r *http.Request) {
		arg = Vars(r)["arg"]
		w.WriteHeader(418)
	})

	r, _ := http.NewRequest("GET", "/Handle+%2B/Something+%2B+Something", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, 418, w.Code)
	assert.Equal(t, "Something+%2B+Something", arg)
}

func TestRouterWithOverlappingRoutes(t *testing.T) {
	router := New()

	wildHandler := &recordingHandler{}
	createHandler := &recordingHandler{}

	router.Handle("/user/:name", wildHandler)
	router.Handle("/user/create", createHandler)

	r, _ := http.NewRequest("GET", "/user/gopher", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	r, _ = http.NewRequest("GET", "/user/create", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.True(t, wildHandler.Used)
	assert.Equal(t, map[string]string{"name": "gopher"}, wildHandler.Vars)

	assert.True(t, createHandler.Used)
	assert.Equal(t, map[string]string{}, createHandler.Vars)
}

func TestRouterUncleanPathRedirect(t *testing.T) {
	router := New()

	cases := map[string]string{
		"/../what":                          "/what",
		"/what/..":                          "/",
		"/./what":                           "/what",
		"/what/./":                          "/what/",
		"///what":                           "/what",
		"/what///":                          "/what/",
		"///a///b///c///d///..///.///..///": "/a/b/",
	}

	for reqpath, loc := range cases {
		r, _ := http.NewRequest("GET", reqpath, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, 301, w.Code)
		assert.Equal(t, loc, w.Header().Get("Location"))
	}
}

func TestRouterUncleanPathRedirectDoesNotClearQuery(t *testing.T) {
	router := New()

	r, _ := http.NewRequest("GET", "/test/../?val=5&thing=yeah", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, 301, w.Code)
	assert.Equal(t, "/?val=5&thing=yeah", w.Header().Get("Location"))
}

func TestRouterUncleanPathDoNotRedirectConnectRequests(t *testing.T) {
	router := New()

	r, _ := http.NewRequest("CONNECT", "/../what", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}

func TestRouterNotFound(t *testing.T) {
	router := New()

	r, _ := http.NewRequest("GET", "/nowhere", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}

func TestRouterNotFoundHandlerSet(t *testing.T) {
	router := New()
	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(418)
	})

	r, _ := http.NewRequest("GET", "/nowhere", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, 418, w.Code)
}

// comment the mutex code and run with go test -race to see fail
func TestRouterConcurrentRegisterAndRouting(t *testing.T) {
	router := New()
	var wg sync.WaitGroup
	wg.Add(2)

	handler := &recordingHandler{}

	// register
	go func() {
		router.Handle("/somepath", handler)
		wg.Done()
	}()

	// route
	go func() {
		r, _ := http.NewRequest("GET", "/somepath", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		wg.Done()
	}()

	wg.Wait()
	assert.True(t, handler.Used)
}
