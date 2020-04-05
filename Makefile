LAST_COMMIT := $(shell git rev-parse --short HEAD)
LAST_COMMIT_DATE := $(shell git show -s --format=%ci ${LAST_COMMIT})
VERSION := $(shell git describe)
BUILDSTR := ${VERSION} (${LAST_COMMIT} $(shell date -u +"%Y-%m-%dT%H:%M:%S%z"))

BIN := niltalk
STATIC := static/templates static/static:/static config.toml.sample

.PHONY: deps
deps:
	go get -u github.com/knadh/stuffbin/...

.PHONY: build
build:
	go build -o ${BIN} -ldflags="-s -w -X 'main.buildString=${BUILDSTR}'"

.PHONY: run
run: build
	 ./${BIN}

.PHONY: dist
dist: build
	# If dependencies are not installed, install.
	@type stuffbin >/dev/null 2>&1 || make deps
	stuffbin -a stuff -in ${BIN} -out ${BIN} ${STATIC}

# pack-releases runns stuffbin packing on a given list of
# binaries. This is used with goreleaser for packing
# release builds for cross-build targets.
.PHONY: pack-releases
pack-releases:
	$(foreach var,$(RELEASE_BUILDS),stuffbin -a stuff -in ${var} -out ${var} ${STATIC} $(var);)

.PHONY: test
test:
	go test

.PHONE: clean
clean:
	go clean
	- rm -f ${BIN}
