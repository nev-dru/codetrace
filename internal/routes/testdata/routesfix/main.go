package main

type router struct{}

func (r router) Route(p string, fn func(r router)) {}
func (r router) Group(fn func(r router))           {}
func (r router) Get(p string, h func())            {}
func (r router) Post(p string, h func())           {}

type ChatHandler struct{}

func (h *ChatHandler) Handle()  {}
func (h *ChatHandler) History() {}

func ping() {}

// registerAdmin receives the router as a parameter — its mount prefix is
// unknown to a static walker, so registrations here must land in unmapped.
func registerAdmin(r router) {
	r.Get("/organizations", ping)
}

func main() {
	r := router{}
	ch := &ChatHandler{}
	r.Get("/health", func() {}) // func literal → unmapped
	r.Get("/ping", ping)        // plain function handler → bridged
	r.Route("/api", func(r router) {
		r.Post("/chat", ch.Handle)
		r.Group(func(r router) {
			r.Get("/chat/{chatId}/history", ch.History)
		})
	})
	registerAdmin(r)
}
