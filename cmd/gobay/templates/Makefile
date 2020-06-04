#
# Tools
#

GO = go
GOFMT = gofmt
GOCI = golangci-lint
GIT = git
DOCKER = docker
PROTOC = protoc

#
# Temporary dirs
#

DIR_BUILD = .build
DIR_DIST = .dist
DIR_TEST = .test

$(DIR_TEST):
	mkdir -p $(DIR_TEST)

$(DIR_BUILD):
	mkdir -p $(DIR_BUILD)

$(DIR_DIST):
	mkdir -p $(DIR_DIST)

#
# Cleanup
#

.PHONY: clean

clean:
	rm -rf $(DIR_DIST)
	rm -rf $(DIR_BUILD)
	rm -rf $(DIR_TEST)

#
# global flags
#

# app binary output name
BIN ?= app

# app main source file
BIN_SRC ?= cmd/main.go

# build/test environment tag
GOBAY_ENV ?= development

# args for go test -run or go test -bench
RUN ?= .
COMMAND ?=
# app run or test run arguments (flags)
ARGS ?=
# packages to run test/benchmark
PKG ?= ./...

.PHONY: build_debug build_release run

FILE_DEBUG_BIN = $(DIR_BUILD)/$(BIN)
FILE_RELEASE_BIN = $(DIR_DIST)/$(BIN)

#
# Version and commit info
#

GOVERSION = $(shell $(GO) version | cut -d\  -f3)
COMMIT = $(shell $(GIT) rev-parse HEAD 2>/dev/null | tail -c 16)
BRANCH = $(shell $(GIT) rev-parse --abbrev-ref HEAD 2>/dev/null)
VERSION = $(shell $(GIT) describe --tags 2>/dev/null)

ifeq ($(COMMIT),HEAD)
	COMMIT = none
else ifeq ($(COMMIT),)
	COMMIT = none
endif

ifeq ($(BRANCH),HEAD)
	BRANCH = none
else ifeq ($(BRANCH),)
	BRANCH = none
endif

ifeq ($(VERSION),)
	VERSION = none
endif

LDFLAGS := \
	-X main.branch=$(BRANCH) \
	-X main.commit=$(COMMIT) \
	-X main.version=$(VERSION) \
	-X main.goVersion=$(GOVERSION)

DEBUG_FLAGS := -ldflags='$(LDFLAGS)' -gcflags "all=-N -l"

RELEASE_FLAGS := -ldflags='-w -s $(LDFLAGS)'

build_debug: $(DIR_BUILD)
	$(GO) build $(DEBUG_FLAGS) -o $(FILE_DEBUG_BIN) $(BIN_SRC)

run: build_debug
	./$(FILE_DEBUG_BIN) $(COMMAND) $(ARGS)

build_release: $(DIR_DIST)
	$(GO) build $(RELEASE_FLAGS) -o $(FILE_RELEASE_BIN) $(BIN_SRC)

#
# Test and coverage
#

.PHONY: test coverage

FILE_TEST_BIN = $(DIR_TEST)/$(BIN).test
FILE_COVERAGE = $(DIR_TEST)/coverage.txt

# fail coverage report under xx percent
COVERAGE_FAIL_UNDER ?= 30

TEST_FLAGS := \
	-v -race -failfast -p=1 \
	-covermode=atomic \

test: $(DIR_TEST)
	for package in $$(go list ./... | grep -v gen); do \
		coverfile="$(DIR_TEST)/$$(echo $$package | tr / -).cover"; \
		$(GO) test $(TEST_FLAGS) -coverprofile="$$coverfile" -coverpkg=./app/... "$$package"; \
	done
	gocovmerge $(DIR_TEST)/*.cover > $(FILE_COVERAGE)


# Open up a browser to view coverage report
coverage: test
	$(GO) tool cover -html=$(FILE_COVERAGE)

coverage_fail: test
	$(GO) tool cover -func $(FILE_COVERAGE) | tee $(DIR_TEST)/cover.txt
	GOBAY_ENV=testing test `tail -1 $(DIR_TEST)/cover.txt | cut -d\) -f2 | tr -d '[:space:]|%' | cut -d. -f1` -ge $(COVERAGE_FAIL_UNDER)

#
# Benchmark
#

.PHONY: benchmark profile_cpu profile_mem profile_block profile_trace \
		profile_all_start profile_all_stop

FILE_BLOCK_PROFILE = $(DIR_TEST)/blockprofile.out
FILE_CPU_PROFILE = $(DIR_TEST)/cpuprofile.out
FILE_MEM_PROFILE = $(DIR_TEST)/memprofile.out
FILE_MUTEX_PROFILE = $(DIR_TEST)/mutexprofile.out
FILE_TRACE_PROFILE = $(DIR_TEST)/trace.out

FILE_CPU_PROFILE_PID = $(DIR_TEST)/cpuprofile.pid
FILE_MEM_PROFILE_PID = $(DIR_TEST)/memprofile.pid
FILE_BLOCK_PROFILE_PID = $(DIR_TEST)/blockprofile.pid
FILE_MUTEX_PROFILE_PID = $(DIR_TEST)/mutexprofile.pid
FILE_TRACE_PROFILE_PID = $(DIR_TEST)/trace.pid

BENCH_FLAGS := \
	-v -race -benchmem \
	-trace=$(FILE_TRACE_PROFILE) \
	-blockprofile=$(FILE_BLOCK_PROFILE) \
	-cpuprofile=$(FILE_CPU_PROFILE) \
	-memprofile=$(FILE_MEM_PROFILE) \
	-mutexprofile=$(FILE_MUTEX_PROFILE) \
	-o=$(FILE_TEST_BIN) \

benchmark: $(DIR_TEST)
	$(GO) test $(BENCH_FLAGS) -run=Benchmark.* -bench=$(RUN) $(PKG)

profile_cpu:
	$(GO) tool pprof -http localhost:50080 $(FILE_TEST_BIN) $(FILE_CPU_PROFILE)

profile_mem:
	$(GO) tool pprof -http localhost:50081 $(FILE_TEST_BIN) $(FILE_MEM_PROFILE)

profile_block:
	$(GO) tool pprof -http localhost:50082 $(FILE_TEST_BIN) $(FILE_BLOCK_PROFILE)

profile_mutex:
	$(GO) tool pprof -http localhost:50083 $(FILE_TEST_BIN) $(FILE_MUTEX_PROFILE)

profile_trace:
	$(GO) tool trace -http localhost:50084 $(FILE_TEST_BIN) $(FILE_TRACE_PROFILE)

profile_all_start:
	$(GO) tool pprof -http localhost:50080 $(FILE_TEST_BIN) $(FILE_CPU_PROFILE) \
		& echo $$! > $(FILE_CPU_PROFILE_PID)
	$(GO) tool pprof -http localhost:50081 $(FILE_TEST_BIN) $(FILE_MEM_PROFILE) \
		& echo $$! > $(FILE_MEM_PROFILE_PID)
	$(GO) tool pprof -http localhost:50082 $(FILE_TEST_BIN) $(FILE_BLOCK_PROFILE) \
		& echo $$! > $(FILE_BLOCK_PROFILE_PID)
	$(GO) tool pprof -http localhost:50083 $(FILE_TEST_BIN) $(FILE_MUTEX_PROFILE) \
		& echo $$! > $(FILE_MUTEX_PROFILE_PID)
	$(GO) tool trace -http localhost:50084 $(FILE_TEST_BIN) $(FILE_TRACE_PROFILE) \
		& echo $$! > $(FILE_TRACE_PROFILE_PID)

profile_all_stop: \
	$(FILE_BLOCK_PROFILE_PID) $(FILE_CPU_PROFILE_PID) \
	$(FILE_MEM_PROFILE_PID) $(FILE_MUTEX_PROFILE_PID) \
	$(FILE_TRACE_PROFILE_PID)

	kill -KILL `cat $(FILE_BLOCK_PROFILE_PID)`
	kill -KILL `cat $(FILE_CPU_PROFILE_PID)`
	kill -KILL `cat $(FILE_MEM_PROFILE_PID)`
	kill -KILL `cat $(FILE_MUTEX_PROFILE_PID)`
	kill -KILL `cat $(FILE_TRACE_PROFILE_PID)`

	rm -f $(FILE_BLOCK_PROFILE_PID)
	rm -f $(FILE_CPU_PROFILE_PID)
	rm -f $(FILE_MEM_PROFILE_PID)
	rm -f $(FILE_MUTEX_PROFILE_PID)
	rm -f $(FILE_TRACE_PROFILE_PID)

#
# Format
#

.PHONY: fmt lint

# check code style in these directories
FMT_DIRS = app cmd lib test

fmt:
	$(GOFMT) -d -s -w $(FMT_DIRS)

lint:
	$(GOCI) run
	swagger validate  $(SWAGGER_SPEC)

#
# Dependency management
#

.PHONY: tidy ensure

tidy:
	$(GO) mod tidy

ensure:
	$(GO) mod download

#
# Utility
#

.PHONY: genswagger

APP ?= $(BIN)
SWAGGER_SPEC ?= spec/openapi/main.yml
SWAGGER_TARGET ?= gen/openapi

genswagger:
	swagger generate server -f $(SWAGGER_SPEC) -A $(APP) -t $(SWAGGER_TARGET) --exclude-main

# ent

.PHONY: entdesc

DIR_SCHEMA = ./spec/schema/
DIR_SCHEMA_GEN = ./gen/schema/
DIR_SCHEMA_TMPL = ./spec/enttmpl
EMPTY_SCHEMA_NAME = schema.go
EMPTY_SCHEMA_PATH = $(DIR_SCHEMA_GEN)$(EMPTY_SCHEMA_NAME)

$(DIR_SCHEMA):
	mkdir -p $(DIR_SCHEMA)

$(DIR_SCHEMA_GEN):
	mkdir -p $(DIR_SCHEMA_GEN)

$(EMPTY_SCHEMA_PATH): $(DIR_SCHEMA_GEN)
	echo "package schema" > $(EMPTY_SCHEMA_PATH)

entinit:
	entc init --target $(DIR_SCHEMA) $(ARGS)

entgen: $(EMPTY_SCHEMA_PATH)
	entc generate --target $(DIR_SCHEMA_GEN) --template $(DIR_SCHEMA_TMPL) $(DIR_SCHEMA)

entdesc:
	entc describe $(DIR_SCHEMA)
