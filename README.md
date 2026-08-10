# mdserver

本機 Markdown 目錄預覽伺服器 (local Markdown directory preview server) —
`目錄即 router、檔名即 page`，每次 request 即時 render，存檔後重新整理就看得到。

## 業務定義 (Business Definition)

給在本機寫 Markdown 的人用的零設定預覽工具。在任何放滿 `.md` 的資料夾裡直接執行
`mdserver`，它就把整個資料夾當成一個網站服務出去：不需要設定檔、不需要 build step、
不需要 front matter 宣告路由。編輯檔案 → 重新整理瀏覽器，就是完整的工作流程。

與靜態網站產生器 (static site generator) 的差別在於`沒有產生物`：
mdserver 不寫任何 HTML 到磁碟，也不維護 build cache，每個 request 當場把 `.md`
讀進來轉成 HTML。代價是每次 request 的轉換成本，換來的是零建置延遲與零陳舊輸出。

## 路由規則 (Routing)

資料夾是路由，檔名是頁面：

| 請求路徑 | 解析結果 |
| --- | --- |
| `/docs/setup` | 檔案 `docs/setup.md` 轉成 HTML |
| `/docs/` | 目錄 `docs/` 的索引列表；若有 `README.md` / `index.md` 則以它為該目錄的頁面 |
| `/logo.png` | 原樣輸出 (static file) |
| `/page.html` | 原樣輸出，`不`進 Markdown pipeline，也不套版型 |
| `/docs/setup.md` | 302 導向正規網址 `/docs/setup` |

路徑逃逸 (path escape) 一律拒絕：任何解析後落在服務根目錄之外的請求都被擋下。

## Domain Flow

```
mdserver [dir]
     │
     ▼
選 port (預設 8080，被佔用就往後找，最多 20 個)
     │
     ▼
HTTP request
     │
     ├─► site 解析路徑 → Target (PAGE / DIR / STATIC / REDIRECT / NOT_FOUND)
     │
     ├─► PAGE   → 讀檔 → render (GFM + frontmatter + alert + mermaid + 語法上色)
     │                 → 套 page 版型 → HTML
     ├─► DIR    → 列出目錄項目 (帶 TTL cache) → 索引頁
     ├─► STATIC → 直接輸出檔案
     └─► REDIRECT → 302 到正規網址
```

## 支援的 Markdown 語法

- GitHub Flavored Markdown 與腳註 (footnote)
- YAML frontmatter — 提供頁面標題與描述
- GitHub alert 區塊 (`> [!NOTE]`, `> [!WARNING]` 等)
- ```` ```mermaid ```` 圖表 — mermaid bundle 內嵌於 binary，離線可用
- 程式碼語法上色 (syntax highlighting)，主題 CSS 於啟動時產生
- 自動目錄 (table of contents) 與內部連結改寫

## 使用 (Usage)

```bash
mdserver                      # 服務目前目錄，自動挑 port
mdserver ./docs               # 服務指定目錄
mdserver -port 3000           # 指定 port
mdserver -addr 127.0.0.1:3000 # 指定完整位址
mdserver -title "My Notes"    # 指定站台標題 (預設為目錄名)
mdserver -quiet               # 只輸出 warning 與 error
```

## 開發 (Dev)

```bash
npm run dev     # go run .
npm test        # go test ./...
npm run build   # go build -o bin/mdserver .
npm run deploy  # go install .
npm run lint    # gofmt -l . && go vet ./...
```

技術脈絡與模組分層見 [`CLAUDE.md`](CLAUDE.md)，領域名詞見
[`docs/terminology.md`](docs/terminology.md)。
