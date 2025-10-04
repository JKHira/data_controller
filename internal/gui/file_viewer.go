package gui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/trade-engine/data-controller/internal/sink/arrow"
)

// CreateFileViewerPanel creates the file viewer panel with pagination controls
func CreateFileViewerPanel(fileViewer, metadataViewer *widget.Entry, prevBtn, nextBtn, closeBtn *widget.Button, pageLabel *widget.Label) fyne.CanvasObject {
	// File viewer with controls - use Border layout for proper expansion
	viewerControls := container.NewHBox(
		prevBtn,
		nextBtn,
		widget.NewSeparator(),
		pageLabel,
		widget.NewSeparator(),
		closeBtn,
	)
	metadataScroll := container.NewVScroll(metadataViewer)
	metadataScroll.SetMinSize(fyne.NewSize(220, 220))
	metadataCard := widget.NewCard("📑 Metadata", "", metadataScroll)
	viewerScroll := container.NewVScroll(fileViewer)
	contentBody := container.NewBorder(metadataCard, nil, nil, nil, viewerScroll)
	viewerContent := container.NewBorder(
		viewerControls, // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		contentBody,
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
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("📊 Arrow File: %s\n", filepath.Base(filePath)))
	if size, ok := summary["file_size"].(int64); ok {
		builder.WriteString(fmt.Sprintf("📏 File Size: %d bytes (%.2f MB)\n", size, float64(size)/(1024*1024)))
	}
	if val, ok := summary["total_records"]; ok {
		builder.WriteString(fmt.Sprintf("📈 Total Records: %v\n", val))
	}
	builder.WriteString(fmt.Sprintf("📊 Data Read: %.2f MB / %.2f MB\n", float64(pageData.BytesRead)/(1024*1024), float64(pageData.TotalBytes)/(1024*1024)))
	builder.WriteString(fmt.Sprintf("📄 Chunk %d/%d (~10MB per chunk)\n", pageData.PageNumber, pageData.TotalPages))
	builder.WriteString(fmt.Sprintf("📊 Records in this chunk: %d\n", len(pageData.Records)))
	builder.WriteString(fmt.Sprintf("💾 Bytes loaded: %.2f MB\n", float64(pageData.BytesRead)/(1024*1024)))
	builder.WriteString(strings.Repeat("─", 80))
	builder.WriteString("\n\n")

	fieldOrder := fieldOrderFromSummary(summary)
	if len(pageData.FieldNames) > 0 {
		fieldOrder = copyStringSlice(pageData.FieldNames)
	}
	if len(fieldOrder) == 0 && len(pageData.Records) > 0 {
		fieldOrder = deriveFieldOrder(pageData.Records[0])
	}

	maxRecords := len(pageData.Records)
	if maxRecords > 3000 {
		maxRecords = 3000
	}
	for i := 0; i < maxRecords; i++ {
		record := pageData.Records[i]
		builder.WriteString(fmt.Sprintf("🔢 Record #%d:\n", i+1))
		for _, fieldName := range fieldOrder {
			value, exists := record[fieldName]
			if !exists {
				continue
			}
			if value == nil {
				builder.WriteString(fmt.Sprintf("  %s: <null>\n", fieldName))
			} else {
				builder.WriteString(fmt.Sprintf("  %s: %v\n", fieldName, value))
			}
		}
		builder.WriteString("\n")
	}

	if len(pageData.Records) > maxRecords {
		builder.WriteString(fmt.Sprintf("... and %d more records in this chunk\n", len(pageData.Records)-maxRecords))
		builder.WriteString("💡 Use Previous/Next buttons to navigate through data\n")
	}

	fileViewer.SetText(builder.String())
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

func fieldOrderFromSummary(summary map[string]interface{}) []string {
	if summary == nil {
		return nil
	}
	fields, ok := summary["schema_fields"].([]map[string]string)
	if !ok {
		return nil
	}
	order := make([]string, 0, len(fields))
	for _, field := range fields {
		if name, ok := field["name"]; ok {
			order = append(order, name)
		}
	}
	return order
}

func deriveFieldOrder(record map[string]interface{}) []string {
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func copyStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
