package persist

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Agent kinds we know how to resume.
const (
	AgentClaude = "claude"
	AgentCodex  = "codex"
)

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return os.Getenv("HOME")
}

// DetectAgentForPID walks the descendant process tree of a shell PID looking
// for a supported AI agent (claude/codex). When found it returns the AgentRef
// and the agent process's own working directory — the agent's cwd, not the
// shell's launch directory, is what determines which project/session it belongs
// to. Returns (nil, "") when no supported agent is running.
func DetectAgentForPID(shellPID int) (*AgentRef, string) {
	pid, kind := findAgentProcess(shellPID)
	if kind == "" {
		return nil, ""
	}
	cwd := ProcessCwd(pid)
	ref := &AgentRef{Kind: kind}
	switch kind {
	case AgentClaude:
		ref.SessionID = FindClaudeSessionID(cwd)
	case AgentCodex:
		ref.SessionID = FindCodexSessionID(cwd)
	}
	return ref, cwd
}

// ProcessCwd returns the live working directory of a process, tracked through
// chdir, via lsof (available on macOS and Linux). Returns "" on failure.
func ProcessCwd(pid int) string {
	if pid <= 0 {
		return ""
	}
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	// -Fn output looks like:
	//   p1234
	//   fcwd
	//   n/some/path
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") && len(line) > 1 {
			return line[1:]
		}
	}
	return ""
}

// findAgentProcess returns the PID and kind of a supported agent running
// anywhere in the descendant process tree of shellPID, or (0, "") if none.
// Agents like claude may be grandchildren of the shell (shell → launcher →
// agent), so we walk the tree breadth-first using pgrep, which is available on
// macOS and Linux.
func findAgentProcess(shellPID int) (int, string) {
	if shellPID <= 0 {
		return 0, ""
	}
	visited := map[int]bool{}
	queue := []int{shellPID}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if visited[pid] {
			continue
		}
		visited[pid] = true
		for _, child := range childProcesses(pid) {
			if visited[child.pid] {
				continue
			}
			switch child.name {
			case AgentClaude:
				return child.pid, AgentClaude
			case AgentCodex:
				return child.pid, AgentCodex
			}
			queue = append(queue, child.pid)
		}
	}
	return 0, ""
}

type childProc struct {
	pid  int
	name string
}

// childProcesses lists the direct children of pid with their command names.
func childProcesses(pid int) []childProc {
	out, err := exec.Command("pgrep", "-lf", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		// pgrep exits non-zero when nothing matches; that is the common case.
		return nil
	}
	var children []childProc
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		cpid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		children = append(children, childProc{pid: cpid, name: strings.ToLower(filepath.Base(fields[1]))})
	}
	return children
}

// FindClaudeSessionID returns the most recently modified Claude Code session ID
// for the project rooted at cwd. Claude stores sessions as
// ~/.claude/projects/<cwd-slug>/<sessionId>.jsonl where the slug replaces any
// character outside [A-Za-z0-9-] (including "/" and ".") with "-".
func FindClaudeSessionID(cwd string) string {
	if cwd == "" {
		return ""
	}
	slug := claudeSlug(cwd)
	dir := filepath.Join(homeDir(), ".claude", "projects", slug)
	return newestJSONLStem(dir)
}

// claudeSlug mirrors how Claude Code derives a project directory name from a
// working directory path.
func claudeSlug(cwd string) string {
	var b strings.Builder
	for _, r := range cwd {
		if r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// FindCodexSessionID returns the most recent Codex session ID rooted at cwd.
// Codex names files ~/.codex/sessions/**/rollout-*.jsonl and records
// payload.session_id and payload.cwd in the first line. Sessions from other
// working directories are skipped so we never resume the wrong project; an
// empty cwd falls back to the newest session overall.
func FindCodexSessionID(cwd string) string {
	root := filepath.Join(homeDir(), ".codex", "sessions")
	var files []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), "rollout-") && strings.HasSuffix(d.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	// Newest first.
	sort.Slice(files, func(i, j int) bool {
		fi, ei := fileModTime(files[i]), fileModTime(files[j])
		if fi.Equal(ei) {
			return files[i] > files[j]
		}
		return fi.After(ei)
	})
	var fallback string
	for _, f := range files {
		id, sessionCwd := readCodexSessionMeta(f)
		if id == "" {
			continue
		}
		if cwd == "" && fallback == "" {
			fallback = id
		}
		if sessionCwd == cwd {
			return id
		}
	}
	return fallback
}

// readCodexSessionMeta extracts the session ID and cwd from the session_meta
// line at the head of a Codex rollout file.
func readCodexSessionMeta(path string) (id, cwd string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	// Only the first line carries session_meta; scan just the head.
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, []byte("session_meta")) {
			continue
		}
		var meta struct {
			Payload struct {
				SessionID string `json:"session_id"`
				ID        string `json:"id"`
				Cwd       string `json:"cwd"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(line, &meta); err != nil {
			return "", ""
		}
		id = meta.Payload.SessionID
		if id == "" {
			id = meta.Payload.ID
		}
		return id, meta.Payload.Cwd
	}
	return "", ""
}

// newestJSONLStem returns the filename stem (without .jsonl) of the most
// recently modified *.jsonl file in dir, or "" if none.
func newestJSONLStem(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestTime int64 = -1
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		mod := info.ModTime().UnixNano()
		if mod > bestTime {
			bestTime = mod
			best = entry.Name()
		}
	}
	return strings.TrimSuffix(best, ".jsonl")
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// ResumeCommand builds the command used to resume an agent session after a
// restart. Returns nil for unknown kinds or a missing session ID.
func ResumeCommand(agent *AgentRef) []string {
	if agent == nil || agent.SessionID == "" {
		return nil
	}
	switch agent.Kind {
	case AgentClaude:
		return []string{"claude", "--resume", agent.SessionID}
	case AgentCodex:
		return []string{"codex", "resume", agent.SessionID}
	}
	return nil
}
