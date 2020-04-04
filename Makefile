HASH := $(shell git rev-parse --short HEAD)
VER := $(shell git describe --abbrev=0)
COMMIT_DATE := $(shell git show -s --format=%ci ${HASH})
BUILD := (${HASH}) $(shell date '+%Y-%m-%d %H:%M:%S')

BIN := niltalk
STATIC := static/templates static/static:/static

.PHONY: build
build:
	go build -o ${BIN} -ldflags="-s -w -X 'main.buildVersion=${VER}' -X 'main.buildDate=${BUILD}'"
	stuffbin -a stuff -in ${BIN} -out ${BIN} ${STATIC}

.PHONY: run
run: build
	 ./${BIN}

.PHONY: deps
deps:
	go get -u github.com/knadh/stuffbin/...

.PHONY: test
test:
	go test

clean:
	go clean
	- rm -f ${BIN}
