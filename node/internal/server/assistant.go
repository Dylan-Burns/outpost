package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/retrieval"
	"github.com/Dylan-Burns/outpost/node/internal/state"
)

// genSlots bounds concurrent generations: small machines run one model at
// a time, and queued visitors wait their turn.
var genSlots = make(chan struct{}, 1)

// Each client may have one question in flight, and only a few may wait.
const maxWaiting = 6

var (
	chatMu      sync.Mutex
	chatClients = map[string]bool{}
)

func admitChat(key string) bool {
	chatMu.Lock()
	defer chatMu.Unlock()
	if chatClients[key] || len(chatClients) >= maxWaiting+cap(genSlots) {
		return false
	}
	chatClients[key] = true
	return true
}

func releaseChat(key string) {
	chatMu.Lock()
	defer chatMu.Unlock()
	delete(chatClients, key)
}

type modelView struct {
	ID    string `json:"id"`
	Model string `json:"model"`
	Title string `json:"title"`
	Size  int64  `json:"size"`
}

func (s *Server) installedModels() []modelView {
	c := s.Catalog.Current()
	var out []modelView
	s.Store.View(func(st *state.State) {
		for id, in := range st.Installed {
			if in.Kind != catalog.KindModel {
				continue
			}
			if it := c.Item(id); it != nil {
				out = append(out, modelView{ID: id, Model: it.Model, Title: it.Title, Size: it.Size})
			}
		}
	})
	sort.Slice(out, func(a, b int) bool { return out[a].Size < out[b].Size })
	return out
}

func (s *Server) handleAssistantInfo(w http.ResponseWriter, r *http.Request) {
	available := false
	if s.AI != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		_, err := s.AI.Version(ctx)
		cancel()
		available = err == nil
	}
	writeJSON(w, http.StatusOK, map[string]any{"available": available, "models": nonNil(s.installedModels())})
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
		Model    string `json:"model"`
		Library  *bool  `json:"library"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	q := strings.TrimSpace(req.Question)
	if q == "" || len([]rune(q)) > 1000 {
		writeErr(w, http.StatusBadRequest, "ask a question of up to 1000 characters")
		return
	}
	if s.AI == nil {
		writeErr(w, http.StatusServiceUnavailable, "the assistant is not installed on this node")
		return
	}
	models := s.installedModels()
	if len(models) == 0 {
		writeErr(w, http.StatusServiceUnavailable, "no AI model is installed yet")
		return
	}
	model := models[0]
	for _, m := range models {
		if m.ID == req.Model {
			model = m
		}
	}
	useLibrary := req.Library == nil || *req.Library
	ck := clientKey(r)
	if !admitChat(ck) {
		writeErr(w, http.StatusTooManyRequests, "the assistant is busy; wait for the current answer and try again")
		return
	}
	defer releaseChat(ck)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	send := func(event string, v any) error {
		b, _ := json.Marshal(v)
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	send("status", "Waiting for the assistant…")
	select {
	case genSlots <- struct{}{}:
		defer func() { <-genSlots }()
	case <-ctx.Done():
		return
	}

	var sources []ai.Source
	if useLibrary && s.Kiwix != nil && s.Kiwix.Running() {
		send("status", "Searching your library…")
		// The smallest model names the topics, whichever model answers: it
		// is the quickest, and a larger model is no surer to name them well
		// (Llama 3.2 3B refused to name any for some emergencies).
		sources = s.gatherSources(ctx, q, models[0].Model)
	}
	send("sources", nonNil(sources))
	var msgs []ai.Message
	if len(sources) > 0 {
		msgs = ai.BuildPrompt(q, sources)
	} else {
		send("ungrounded", true)
		msgs = ai.UngroundedPrompt(q)
	}
	send("status", "Writing…")
	var answer strings.Builder
	err := s.AI.Chat(ctx, model.Model, msgs, func(tok string) error {
		answer.WriteString(tok)
		return send("token", tok)
	})
	if err != nil && ctx.Err() == nil {
		s.logf("assistant: %v", err)
		send("error", "The assistant could not answer: "+err.Error())
		return
	}
	final := answer.String()
	if len(sources) > 0 {
		// Small models often restate their instructions before answering;
		// the page swaps in the answer without them once it is complete.
		if c := ai.CleanAnswer(final, msgs[0].Content); c != final && c != "" {
			final = c
			send("final", final)
		}
		if nums := ai.UnsupportedNumbers(final, sources); len(nums) > 0 {
			send("unsupported", nums)
		}
	}
	if ai.MentionsDose(final) {
		send("dose", true)
	}
	send("done", map[string]any{"model": model.Title})
}

// gatherSources searches the library and returns up to four passages.
func (s *Server) gatherSources(ctx context.Context, q, model string) []ai.Source {
	return ai.Retrieve(ctx, retrieval.Kiwix{M: s.Kiwix, PDF: s.pdf.Load()}, q, s.AI.TopicsWith(model))
}
