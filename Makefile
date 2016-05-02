SHELL=/bin/bash
CMD=elastic-etcd
GOBUILD=go build
REPOSITORY=sttts
VERSION=$(shell git describe --always --tags --dirty)
DOCKER_TAG=$$(grep "^FROM" Dockerfile | sed 's/.*etcd://')

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
		--disable=gotype --disable=aligncheck --disable=unconvert --disable=structcheck --disable=interfacer --disable=deadcode --disable=gocyclo --disable=dupl \
		./...

check: gofmt gometalinter

clean:
	rm -f $(CMD) docker/elastic-etcd

.PHONY: release/elastic-etcd
release/elastic-etcd:
	mkdir -p release
	cd release && GOOS=linux go build github.com/sttts/elastic-etcd/cmd/elastic-etcd
.PHONY: release
release: release/elastic-etcd
	go get github.com/aktau/github-release
	github-release upload -u sttts --repo elastic-etcd --tag $(VERSION) --file release/elastic-etcd --name elastic-etcd

.PHONY: docker
docker: release/elastic-etcd
	docker build -t $(REPOSITORY)/elastic-etcd:$(DOCKER_TAG) .

push: docker
	docker push $(REPOSITORY)/elastic-etcd:$(DOCKER_TAG)

.PHONY: build test check
