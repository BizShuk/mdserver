# CLAUDE.md — mdserver 技術脈絡 (Technical Context)

Go module `github.com/bizshuk/mdserver`，Go `1.26.3`。單一 binary，無 daemon、
無資料庫、無設定檔。

## 結構 (Structure)

```text
mdserver/
├── main.go              # composition root：flag 解析、port 選擇、HTTP server 生命週期
├── main_test.go
├── svc/
│   ├── site/            # 路徑 → 檔案的解析層 (routing)
│   │   ├── site.go      # Site、Target、Kind；路徑解析與 root 逃逸防護
│   │   ├── listing.go   # 目錄索引項目列舉
│   │   ├── pagemeta.go  # 從 frontmatter 取頁面標題/描述
│   │   └── cache.go     # ttlCache：目錄列舉結果的短期快取
│   ├── render/          # Markdown → HTML 的轉換層
│   │   ├── render.go    # goldmark pipeline 組裝、Document、Renderer
│   │   ├── alert.go     # GitHub alert 區塊 (AST transformer + renderer)
│   │   ├── mermaid.go   # mermaid fence → 前端可辨識的節點
│   │   ├── link.go      # 內部連結改寫成正規網址
│   │   ├── toc.go       # heading 收集 → 目錄
│   │   └── chroma.go    # SyntaxCSS()：啟動時產生語法上色 CSS
│   └── server/          # HTTP 層
│       ├── server.go    # Server、asset handler、版型解析
│       └── page.go      # 單一 request 的 Target → response 分派
├── web/                 # 內嵌前端資產 (go:embed)
│   ├── assets.go        # embed.FS 與 PAGE_TEMPLATE 常數
│   ├── page.html        # 頁面版型
│   ├── style.css
│   ├── app.js
│   └── vendor/mermaid.min.js
└── skills/mdserver/     # Claude Code skill 定義
```

## 分層規則 (Layering)

```
main  →  server  →  render
                 →  site
                 →  web (embed)
```

- `site` 不知道 HTTP，也不知道 Markdown 怎麼轉；它只回答「這個路徑對應到磁碟上的什麼」。
- `render` 不知道 HTTP，也不碰檔案系統；它的輸入是 bytes，輸出是 `Document`。
- `server` 是唯一同時認識兩者的地方，負責把 `Target` 分派成 response。
- `site` 與 `render` 互不 import。新增功能時若覺得需要打通，代表分層放錯了。

## 關鍵決策 (Key Decisions)

- `每 request 重新轉換`：不做輸出快取。編輯即所見是這個工具的全部價值，
  任何 render cache 都會製造陳舊輸出的風險。唯一的快取是目錄列舉的 TTL cache
  (`svc/site/cache.go`)，因為它是 `readdir` 成本而非正確性問題。
- `零設定`：路由規則就是全部的設定。沒有設定檔、沒有 `_config.yml`、
  沒有 front matter 路由宣告。新增行為前先問「能不能從資料夾結構推導出來」。
- `資產內嵌`：CSS/JS/mermaid 全部 `go:embed` 進 binary，因此離線可用，
  且 `ASSET_CACHE_CONTROL` 可以放心設長 TTL — 資產只會隨 binary 一起變。
- `資產路徑前綴 _mdserver/`：以底線開頭，確保不會與服務根目錄下的真實資料夾撞名。
- `HTML 檔原樣輸出`：`.html` / `.htm` 已經是輸出格式，不進 Markdown pipeline、
  不套版型，避免雙重包裝。
- `TLS 憑證只認一個慣例路徑`：`-tls` 不接受參數，固定讀
  `~/.config/mdserver/certs/mdserver.{crt,key}` — 也就是區網私有 CA
  (`platform/inf` 的 `pkg/tls/scripts/issue.sh`) 的固定落點。這讓「零設定」
  延伸到 HTTPS：不必記路徑，也不必在專案裡放憑證。`-cert` / `-key` 是逃生門，
  必須成對出現。憑證在 `net.Listen` `之前`載入，路徑錯誤不會留下半開的 listener。
- `port 自動遞增`：預設 8080，被佔用就往後掃最多 20 個
  (`PORT_SCAN_LIMIT`)，確保零參數情境永遠可用。

## 慣例 (Conventions)

- 常數一律 `SCREAMING_SNAKE_CASE`（含 unexported 與函式內 block-scoped）；
  `var` 維持 `MixedCaps`。
- goldmark 的 AST transformer 以 `PRIORITY_TRANSFORM_*` 常數明確排序；
  alert 與 mermaid 都必須在 render 前完成，改動順序前先看這組常數。
- 錯誤一律 `fmt.Errorf(...: %w, err)` 包裝並向上傳；`main.go` 是唯一
  決定 exit code 的地方。
- 日誌用 stdlib `log/slog`；`-quiet` 只提高 level，不改變訊息內容。

## 已知陷阱 (Gotchas)

- goldmark 的 `ASTTransformer` 在 inline parsing `之後`才執行，
  `SetLines` 不會清掉已解析的 inline node — 若用 marker 字串做轉換，
  marker 會洩漏到畫面上。`alert.go` / `mermaid.go` 因此都直接操作 AST 節點。
- `svc/site` 的路徑解析必須在 `filepath.Abs` 之後才判斷 root 邊界，
  否則 `..` 可能繞過檢查；改動 `Target` 解析時務必保留 `ErrOutsideRoot` 路徑。
- 相依只有 goldmark 家族與 chroma；不要為了單一功能引入新的 render 相依，
  優先看 goldmark extension 是否已提供。
