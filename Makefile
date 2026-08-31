.PHONY: all build build-frontend build-all test test-coverage lint run docker-build docker-up docker-down docker-logs clean btc-master-auto btc-research-loop-install btc-research-loop-run btc-research-loop-status btc-research-loop-uninstall

BINARY  := ./chart-analyzer
WEB_DIR := ./web

all: build

build:
	go build -o $(BINARY) ./cmd/server

build-frontend:
	cd $(WEB_DIR) && npm install && npm run build

build-all: build-frontend build

test:
	go test ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...

run: build
	$(BINARY)

btc-master-auto:
	bash ./scripts/btc-master-auto.sh

btc-research-loop-install:
	bash ./scripts/install-btc-research-loop-macos.sh

btc-research-loop-run:
	bash ./scripts/btc-research-poller.sh

btc-research-loop-status:
	@launchctl print gui/$$(id -u)/com.chartnagari.btc15m-research 2>/dev/null || echo "BTCUSDT/15M background research loop is not loaded."

btc-research-loop-uninstall:
	bash ./scripts/install-btc-research-loop-macos.sh uninstall

docker-build: build-frontend
	docker build -t ChartAnalysis:latest .

docker-up: build-frontend
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

clean:
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf $(WEB_DIR)/dist
