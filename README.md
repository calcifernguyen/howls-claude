# hcc — howls-claude, Claude Code account switcher

Chạy nhiều subscription Claude Code song song. Mỗi account = 1 thư mục config (`CLAUDE_CONFIG_DIR`),
keychain macOS tự tách credential theo thư mục. Dựa trên bài "Running Two Claude Code Subscriptions Side by Side" (docs/).

- `main` = `~/.claude`, `<name>` = `~/.claude-<name>`
- Account phụ symlink từ `~/.claude`: `CLAUDE.md settings.json skills agents commands plugins history.jsonl projects`
  (dùng chung skill/plugin/lịch sử prompt, `--resume` được session của account khác)
- Riêng từng account: `.claude.json` (login, MCP user-scope), credential, cache

## Cài

```sh
go build -o ~/.local/bin/hcc .
```

## Dùng

```sh
hcc add work              # tạo/đồng bộ ~/.claude-work; item thật cũ -> <item>.hcc-bak-<unix>
hcc work                  # chạy claude bằng account work (lần đầu sẽ login)
hcc main -c               # args sau tên account chuyển thẳng cho claude
hcc list                  # * = account theo $CLAUDE_CONFIG_DIR hiện tại
hcc quota [name]          # biểu đồ quota 5h/7 ngày + giờ reset (không name = mọi account)
eval "$(hcc env work)"    # set env cho shell hiện tại (như c2 trong bài)
```

`hcc quota` vẽ thanh 20 ô cho mỗi cửa sổ; màu ANSI chuẩn (xanh < 50%, vàng < 80%, đỏ ≥ 80%) theo theme terminal, tự tắt khi pipe hoặc `NO_COLOR` được set:

```
main       a@x.com
  5h         ███░░░░░░░░░░░░░░░░░  13%  reset Thu 09:59
  7d         █████████████░░░░░░░  66%  reset Mon 17:00
```

Alias gợi ý trong `~/.zshrc`:

```sh
alias claude='hcc main --dangerously-skip-permissions'
alias claude-w='hcc work --dangerously-skip-permissions'
```

`$CLAUDE_ACCOUNT` được set khi chạy, statusline có thể đọc để hiện account.

Test: `go test ./...`
