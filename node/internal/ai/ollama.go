// Package ai talks to a local Ollama server and builds grounded prompts that
// make the assistant answer from the node's own library, citing it.
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal Ollama API client.
type Client struct {
	Base string // e.g. http://127.0.0.1:11434
	HTTP *http.Client
}

// New returns a client for base.
func New(base string) *Client {
	return &Client{Base: strings.TrimRight(base, "/"), HTTP: &http.Client{}}
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTP.Do(req)
}

// Version returns Ollama's version, or an error when it is not reachable.
func (c *Client) Version(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := c.do(ctx, http.MethodGet, "/api/version", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var v struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&v); err != nil {
		return "", err
	}
	return v.Version, nil
}

// Model is an installed model.
type Model struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// List returns the installed models.
func (c *Client) List(ctx context.Context) ([]Model, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := c.do(ctx, http.MethodGet, "/api/tags", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var v struct {
		Models []Model `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&v); err != nil {
		return nil, err
	}
	return v.Models, nil
}

// Pull downloads a model, reporting completed and total bytes.
func (c *Client) Pull(ctx context.Context, name string, progress func(done, total int64)) error {
	resp, err := c.do(ctx, http.MethodPost, "/api/pull", map[string]any{"model": name, "stream": true})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ollama pull: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	// Ollama reports progress per layer; sum the layers it has seen.
	layers := map[string][2]int64{}
	success := false
	for sc.Scan() {
		var ev struct {
			Status    string `json:"status"`
			Digest    string `json:"digest"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
			Error     string `json:"error"`
		}
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Error != "" {
			return fmt.Errorf("ollama pull: %s", ev.Error)
		}
		if ev.Digest != "" && ev.Total > 0 {
			layers[ev.Digest] = [2]int64{ev.Completed, ev.Total}
			var d, t int64
			for _, l := range layers {
				d += l[0]
				t += l[1]
			}
			if progress != nil {
				progress(d, t)
			}
		}
		if ev.Status == "success" {
			success = true
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if !success {
		return errors.New("ollama pull ended without success")
	}
	return nil
}

// Delete removes a model. A model that is not installed is not an error.
func (c *Client) Delete(ctx context.Context, name string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/api/delete", map[string]any{"model": name})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("ollama delete: %s", resp.Status)
	}
	return nil
}

// Message is a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chat streams a reply, calling onToken for each piece of text.
func (c *Client) Chat(ctx context.Context, model string, msgs []Message, onToken func(string) error) error {
	return c.chat(ctx, model, msgs, map[string]any{"temperature": 0.2, "num_ctx": 8192}, onToken)
}

func (c *Client) chat(ctx context.Context, model string, msgs []Message, opts map[string]any, onToken func(string) error) error {
	resp, err := c.do(ctx, http.MethodPost, "/api/chat", map[string]any{
		"model": model, "messages": msgs, "stream": true, "think": false,
		"options": opts,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(b))
		// Models without a thinking switch reject "think"; retry without it.
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(msg, "think") {
			return c.chatNoThink(ctx, model, msgs, opts, onToken)
		}
		return fmt.Errorf("ollama chat: %s %s", resp.Status, msg)
	}
	return readChat(resp.Body, onToken)
}

func (c *Client) chatNoThink(ctx context.Context, model string, msgs []Message, opts map[string]any, onToken func(string) error) error {
	resp, err := c.do(ctx, http.MethodPost, "/api/chat", map[string]any{
		"model": model, "messages": msgs, "stream": true,
		"options": opts,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama chat: %s", resp.Status)
	}
	return readChat(resp.Body, onToken)
}

func readChat(r io.Reader, onToken func(string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var ev struct {
			Message Message `json:"message"`
			Done    bool    `json:"done"`
			Error   string  `json:"error"`
		}
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Error != "" {
			return errors.New(ev.Error)
		}
		if ev.Message.Content != "" {
			if err := onToken(ev.Message.Content); err != nil {
				return err
			}
		}
		if ev.Done {
			return nil
		}
	}
	return sc.Err()
}
