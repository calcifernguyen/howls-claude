package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddIdempotentWithBackup(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	os.MkdirAll(filepath.Join(h, ".claude", "skills"), 0o755)
	os.WriteFile(filepath.Join(h, ".claude", "settings.json"), []byte("{}"), 0o644)
	acc := filepath.Join(h, ".claude-work")
	os.MkdirAll(acc, 0o755)
	os.WriteFile(filepath.Join(acc, "settings.json"), []byte("old"), 0o644) // file thật phải bị backup

	for i := 0; i < 2; i++ {
		if err := cmdAdd("work"); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range shared {
		got, err := os.Readlink(filepath.Join(acc, item))
		if want := filepath.Join(h, ".claude", item); err != nil || got != want {
			t.Errorf("%s: link=%q err=%v, want %q", item, got, err, want)
		}
	}
	baks, _ := filepath.Glob(filepath.Join(acc, "settings.json.hcc-bak-*"))
	if len(baks) != 1 {
		t.Errorf("want 1 backup, got %v", baks)
	}
	if got := accounts(); len(got) != 2 || got[1] != "work" {
		t.Errorf("accounts = %v", got)
	}
	if cmdAdd("main") == nil || cmdAdd("../x") == nil {
		t.Error("tên không hợp lệ phải lỗi")
	}
}

func TestKeychainService(t *testing.T) {
	if got := keychainService("/Users/howlpendragon/.claude-work", true); got != "Claude Code-credentials-b4807af0" {
		t.Errorf("hashed = %q", got)
	}
	if got := keychainService("x", false); got != "Claude Code-credentials" {
		t.Errorf("plain = %q", got)
	}
}

func TestFormatUsage(t *testing.T) {
	got, err := formatUsage([]byte(`{"five_hour":{"utilization":12.4,"resets_at":null},"seven_day":{"utilization":40,"resets_at":"2026-09-28T09:00:00Z"},"seven_day_opus":null}`), false)
	if err != nil {
		t.Fatal(err)
	}
	want := "  5h         ██░░░░░░░░░░░░░░░░░░  12%\n" +
		"  7d         ████████░░░░░░░░░░░░  40%  reset " + time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC).Local().Format("Mon 15:04")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBar(t *testing.T) {
	for pct, n := range map[float64]int{0: 0, -5: 0, 100: 20, 150: 20, 12.4: 2} {
		if got := bar(pct, false); got != strings.Repeat("█", n)+strings.Repeat("░", 20-n) {
			t.Errorf("bar(%v) = %q", pct, got)
		}
	}
	if got := bar(90, true); !strings.HasPrefix(got, "\x1b[31m") {
		t.Errorf("bar(90) màu = %q", got)
	}
}

func TestAliasLine(t *testing.T) {
	if got := aliasLine("main"); got != "alias claude='hcc main --dangerously-skip-permissions'" {
		t.Errorf("aliasLine(main) = %q", got)
	}
	if got := aliasLine("work"); got != "alias claude-work='hcc work --dangerously-skip-permissions'" {
		t.Errorf("aliasLine(work) = %q", got)
	}
}

func TestCmdAlias(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)

	if err := cmdAlias([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "chưa có") {
		t.Errorf("cmdAlias(nope) want error containing 'chưa có', got %v", err)
	}
	if err := cmdAlias([]string{"bad/name"}); err == nil || !strings.Contains(err.Error(), "tên không hợp lệ") {
		t.Errorf("cmdAlias(bad/name) want invalid name error, got %v", err)
	}
	if err := cmdAlias(nil); err != nil {
		t.Errorf("cmdAlias(nil) error: %v", err)
	}

	os.MkdirAll(filepath.Join(h, ".claude"), 0o755)
	if err := cmdAdd("work"); err != nil {
		t.Fatal(err)
	}
	if err := cmdAlias([]string{"work"}); err != nil {
		t.Errorf("cmdAlias(work) error: %v", err)
	}
}
