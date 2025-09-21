# Data Controller Makefile
# Apache Arrow Go implementation

.PHONY: build build-gui clean test run run-gui help deps

# Default target
all: build

# Variables
BINARY_NAME=data-controller
BINARY_GUI=data-controller-gui
SRC_DIR=cmd/data-controller
MAIN_FILES=$(SRC_DIR)/main.go $(SRC_DIR)/nogui.go $(SRC_DIR)/gui.go
STUB_GUI=$(SRC_DIR)/stub_gui.go
FYNE_GUI=$(SRC_DIR)/fyne_gui.go

# ヘッドレス版ビルド（デフォルト）
build:
	@echo "Building headless version..."
	go build -o $(BINARY_NAME) $(MAIN_FILES) $(STUB_GUI)
	@echo "✅ Built: $(BINARY_NAME)"

# GUI版ビルド（Fyne GUI付き）
build-gui:
	@echo "Building GUI version with Fyne..."
	CC=clang CXX=clang++ go build -tags gui -o $(BINARY_GUI) $(MAIN_FILES) $(FYNE_GUI)
	@echo "✅ Built: $(BINARY_GUI)"

# 依存関係のダウンロード
deps:
	@echo "Downloading dependencies..."
	go mod tidy
	go mod download
	@echo "✅ Dependencies updated"

# テスト実行
test:
	@echo "Running tests..."
	go test ./...
	@echo "✅ Tests completed"

# ヘッドレス版実行（ノーGUIモード）
run:
	@echo "Running headless version..."
	./$(BINARY_NAME) -nogui

# GUI版実行
run-gui:
	@echo "Running GUI version..."
	./$(BINARY_GUI)

# Terminal GUI版実行
run-terminal:
	@echo "Running terminal GUI version..."
	./$(BINARY_NAME)

# クリーンアップ
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME) $(BINARY_GUI)
	rm -rf data/bitfinex/*/
	@echo "✅ Cleaned up binaries and data"

# 開発用クリーンアップ（データは保持）
clean-build:
	@echo "Cleaning up binaries only..."
	rm -f $(BINARY_NAME) $(BINARY_GUI)
	@echo "✅ Cleaned up binaries"

# データディレクトリの確認
check-data:
	@echo "Checking data directory..."
	@find data -name "*.arrow*" -type f -exec ls -lah {} \; 2>/dev/null || echo "No Arrow files found"

# ヘルプ
help:
	@echo "Data Controller - Available targets:"
	@echo ""
	@echo "  build         - Build headless version (default)"
	@echo "  build-gui     - Build GUI version with Fyne"
	@echo "  deps          - Download and update dependencies"
	@echo "  test          - Run tests"
	@echo "  run           - Run headless version (-nogui)"
	@echo "  run-gui       - Run GUI version"
	@echo "  run-terminal  - Run terminal GUI version"
	@echo "  clean         - Clean binaries and data"
	@echo "  clean-build   - Clean binaries only"
	@echo "  check-data    - Check Arrow data files"
	@echo "  help          - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make build         # Build headless version"
	@echo "  make build-gui     # Build GUI version"
	@echo "  make run           # Run headless data collection"
	@echo "  make clean         # Clean everything"