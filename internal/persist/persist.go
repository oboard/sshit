// Package persist implements on-disk session snapshots and terminal history
// so the web workspace can be restored after the server (or machine) restarts.
//
// Layout, anchored at a per-port directory like ~/.sshit/<port>/:
//
//	session.json           structural snapshot of every window (kind-discriminated)
//	history/<windowID>.txt terminal scrollback for shell windows (optional)
package persist

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Window kinds.
const (
	KindShell  = "shell"
	KindEditor = "editor"
)

// AgentRef identifies a detected AI agent session that can be resumed.
type AgentRef struct {
	Kind      string `json:"kind"` // "claude" | "codex"
	SessionID string `json:"sessionId,omitempty"`
}

// Window is the persisted form of a single workspace window. Shell and editor
// windows share one array and are told apart by Kind.
type Window struct {
	ID     int64  `json:"id"`
	Kind   string `json:"kind"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	ZIndex int    `json:"zIndex"`

	// Shell-only.
	Cols  uint16    `json:"cols,omitempty"`
	Rows  uint16    `json:"rows,omitempty"`
	Cwd   string    `json:"cwd,omitempty"`
	Agent *AgentRef `json:"agent,omitempty"`

	// Editor-only.
	DocID string `json:"docId,omitempty"`
}

// Snapshot is the full restorable workspace state.
type Snapshot struct {
	Version int      `json:"version"`
	IDSeq   int64    `json:"idSeq"`
	Windows []Window `json:"windows"`
}

const snapshotVersion = 1

func sessionPath(dir string) string { return filepath.Join(dir, "session.json") }

func collabPath(dir string) string { return filepath.Join(dir, "collab.json") }

func historyDir(dir string) string { return filepath.Join(dir, "history") }

func historyPath(dir string, id int64) string {
	return filepath.Join(historyDir(dir), strconv.FormatInt(id, 10)+".txt")
}

// Load reads session.json from dir. A missing file is not an error: it returns
// an empty snapshot so first-run behavior stays unchanged.
func Load(dir string) (*Snapshot, error) {
	data, err := os.ReadFile(sessionPath(dir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Snapshot{Version: snapshotVersion}, nil
		}
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse session snapshot: %w", err)
	}
	if snap.Windows == nil {
		snap.Windows = []Window{}
	}
	return &snap, nil
}

// Write atomically persists the snapshot to session.json so a crash mid-write
// never leaves a truncated file behind.
func Write(dir string, snap *Snapshot) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	snap.Version = snapshotVersion
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	tmp := sessionPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, sessionPath(dir))
}

// WriteHistory persists a shell window's scrollback buffer.
func WriteHistory(dir string, id int64, data []byte) error {
	if err := os.MkdirAll(historyDir(dir), 0700); err != nil {
		return err
	}
	return os.WriteFile(historyPath(dir, id), data, 0600)
}

// ReadHistory returns a shell window's saved scrollback, or nil if none exists.
func ReadHistory(dir string, id int64) ([]byte, error) {
	data, err := os.ReadFile(historyPath(dir, id))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

// DeleteHistory removes a shell window's history file, if present.
func DeleteHistory(dir string, id int64) error {
	err := os.Remove(historyPath(dir, id))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// PruneHistory deletes history files for window IDs not in keep.
func PruneHistory(dir string, keep map[int64]bool) error {
	entries, err := os.ReadDir(historyDir(dir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".txt")
		id, err := strconv.ParseInt(name, 10, 64)
		if err != nil {
			continue
		}
		if !keep[id] {
			if err := os.Remove(filepath.Join(historyDir(dir), entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteCollab atomically persists the collaborative document's update log.
// Updates are raw Yjs update payloads; encoding/json renders each []byte as
// base64, which keeps the file self-describing and diff-friendly.
func WriteCollab(dir string, updates [][]byte) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(updates)
	if err != nil {
		return err
	}
	tmp := collabPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, collabPath(dir))
}

// LoadCollab reads the persisted collaborative update log. A missing file is
// not an error: it returns nil.
func LoadCollab(dir string) ([][]byte, error) {
	data, err := os.ReadFile(collabPath(dir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var updates [][]byte
	if err := json.Unmarshal(data, &updates); err != nil {
		return nil, fmt.Errorf("parse collab log: %w", err)
	}
	return updates, nil
}
