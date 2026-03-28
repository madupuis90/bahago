package router

import "net/http"

type Router interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request))
}

type MiddlewareRouter struct {
	Router     *http.ServeMux
	Middleware func(h http.Handler) http.Handler
}

func (r *MiddlewareRouter) Handle(pattern string, handler http.Handler) {
	r.Router.Handle(pattern, r.Middleware(handler))
}

func (r *MiddlewareRouter) HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.Router.Handle(pattern, r.Middleware(http.HandlerFunc(handler)))
}

// Chain returns a new MiddlewareRouter that applies additional on top of the
// existing one. Routes registered on the returned router go through both
// middlewares: the parent's first, then additional.
func (r *MiddlewareRouter) Chain(additional func(h http.Handler) http.Handler) *MiddlewareRouter {
	return &MiddlewareRouter{
		Router: r.Router,
		Middleware: func(h http.Handler) http.Handler {
			return r.Middleware(additional(h))
		},
	}
}
