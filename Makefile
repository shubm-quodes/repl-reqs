VERSION := 1.0.0
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# If OMIT_SYS_CMDS=1 is supplied system commands will not be registered. Pass this flag for provider builds
OMIT_SYSTEM_COMMANDS := $(if $(OMIT_SYS_CMDS),true,false)

LDFLAGS := -ldflags "\
	-X main.version=$(VERSION) \
	-X main.buildDate=$(BUILD_DATE) \
	-X main.omitSystemCommands=$(OMIT_SYSTEM_COMMANDS) \
"

BUILD_DIR := bin

.PHONY: build linux windows mac all clean dirs

dirs:
	mkdir -p $(BUILD_DIR)

build: dirs
	go build $(LDFLAGS) -o $(BUILD_DIR)/repl-reqs repl.go

linux: dirs
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/repl-reqs-linux-amd64 repl.go

windows: dirs
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/repl-reqs-windows-amd64.exe repl.go

mac: dirs
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/repl-reqs-darwin-amd64 repl.go
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/repl-reqs-darwin-arm64 repl.go

all: clean linux windows mac build

clean:
	rm -rf $(BUILD_DIR)
