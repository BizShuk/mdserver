---
name: mdserver
description: Use when previewing, serving, or locally browsing Markdown directories with mdserver, especially when selecting a port, binding address, site root, or title, or diagnosing port conflicts and stale installed binaries.
---

# mdserver

## Overview

使用 `mdserver` 將 Markdown directory 即時 render 成 website。先確認實際 executable 的 flags，再明確選擇 site root、port 與 network exposure；startup output 顯示的 URL 才是 runtime truth。

## Workflow

1. 確認 target directory 與 runtime capability：

   ```bash
   target_dir="./docs"
   test -d "$target_dir"
   command -v mdserver
   mdserver -h
   ```

   若 installed binary 的 help 沒有需要的 flag，回報 version drift。在 mdserver source checkout 內可用 `go run .` 執行 current source；只有 user 要求更新 executable 時才執行 `go install .`。

2. 依 user request 選擇 execution mode：

   - **Read-only／instructions only：** 只檢查 directory、`command -v`、help 與 source；提供可執行 command，並明確標示 server 與 HTTP verification 未執行。
   - **Start／preview now：** 繼續執行以下 startup、verification 與 lifecycle steps。

3. 依 exposure requirement 選擇一種 command：

   | Requirement | Command | Binding behavior |
   | --- | --- | --- |
   | Automatic port | `mdserver "$target_dir"` | 從 `8080` 掃描至 `8099`，wildcard bind |
   | Explicit port | `mdserver -port 4317 "$target_dir"` | 使用 `:4317`，wildcard bind |
   | Loopback only | `mdserver -addr 127.0.0.1:4317 "$target_dir"` | 只接受本機連線 |
   | Custom title | `mdserver -title "Project Docs" "$target_dir"` | 使用 automatic port |
   | HTTPS (區網 CA) | `mdserver -tls "$target_dir"` | 讀 `~/.config/mdserver/certs/mdserver.{crt,key}` |
   | HTTPS (指定憑證) | `mdserver -cert a.crt -key a.key "$target_dir"` | 覆蓋慣例路徑，兩者必須成對 |

   `-port` 會覆蓋 `-addr`。需要 loopback 時只使用完整的 `-addr`，不要同時傳入兩者。

   `-tls` 不接受參數。憑證由 `platform/inf` 的 `pkg/tls/scripts/issue.sh mdserver`
   在`持有 CA 私鑰的機器`上簽發；本機沒有 `~/.config/inf/ca.key` 就簽不了，
   必須把簽好的一對拷到 `~/.config/mdserver/certs/`。憑證的 SAN 只含
   `mdserver.local`、`localhost` 與簽發當下的 LAN IP，用其他名字連會握手失敗。

4. 以前景模式啟動並保留 process handle。若 user 要求 persistent background lifecycle，**REQUIRED SUB-SKILL:** 使用 `pm2`。

5. 從 startup output 取得 actual URL，再驗證 HTTP 與 listener：

   ```bash
   curl -fsSI http://127.0.0.1:4317/
   lsof -nP -a -iTCP:4317 -sTCP:LISTEN
   ```

   HTTPS 模式改用 `curl -fsSI https://localhost:4317/`；憑證鏈驗不過時用
   `--cacert ~/projects/platform/inf/pkg/tls/ca/ca.crt` 確認是 client 未信任 CA，
   而不是 server 憑證有問題。

   Loopback requirement 下，listener 必須顯示 `127.0.0.1:4317`，不能是 `*:4317`、`0.0.0.0:4317` 或 `[::]:4317`。

6. Foreground preview 用 `Ctrl-C`／`SIGINT` 停止，讓 server graceful shutdown。若 user 要求持續運行，回報 actual URL 與 process manager 狀態，不要自行停止。

## Example

在 loopback port `4317` preview `./docs` 並使用自訂 title：

```bash
target_dir="./docs"
test -d "$target_dir" &&
mdserver -addr 127.0.0.1:4317 -title "Project Docs" "$target_dir"
```

另一個 terminal 執行 `curl -fsSI http://127.0.0.1:4317/`；完成 preview 後回到 server terminal 按 `Ctrl-C`。

## Common Mistakes

- 不先讀 `mdserver -h`，直接對 stale binary 使用不存在的 `-port`。
- 同時使用 `-addr` 與 `-port`，誤以為仍是 loopback bind。
- 假設 automatic mode 一定使用 `8080`，沒有讀 startup output 的 actual URL。
- 未確認 target directory 存在，或從錯誤 cwd serve 錯誤 root。
- 在 read-only／instructions-only request 中仍啟動 server 或執行 HTTP smoke test。
- 只看到 process running 就宣稱成功，沒有用 HTTP request 驗證 rendered site。
- 在 HTTPS 模式下仍用 `http://` 連線，把 `client sent an HTTP request to an HTTPS server` 誤判成 server 壞掉。
- 只給 `-cert` 沒給 `-key`（或反過來），mdserver 會直接拒絕啟動。
