SHELL=/bin/bash
CMD=elastic-etcd
GOBUILD=go build

default: all

all: check test build

.PHONY: $(CMD)
$(CMD):
	$(GOBUILD) github.com/sttts/elastic-etcd/cmd/$@

build: $(CMD)

test:
	for m in $$(go list ./... | grep -v /vendor/ | grep -v '/elastic-etcd$$'); do go test -v $$m -vmodule="*=6" --logtostderr; done

GFMT=find . -not \( \( -wholename "./vendor" \) -prune \) -name "*.go" | xargs gofmt -l
gofmt:
	@UNFMT=$$($(GFMT)); if [ -n "$$UNFMT" ]; then echo "gofmt needed on" $$UNFMT && exit 1; fi

fix:
	@UNFMT=$$($(GFMT)); if [ -n "$$UNFMT" ]; then echo "goimports -w" $$UNFMT; goimports -w $$UNFMT; fi

gometalinter:
	gometalinter \
		--vendor \
		--cyclo-over=13 \
		--tests \
		--deadline=120s \
		--dupl-threshold=53 \
		--disable=gotype --disable=aligncheck --disable=structcheck --disable=interfacer --disable=deadcode --disable=gocyclo --disable=dupl \
		./...

check: gofmt gometalinter

clean:
	rm -f $(CMD)

.PHONY: build test check
