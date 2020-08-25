LAST_COMMIT := $(shell git rev-parse --short HEAD)
LAST_COMMIT_DATE := $(shell git show -s --format=%ci ${LAST_COMMIT})
VERSION := $(shell git describe)
BUILDSTR := ${VERSION} (${LAST_COMMIT} $(shell date -u +"%Y-%m-%dT%H:%M:%S%z"))

BIN := niltalk

.PHONY: deps
deps:
	# If dependencies are not installed, install.
	go get -u github.com/GeertJohan/go.rice/rice

.PHONY: build
build:
	go build -o ${BIN} -tags prod -ldflags="-s -w -X 'main.buildString=${BUILDSTR}'"

.PHONY: run
run: build
	 ./${BIN}

.PHONY: dist
dist: build deps
	rice append --exec ${BIN}

# pack-releases runs rice packing on a given list of
# binaries. This is used with goreleaser for packing
# release builds for cross-build targets.
.PHONY: pack-releases
pack-releases: deps
	$(foreach var,$(RELEASE_BUILDS),rice --append $(var);)

.PHONY: test
test:
	go test

.PHONE: clean
clean:
	go clean
	- rm -f ${BIN}
