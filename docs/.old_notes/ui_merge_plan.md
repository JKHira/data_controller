# UI統合 修正計画書

## 作成日: 2025-10-04
## 目的: File LoaderとFile Viewerを統合し、レイアウトを最適化

---

## 現在のレイアウト

**app.go:245-251**:
```go
columns := container.New(layout.NewHBoxLayout(),
    wrapColumn(wsPane, 380),        // WebSocket
    wrapColumn(restPane, 380),      // REST API
    wrapColumn(filesCard, 380),     // File Loader
    wrapColumn(fileViewerCard, 380),// File Viewer
    wrapColumn(controlPanel, 780),  // Controls
)
```

**合計幅**: 380 × 4 + 780 = 2300px

---

## 要求される変更

### 1. File LoaderとFile Viewerを統合
- 1つのパネルにまとめる
- タブ形式で分ける or 上下配置

### 2. 幅の調整
- **統合パネル**: 380px (File Loader幅を維持)
- **Controls Panel**: 1160px (780 + 380 = File Viewer分を追加)

### 3. File Viewerにメタデータ表示追加
- Previous/Nextボタンの下に正方形エリア
- ロードしたファイルのメタデータを表示:
  - ファイル名
  - サイズ
  - レコード数
  - スキーマ情報
  - 作成日時

---

## 新しいレイアウト

```go
columns := container.New(layout.NewHBoxLayout(),
    wrapColumn(wsPane, 380),                    // WebSocket
    wrapColumn(restPane, 380),                  // REST API
    wrapColumn(mergedFilesViewerCard, 380),     // File Loader + Viewer (統合)
    wrapColumn(expandedControlPanel, 1160),     // Controls (拡張)
)
```

**合計幅**: 380 × 3 + 1160 = 2300px (変更なし)

---

## 実装手順

### Phase 1: 統合パネルの作成

**新規ファイル**: `internal/gui/panels/files_viewer_panel.go`

```go
type FilesViewerPanel struct {
    logger         *zap.Logger
    cfg            *config.Config
    appState       *state.AppState
    fileController *controllers.FileController
    window         fyne.Window

    // File Loader components
    scanButton     *widget.Button
    fileList       *widget.List
    // ...

    // File Viewer components
    metadataCard   *widget.Card    // 新規: メタデータ表示
    prevButton     *widget.Button
    nextButton     *widget.Button
    dataTable      *widget.Table
    // ...
}

func NewFilesViewerPanel(...) *FilesViewerPanel {
    // File LoaderとFile Viewerのコンポーネントを統合
}

func (p *FilesViewerPanel) Build() fyne.CanvasObject {
    // 上部: File Loader (scan, filter, file list)
    // 中部: メタデータ表示
    // 下部: File Viewer (prev/next, data table)

    return container.NewVBox(
        loaderSection,
        metadataSection,  // 新規
        viewerSection,
    )
}
```

---

### Phase 2: app.goの修正

**修正箇所**: [app.go:104-106, 235-251](internal/gui/app/app.go)

1. FilesPanel と ViewerPanel を削除
2. FilesViewerPanel を追加
3. レイアウトを調整

```go
// NewApplication()内
filesViewerPanel := panels.NewFilesViewerPanel(logger, cfg, appState, fileController, window)

// createLayout()内
mergedCard := filesViewerPanel.GetContent()
expandedControlPanel := widget.NewCard("Controls", "", container.NewVBox())

columns := container.New(layout.NewHBoxLayout(),
    wrapColumn(wsPane, 380),
    wrapColumn(restPane, 380),
    wrapColumn(mergedCard, 380),
    wrapColumn(expandedControlPanel, 1160),
)
```

---

### Phase 3: メタデータ表示の実装

**メタデータ情報**:
```go
type FileMetadata struct {
    FileName    string
    FilePath    string
    FileSize    int64
    RecordCount int
    Schema      string
    CreatedAt   time.Time
    Channel     string
    Symbol      string
    Exchange    string
}
```

**表示レイアウト**:
```
┌─────────────────────────────┐
│ File Metadata               │
├─────────────────────────────┤
│ File: ticker-20251004...    │
│ Size: 2.5 MB                │
│ Records: 15,234             │
│ Channel: ticker             │
│ Symbol: tBTCEUR             │
│ Created: 2025-10-04 13:06   │
│ Schema: [timestamp, price...│
└─────────────────────────────┘
```

---

## 影響範囲

### 変更が必要なファイル
- ✅ `internal/gui/panels/files_viewer_panel.go` (新規作成)
- ✅ `internal/gui/app/app.go` (レイアウト修正)
- ✅ `internal/gui/panels/files_panel.go` (非推奨 or 削除)
- ✅ `internal/gui/panels/viewer_panel.go` (非推奨 or 削除)

### 影響を受けないもの
- FileController (そのまま使用)
- ArrowReader (そのまま使用)
- State管理 (そのまま使用)

---

## テスト計画

1. ビルド確認
2. UI表示確認:
   - File Loader部分が正常に表示
   - メタデータエリアが表示
   - File Viewer部分が正常に表示
3. 機能確認:
   - Scanボタン動作
   - ファイル選択 → メタデータ表示
   - Load → データ表示
   - Prev/Next動作
4. レイアウト確認:
   - 統合パネル幅380px
   - Controls幅1160px
   - 全体幅2300px維持

---

## 備考

この統合により:
- 画面スペースの効率化
- ファイル操作の一元化
- メタデータの即時確認が可能
- Controls領域の拡大による将来の機能拡張余地

作業が複雑なため、段階的に実装することを推奨。
