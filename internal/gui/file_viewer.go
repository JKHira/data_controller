package gui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/trade-engine/data-controller/internal/sink/arrow"
)

// CreateFileViewerPanel creates the file viewer panel with pagination controls
func CreateFileViewerPanel(fileViewer *widget.Entry, prevBtn, nextBtn, closeBtn *widget.Button, pageLabel *widget.Label) fyne.CanvasObject {
	// File viewer with controls - use Border layout for proper expansion
	viewerControls := container.NewHBox(
		prevBtn,
		nextBtn,
		widget.NewSeparator(),
		pageLabel,
		widget.NewSeparator(),
		closeBtn,
	)
	viewerScroll := container.NewVScroll(fileViewer)
	viewerContent := container.NewBorder(
		viewerControls, // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		viewerScroll,   // center (takes remaining space)
	)
	return widget.NewCard("👁️ File Viewer", "", viewerContent)
}

// DisplayFileInfo displays basic file information in the viewer
func DisplayFileInfo(fileViewer *widget.Entry, filePath string) {
	content := fmt.Sprintf("📄 File: %s\n", filepath.Base(filePath))
	content += fmt.Sprintf("📁 Path: %s\n", filePath)
	content += fmt.Sprintf("🏷️ Type: %s\n\n", filepath.Ext(filePath))
	content += "🚫 This file type is not supported for data viewing.\n"

	fileViewer.SetText(content)
}

// DisplayArrowData displays Arrow file data with pagination information
func DisplayArrowData(fileViewer *widget.Entry, filePath string, summary map[string]interface{}, pageData *arrow.PageData) {
	content := fmt.Sprintf("📊 Arrow File: %s\n", filepath.Base(filePath))
	content += fmt.Sprintf("📏 File Size: %v bytes (%.2f MB)\n", summary["file_size"], float64(summary["file_size"].(int64))/(1024*1024))
	content += fmt.Sprintf("📈 Total Records: %v\n", summary["total_records"])
	content += fmt.Sprintf("📦 Batches: %v\n", summary["num_batches"])
	content += fmt.Sprintf("📝 Columns: %v\n", summary["num_columns"])
	content += fmt.Sprintf("📊 Data Read: %.2f MB / %.2f MB\n\n", float64(pageData.BytesRead)/(1024*1024), float64(pageData.TotalBytes)/(1024*1024))

	// Schema information (show only first few fields to save space)
	if fields, ok := summary["schema_fields"].([]map[string]string); ok {
		content += "🗃️ Schema (showing first 8 fields):\n"
		maxFields := min(8, len(fields))
		for i := 0; i < maxFields; i++ {
			field := fields[i]
			content += fmt.Sprintf("  • %s: %s\n", field["name"], field["type"])
		}
		if len(fields) > 8 {
			content += fmt.Sprintf("  ... and %d more fields\n", len(fields)-8)
		}
		content += "\n"
	}

	// Page data with 10MB chunk info
	content += fmt.Sprintf("📄 Chunk %d/%d (~10MB per chunk):\n", pageData.PageNumber, pageData.TotalPages)
	content += fmt.Sprintf("📊 Records in this chunk: %d\n", len(pageData.Records))
	content += fmt.Sprintf("💾 Bytes loaded: %.2f MB\n", float64(pageData.BytesRead)/(1024*1024))
	content += strings.Repeat("─", 80) + "\n\n"

	// Display records (limit to first 3000 for pagination)
	maxRecords := min(3000, len(pageData.Records))
	for i := 0; i < maxRecords; i++ {
		record := pageData.Records[i]
		content += fmt.Sprintf("🔢 Record #%d:\n", i+1)

		// Show important fields first
		importantFields := []string{"exchange", "symbol", "ts_micros", "price", "amount", "bid", "ask", "last"}
		for _, fieldName := range importantFields {
			if value, exists := record[fieldName]; exists {
				if value == nil {
					content += fmt.Sprintf("  %s: <null>\n", fieldName)
				} else {
					content += fmt.Sprintf("  %s: %v\n", fieldName, value)
				}
			}
		}

		// Show other fields (limit to save space)
		otherFieldCount := 0
		for key, value := range record {
			// Skip already shown fields
			isImportant := false
			for _, imp := range importantFields {
				if key == imp {
					isImportant = true
					break
				}
			}
			if isImportant {
				continue
			}

			if otherFieldCount < 3 { // Show max 3 additional fields
				if value == nil {
					content += fmt.Sprintf("  %s: <null>\n", key)
				} else {
					content += fmt.Sprintf("  %s: %v\n", key, value)
				}
				otherFieldCount++
			}
		}
		content += "\n"
	}

	if len(pageData.Records) > 3000 {
		content += fmt.Sprintf("... and %d more records in this chunk\n", len(pageData.Records)-3000)
		content += "💡 Use Previous/Next buttons to navigate through data\n"
	}

	fileViewer.SetText(content)
}

// DisplayFileSelectionInfo displays file information when selected (not double-clicked)
func DisplayFileSelectionInfo(fileViewer *widget.Entry, filePath string, info interface{}) {
	content := fmt.Sprintf("📄 File: %s\n", filepath.Base(filePath))
	content += fmt.Sprintf("📁 Path: %s\n", filePath)

	if fileInfo, ok := info.(interface {
		Size() int64
		ModTime() time.Time
	}); ok {
		content += fmt.Sprintf("📏 Size: %d bytes (%.2f MB)\n", fileInfo.Size(), float64(fileInfo.Size())/(1024*1024))
		content += fmt.Sprintf("🕒 Modified: %s\n", fileInfo.ModTime().Format(time.RFC3339))
	}

	content += fmt.Sprintf("🏷️ Type: %s\n\n", filepath.Ext(filePath))

	if strings.HasSuffix(filePath, ".arrow") {
		content += "📊 This is an Arrow file containing trading data.\n"
		content += "💡 Double-click to view contents (10MB chunks)\n"
		content += "🔍 File contains structured data with timestamps, prices, and volumes.\n"
	}

	fileViewer.SetText(content)
}
