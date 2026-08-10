# mdserver — 術語表 (Terminology)

本檔是領域名詞、狀態值與縮寫的單一定義來源。CLI、程式碼與文件使用同一組正名。

## 路由與解析 (Routing and Resolution)

| 術語 (Term) | 英文 (English) | 定義 (Definition) | 出處 (Source) |
| --- | --- | --- | --- |
| 站台根目錄 | Site Root | 被服務的那一個目錄；所有解析結果都必須落在它之內 | `site.Site.Root` |
| 目標 | Target | 一個 request 路徑的解析結果，含種類、磁碟路徑與正規網址 | `site.Target` |
| 目標種類 | Kind | 目標的分類，值為 `KIND_PAGE` / `KIND_DIR` / `KIND_STATIC` / `KIND_REDIRECT` / `KIND_NOT_FOUND` | `site.Kind` |
| 正規網址 | Canonical URL | 一個目標唯一正確的網址；請求了非正規形式（例如帶 `.md`）會被 302 導向 | `site.Target.URLPath` |
| 索引檔 | Index File | 代表其所在目錄的 Markdown 檔；優先序為 `README.md`、`index.md`、`readme.md`、`Index.md` | `site.INDEX_FILENAMES` |
| 目錄列表 | Listing | 一個目錄下可導覽項目的列舉結果，帶 TTL 快取 | `site.Entry`、`site/listing.go` |
| 根目錄逃逸 | Root Escape | 解析後落在站台根目錄之外的路徑；一律拒絕 | `site.ErrOutsideRoot` |

## 轉換管線 (Render Pipeline)

| 術語 (Term) | 英文 (English) | 定義 (Definition) | 出處 (Source) |
| --- | --- | --- | --- |
| 文件 | Document | 一個 Markdown 檔轉換後的完整結果，含 HTML、標題、描述、frontmatter 與標題清單 | `render.Document` |
| 轉換器 | Renderer | 組裝好的 goldmark pipeline；可並行使用 | `render.Renderer` |
| 前置資料 | Frontmatter | 檔首 YAML 區塊，提供頁面標題與描述 | `render.Document.Meta`、`site/pagemeta.go` |
| 提示區塊 | Alert | GitHub 風格的 `> [!NOTE]` / `> [!WARNING]` 等強調區塊 | `render/alert.go` |
| 圖表區塊 | Mermaid Block | ```` ```mermaid ```` 圍籬區塊；由內嵌的 mermaid bundle 在前端繪製 | `render/mermaid.go`、`web/vendor/mermaid.min.js` |
| 目錄 | Table of Contents (TOC) | 從標題節點收集而成的頁內導覽結構 | `render/toc.go`、`render.Heading` |
| 語法上色 CSS | Syntax CSS | 啟動時由 chroma 主題產生的樣式表，不以檔案形式存在 | `render.SyntaxCSS()`、`server.SYNTAX_CSS_PATH` |
| 轉換器優先序 | Transformer Priority | AST transformer 的執行順序；數字小者先跑，alert 與 mermaid 必須早於 render | `render.PRIORITY_TRANSFORM_*` |

## 服務層 (Serving)

| 術語 (Term) | 英文 (English) | 定義 (Definition) | 出處 (Source) |
| --- | --- | --- | --- |
| 內嵌資產 | Embedded Asset | 以 `go:embed` 打包進 binary 的 CSS / JS / mermaid bundle | `web/assets.go` |
| 資產前綴 | Asset Prefix | 內嵌資產的網址命名空間 `/_mdserver/`；以底線開頭確保不與真實目錄撞名 | `server.ASSET_PREFIX` |
| 頁面版型 | Page Template | 包住轉換結果的 HTML 外框 | `web/page.html`、`web.PAGE_TEMPLATE` |
| Port 遞增掃描 | Port Scan | 預設 port 被佔用時往後尋找可用 port 的行為，上限由 `PORT_SCAN_LIMIT` 界定 | `main.go` |

## 明確排除的用語 (Non-terms)

以下概念`不`存在於本專案，出現在文件中即為錯誤：

- `建置 (build)` / `產生物 (artifact)` — mdserver 不寫任何 HTML 到磁碟。
- `設定檔 (config file)` — 路由規則就是全部的設定，沒有設定檔。
- `頁面快取 (page cache)` — 唯一的快取是目錄列表的 TTL 快取，轉換結果從不快取。
