VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install clean

build:
	go build -ldflags "$(LDFLAGS)" -o hangar .

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 hangar $(DESTDIR)$(PREFIX)/bin/hangar

clean:
	rm -f hangar
