package pages

import (
	"fmt"
	"net/http"
	"sync"

	. "bahago/internal/ui/layouts"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

type TestHandler struct{}

type Hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

var hub = &Hub{
	subs: make(map[chan string]struct{}),
}

func (h *Hub) Subscribe() (chan string, func()) {
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

func (h *Hub) Publish(msg string) {
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

func Test() Node {
	return Layout(
		LayoutArgs{
			Title: "Test Page",
		},
		H1(Text("Test!")),
		Div(ID(containerId), ds.Init(datastar.GetSSE("/read")),
			Div(ID("messages"), Text("Waiting for messages...")),
		),
		Map([]string{"1", "2", "3"}, func(i string) Node {
			return P(Text(i))
		}),
		Form(
			ds.On("submit", datastar.PostSSE("/write")),
			Input(ds.Bind("message")),
			Button(Text("Send")),
		),
	)
}

type Store struct {
	Message string `json:"message"`
}

func Read(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	messages, cleanup := hub.Subscribe()
	defer cleanup()

	for {
		select {
		case <-r.Context().Done():
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

func Write(w http.ResponseWriter, r *http.Request) {
	store := &Store{}
	if err := datastar.ReadSignals(r, store); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hub.Publish(store.Message)

	// reset message
	sse := datastar.NewSSE(w, r)
	sse.MarshalAndPatchSignals(map[string]any{"message": ""})

}
