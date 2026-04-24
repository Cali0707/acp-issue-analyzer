BINARY := baton
BUILD_DIR := bin
CMD := ./cmd/$(BINARY)

.PHONY: build clean test lint

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

test:
	go test ./... -count=1

clean:
	rm -rf $(BUILD_DIR)

lint:
	go vet ./...
