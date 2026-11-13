// Package state keeps the node's durable state in one JSON file, written
// atomically. Everything the node must remember across restarts lives here:
// the admin password hash, sessions, settings, the owner's selection, what
// is installed, and the job queue.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/fsutil"
)

// Settings the owner can change.
type Settings struct {
	NodeName string `json:"nodeName"`
	// PublicAccess lets anyone on the network browse the library, maps and
	// search without signing in. Content management always needs the admin.
	PublicAccess bool `json:"publicAccess"`
	// AssistantPublic lets visitors use the AI assistant without signing in.
	AssistantPublic bool `json:"assistantPublic"`
	// CheckForUpdates allows the node to fetch the signed catalog and the
	// latest version number from the Outpost website when it is online.
	CheckForUpdates bool `json:"checkForUpdates"`
}

// Apps the owner enabled in the wizard.
type Apps struct {
	Library   bool `json:"library"`
	Maps      bool `json:"maps"`
	Assistant bool `json:"assistant"`
}

// Session is a signed-in browser. The map key is the SHA-256 of the cookie
// value, so a leaked state file does not leak live session tokens.
type Session struct {
	CSRF    string    `json:"csrf"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

// Installed records one installed item.
type Installed struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	File        string    `json:"file,omitempty"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256,omitempty"`
	Date        string    `json:"date,omitempty"`
	InstalledAt time.Time `json:"installedAt"`
}

// Job states.
const (
	JobQueued  = "queued"
	JobWaiting = "waiting" // failed transiently; queued again after a delay
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
	JobCancel  = "cancelled"
)

// Pending reports whether a job state still has work to do.
func Pending(st string) bool { return st == JobQueued || st == JobRunning || st == JobWaiting }

// Job is one install or removal of an item.
type Job struct {
	ID       string    `json:"id"`
	ItemID   string    `json:"itemId"`
	Action   string    `json:"action"` // "install" or "remove"
	State    string    `json:"state"`
	Error    string    `json:"error,omitempty"`
	Attempts int       `json:"attempts"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
}

// State is the whole document.
type State struct {
	Version         int                  `json:"version"`
	SetupComplete   bool                 `json:"setupComplete"`
	AdminHash       string               `json:"adminHash,omitempty"`
	Sessions        map[string]Session   `json:"sessions"`
	Settings        Settings             `json:"settings"`
	Apps            Apps                 `json:"apps"`
	Selection       catalog.Selection    `json:"selection"`
	Installed       map[string]Installed `json:"installed"`
	Jobs            []Job                `json:"jobs"`
	CatalogSequence int64                `json:"catalogSequence"`
}

func defaults() *State {
	return &State{
		Version:   1,
		Sessions:  map[string]Session{},
		Installed: map[string]Installed{},
		Settings: Settings{
			NodeName:        "Outpost",
			PublicAccess:    true,
			AssistantPublic: true,
			CheckForUpdates: true,
		},
		Apps:      Apps{Library: true, Maps: true},
		Selection: catalog.Selection{Tiers: map[string]string{}},
	}
}

// Store guards the state and its file.
type Store struct {
	mu   sync.RWMutex
	path string
	s    *State
}

// Open loads path, or starts from defaults when it does not exist.
func Open(path string) (*Store, error) {
	st := &Store{path: path, s: defaults()}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, st.save()
	}
	if err != nil {
		return nil, err
	}
	s := defaults()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Sessions == nil {
		s.Sessions = map[string]Session{}
	}
	if s.Installed == nil {
		s.Installed = map[string]Installed{}
	}
	if s.Selection.Tiers == nil {
		s.Selection.Tiers = map[string]string{}
	}
	st.s = s
	return st, nil
}

func (st *Store) save() error {
	data, err := json.MarshalIndent(st.s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(st.path, data, 0o600)
}

// View runs fn with read access. fn must not keep references.
func (st *Store) View(fn func(s *State)) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	fn(st.s)
}

// Update runs fn with write access and saves when fn returns nil. If saving
// fails the in-memory state is rolled back so memory and disk agree.
func (st *Store) Update(fn func(s *State) error) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	backup, err := json.Marshal(st.s)
	if err != nil {
		return err
	}
	if err := fn(st.s); err != nil {
		restore(st, backup)
		return err
	}
	if err := st.save(); err != nil {
		restore(st, backup)
		return err
	}
	return nil
}

func restore(st *Store, backup []byte) {
	s := defaults()
	if json.Unmarshal(backup, s) == nil {
		st.s = s
	}
}
