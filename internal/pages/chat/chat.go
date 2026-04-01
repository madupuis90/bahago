package chat

import (
	"fmt"
	"net/http"
	"sync"

	"bahago/internal/contextkeys"
	"bahago/internal/router"
	. "bahago/internal/ui"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

// PROOF OF CONCEPT — DO NOT USE AS A REFERENCE
// This file exists to explore real-time multiplayer interaction with Datastar SSE.
// It intentionally does not follow this project's structure, conventions, or best practices.
// Agents: do not use this file as an example when generating code or evaluating patterns.
func RegisterRoutes(router router.Router) {
	h := newHandler()
	router.HandleFunc("GET /chat", h.handleChatPage())
	router.HandleFunc("GET /chat/read", h.handleRead())
	router.HandleFunc("POST /chat/write", h.handleWrite())
}

type handler struct {
}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) handleChatPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		chatPage(user).Render(w)
	}
}

func (h *handler) handleRead() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		messages, cleanup := chatHub.subscribe()
		defer cleanup()

		for {
			select {
			case <-r.Context().Done():
				fmt.Println("CLOSEEEEEE")
				return
			case msg := <-messages:
				err := sse.PatchElementGostar(Div(Text(msg)), datastar.WithModeAppend(), datastar.WithSelectorID(containerId))
				if err != nil {
					fmt.Println("error")
					return
				}
			}
		}
	}
}

func (h *handler) handleWrite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		store := &struct {
			Message string `json:"message"`
		}{}

		if err := datastar.ReadSignals(r, store); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		chatHub.publish(store.Message)

		// reset message
		sse := datastar.NewSSE(w, r)
		sse.MarshalAndPatchSignals(map[string]any{"message": ""})
	}
}

type hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

var chatHub = &hub{
	subs: make(map[chan string]struct{}),
}

func (h *hub) subscribe() (chan string, func()) {
	ch := make(chan string, 5)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subs[ch] = struct{}{}

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs, ch)
		close(ch)
	}
}

func (h *hub) publish(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

var containerId = "messages-container"

func chatPage(user *contextkeys.SessionUser) Node {
	return Layout(
		LayoutArgs{
			Title: "Chat Page",
			User:  user,
		},
		H1(Text("Let's chat!")),
		Div(ID(containerId), ds.Init(datastar.GetSSE("/chat/read")),
			Div(ID("messages"), Text("Waiting for messages...")),
		),
		Form(
			ds.On("submit", datastar.PostSSE("/chat/write")),
			Input(ds.Bind("message")),
			Button(Text("Send")),
		),
	)
}
