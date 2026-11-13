package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/state"
)

// genSlots bounds concurrent generations: small machines run one model at
// a time, and queued visitors wait their turn.
var genSlots = make(chan struct{}, 1)

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
		sources = s.gatherSources(ctx, q)
	}
	send("sources", nonNil(sources))
	var msgs []ai.Message
	if len(sources) > 0 {
		msgs = ai.BuildPrompt(q, sources)
	} else {
		send("ungrounded", true)
		msgs = []ai.Message{
			{Role: "system", Content: "You are a helpful assistant on an offline computer. Answer briefly and plainly. If you are not sure, say so. For medical, electrical or other safety-critical questions, tell the reader to check an authoritative reference."},
			{Role: "user", Content: q},
		}
	}
	send("status", "Writing…")
	err := s.AI.Chat(ctx, model.Model, msgs, func(tok string) error { return send("token", tok) })
	if err != nil && ctx.Err() == nil {
		s.logf("assistant: %v", err)
		send("error", "The assistant could not answer: "+err.Error())
		return
	}
	send("done", map[string]any{"model": model.Title})
}

// gatherSources searches the library and returns up to four passages.
func (s *Server) gatherSources(ctx context.Context, q string) []ai.Source {
	terms := ai.Terms(q)
	query := strings.Join(terms, " ")
	if query == "" {
		query = q
	}
	sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, _, err := s.Kiwix.Search(sctx, query, 0, 8)
	if err != nil || len(res) == 0 {
		return nil
	}
	var out []ai.Source
	seen := map[string]bool{}
	for _, r := range res {
		if len(out) >= 4 || seen[r.Path] {
			continue
		}
		seen[r.Path] = true
		text, err := s.Kiwix.ArticleText(sctx, r.Path, 60000)
		if err != nil || len(text) < 80 {
			text = r.Snippet
		}
		passage := ai.BestPassages(text, terms, 1800)
		if len(passage) < 60 {
			passage = r.Snippet
		}
		if passage == "" {
			continue
		}
		out = append(out, ai.Source{Label: fmt.Sprintf("S%d", len(out)+1), Title: r.Title, Book: r.Book, Path: r.Path, Text: passage})
	}
	return out
}
