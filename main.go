// hcc: chạy nhiều account Claude Code song song bằng CLAUDE_CONFIG_DIR.
// main = ~/.claude, <name> = ~/.claude-<name>. Credential keychain tự tách theo dir.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Item symlink từ ~/.claude sang account phụ.
var shared = []string{"CLAUDE.md", "settings.json", "skills", "agents", "commands", "plugins", "history.jsonl", "projects"}

// Item nguồn chưa có thì tạo dạng thư mục (còn lại tạo file rỗng).
var sharedDirs = map[string]bool{"skills": true, "agents": true, "commands": true, "plugins": true, "projects": true}

const aliasFlags = "--dangerously-skip-permissions"

const usage = `hcc — Claude Code account switcher

  hcc add <name>            tạo/đồng bộ ~/.claude-<name>, symlink shared từ ~/.claude
  hcc list                  liệt kê account (* = đang active theo $CLAUDE_CONFIG_DIR)
  hcc env <name>            in lệnh export, dùng: eval "$(hcc env <name>)"
  hcc alias [name]          in alias zsh gợi ý, dùng: eval "$(hcc alias)"
  hcc quota [name]          xem quota 5h/7 ngày (không name = mọi account)
  hcc <name> [args...]      chạy claude với account <name>`

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		die(err)
	}
	return h
}

func mainDir() string { return filepath.Join(home(), ".claude") }

func accountDir(name string) string {
	if name == "main" {
		return mainDir()
	}
	return filepath.Join(home(), ".claude-"+name)
}

func die(v any) {
	fmt.Fprintln(os.Stderr, "hcc:", v)
	os.Exit(1)
}

func validName(name string) bool {
	return name != "" && !strings.ContainsAny(name, `/\. `) && !strings.HasPrefix(name, "-")
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Println(usage)
		return
	}
	var err error
	switch args[0] {
	case "add":
		if len(args) != 2 {
			die("usage: hcc add <name>")
		}
		err = cmdAdd(args[1])
	case "list", "ls":
		err = cmdList()
	case "env":
		if len(args) != 2 {
			die("usage: hcc env <name>")
		}
		err = cmdEnv(args[1])
	case "alias":
		if len(args) > 2 {
			die("usage: hcc alias [name]")
		}
		err = cmdAlias(args[1:])
	case "quota":
		err = cmdQuota(args[1:])
	default:
		err = run(args[0], args[1:])
	}
	if err != nil {
		die(err)
	}
}

func cmdAdd(name string) error {
	if !validName(name) || name == "main" {
		return fmt.Errorf("tên không hợp lệ: %q", name)
	}
	dir := accountDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, item := range shared {
		if err := linkShared(item, dir); err != nil {
			return fmt.Errorf("%s: %w", item, err)
		}
	}
	fmt.Printf("OK: %s. Login: hcc %s\n", dir, name)
	return nil
}

// linkShared: dir/item -> ~/.claude/item. Idempotent; item thật bị rename sang .hcc-bak-<unix>, không xoá.
func linkShared(item, dir string) error {
	src := filepath.Join(mainDir(), item)
	dst := filepath.Join(dir, item)
	if _, err := os.Lstat(src); os.IsNotExist(err) {
		if sharedDirs[item] {
			err = os.MkdirAll(src, 0o755)
		} else {
			err = os.WriteFile(src, nil, 0o644)
		}
		if err != nil {
			return err
		}
	}
	if cur, err := os.Readlink(dst); err == nil && cur == src {
		return nil
	}
	if _, err := os.Lstat(dst); err == nil {
		bak := fmt.Sprintf("%s.hcc-bak-%d", dst, time.Now().Unix())
		if err := os.Rename(dst, bak); err != nil {
			return err
		}
		fmt.Printf("backup: %s -> %s\n", dst, bak)
	}
	return os.Symlink(src, dst)
}

func accounts() []string {
	names := []string{"main"}
	matches, _ := filepath.Glob(filepath.Join(home(), ".claude-*"))
	for _, m := range matches {
		// Account = đã login (.claude.json) hoặc tạo bởi hcc (settings.json là symlink); loại ~/.claude-mem...
		_, login := os.Stat(filepath.Join(m, ".claude.json"))
		fi, link := os.Lstat(filepath.Join(m, "settings.json"))
		if login == nil || (link == nil && fi.Mode()&os.ModeSymlink != 0) {
			names = append(names, strings.TrimPrefix(filepath.Base(m), ".claude-"))
		}
	}
	return names
}

func email(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, ".claude.json"))
	if err != nil {
		return "(chưa login)"
	}
	var cfg struct {
		OauthAccount struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(b, &cfg) != nil || cfg.OauthAccount.EmailAddress == "" {
		return "(chưa login)"
	}
	return cfg.OauthAccount.EmailAddress
}

func cmdList() error {
	active := filepath.Clean(os.Getenv("CLAUDE_CONFIG_DIR"))
	for _, name := range accounts() {
		dir := accountDir(name)
		mark := " "
		if dir == active {
			mark = "*"
		}
		fmt.Printf("%s %-10s %-30s %s\n", mark, name, email(dir), dir)
	}
	return nil
}

func resolve(name string) (string, error) {
	if !validName(name) {
		return "", fmt.Errorf("tên không hợp lệ: %q\n%s", name, usage)
	}
	dir := accountDir(name)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("account %q chưa có, chạy: hcc add %s", name, name)
	}
	return dir, nil
}

func cmdEnv(name string) error {
	dir, err := resolve(name)
	if err != nil {
		return err
	}
	fmt.Printf("export CLAUDE_CONFIG_DIR=%q CLAUDE_ACCOUNT=%q\n", dir, name)
	return nil
}

func aliasLine(name string) string {
	aliasName := "claude"
	if name != "main" {
		aliasName = "claude-" + name
	}
	return fmt.Sprintf("alias %s='hcc %s %s'", aliasName, name, aliasFlags)
}

func cmdAlias(names []string) error {
	if len(names) == 0 {
		for _, name := range accounts() {
			fmt.Println(aliasLine(name))
		}
		return nil
	}
	if _, err := resolve(names[0]); err != nil {
		return err
	}
	fmt.Println(aliasLine(names[0]))
	return nil
}

func run(name string, args []string) error {
	dir, err := resolve(name)
	if err != nil {
		return err
	}
	bin, err := exec.LookPath("claude")
	if err != nil {
		return err
	}
	env := []string{"CLAUDE_CONFIG_DIR=" + dir, "CLAUDE_ACCOUNT=" + name}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "CLAUDE_CONFIG_DIR=") && !strings.HasPrefix(kv, "CLAUDE_ACCOUNT=") {
			env = append(env, kv)
		}
	}
	return syscall.Exec(bin, append([]string{"claude"}, args...), env)
}

// keychainService: tên service claude dùng lưu credential; có CLAUDE_CONFIG_DIR thì thêm 8 hex đầu sha256(dir).
func keychainService(dir string, hashed bool) string {
	if !hashed {
		return "Claude Code-credentials"
	}
	sum := sha256.Sum256([]byte(dir))
	return "Claude Code-credentials-" + hex.EncodeToString(sum[:])[:8]
}

// token: OAuth access token của account từ keychain macOS. Không tự refresh (claude sẽ refresh khi chạy).
func token(name, dir string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", keychainService(dir, true), "-a", os.Getenv("USER"), "-w").Output()
	if err != nil && name == "main" {
		out, err = exec.Command("security", "find-generic-password", "-s", keychainService(dir, false), "-a", os.Getenv("USER"), "-w").Output()
	}
	if err != nil {
		return "", fmt.Errorf("chưa login, chạy: hcc %s", name)
	}
	var cred struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(out, &cred); err != nil || cred.ClaudeAiOauth.AccessToken == "" {
		return "", fmt.Errorf("credential không đọc được")
	}
	if cred.ClaudeAiOauth.ExpiresAt < time.Now().UnixMilli() {
		return "", fmt.Errorf("token hết hạn, chạy: hcc %s", name)
	}
	return cred.ClaudeAiOauth.AccessToken, nil
}

func fetchUsage(tok string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usage API: %s", resp.Status)
	}
	return body, nil
}

// bar: thanh 20 ô cho pct%. color = 16 màu ANSI chuẩn nên tự khớp theme terminal.
func bar(pct float64, color bool) string {
	n := min(max(int(math.Round(pct/5)), 0), 20)
	full, empty := strings.Repeat("█", n), strings.Repeat("░", 20-n)
	if !color {
		return full + empty
	}
	c := "32" // green
	if pct >= 80 {
		c = "31" // red
	} else if pct >= 50 {
		c = "33" // yellow
	}
	return "\x1b[" + c + "m" + full + "\x1b[0;2m" + empty + "\x1b[0m"
}

func formatUsage(body []byte, color bool) (string, error) {
	type window struct {
		Utilization float64    `json:"utilization"`
		ResetsAt    *time.Time `json:"resets_at"`
	}
	var u struct {
		FiveHour       *window `json:"five_hour"`
		SevenDay       *window `json:"seven_day"`
		SevenDayOpus   *window `json:"seven_day_opus"`
		SevenDaySonnet *window `json:"seven_day_sonnet"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	var lines []string
	for _, w := range []struct {
		label string
		w     *window
	}{{"5h", u.FiveHour}, {"7d", u.SevenDay}, {"7d-opus", u.SevenDayOpus}, {"7d-sonnet", u.SevenDaySonnet}} {
		if w.w == nil {
			continue
		}
		l := fmt.Sprintf("  %-10s %s %3.0f%%", w.label, bar(w.w.Utilization, color), w.w.Utilization)
		if w.w.ResetsAt != nil {
			l += "  reset " + w.w.ResetsAt.Local().Format("Mon 15:04")
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n"), nil
}

func quota(name, dir string, color bool) (string, error) {
	tok, err := token(name, dir)
	if err != nil {
		return "", err
	}
	body, err := fetchUsage(tok)
	if err != nil {
		return "", err
	}
	return formatUsage(body, color)
}

func cmdQuota(names []string) error {
	if len(names) == 0 {
		names = accounts()
	}
	fi, _ := os.Stdout.Stat()
	color := fi != nil && fi.Mode()&os.ModeCharDevice != 0 && os.Getenv("NO_COLOR") == ""
	for _, name := range names {
		dir, err := resolve(name)
		if err != nil {
			return err
		}
		out, err := quota(name, dir, color)
		if err != nil {
			out = "  lỗi: " + err.Error()
		}
		fmt.Printf("%-10s %s\n%s\n", name, email(dir), out)
	}
	return nil
}
