package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func seedSession(t *testing.T, dir, id, repo string, state models.SessionState, pid int) {
	t.Helper()
	sd := filepath.Join(dir, id)
	if err := os.MkdirAll(sd, 0755); err != nil {
		t.Fatal(err)
	}
	rec := models.SessionRecord{
		SessionID:  id,
		RepoSource: repo,
		State:      state,
		FCPid:      pid,
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sd, "session.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteSessionIDListsSessionsWithRepoPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CELL_SESSION_DATA_DIR", dir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	seedSession(t, dir, "aaa111", "/home/u/repo-one", models.StateStopped, 0)
	seedSession(t, dir, "bbb222", "/home/u/repo-two", models.StateRunning, 999999999)

	cmd := &cobra.Command{Use: "attach"}
	cmd.Flags().String("session", "", "")
	comp := completeSessionID(cfg)
	got, directive := comp(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive=%v", directive)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	found := strings.Join(got, "\n")
	for _, want := range []string{"aaa111\t/home/u/repo-one (stopped)", "bbb222\t/home/u/repo-two (running)"} {
		if !strings.Contains(found, want) {
			t.Fatalf("missing %q in %v", want, got)
		}
	}
}

func TestListSummariesSkipsBrokenSessions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CELL_SESSION_DATA_DIR", dir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	seedSession(t, dir, "ccc333", "/home/u/repo-three", models.StateStopped, 0)
	if err := os.MkdirAll(filepath.Join(dir, "broke"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broke", "session.json"), []byte("{bad"), 0644); err != nil {
		t.Fatal(err)
	}
	sm, err := session.NewSessionManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	list, err := sm.ListSummaries()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].SessionID != "ccc333" {
		t.Fatalf("list=%v", list)
	}
}
