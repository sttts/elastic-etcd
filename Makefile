bin/etcd:
	mkdir -p bin
	cd bin && go build github.com/coreos/etcd

test:
	cd e2e && go test -v -vmodule="*=6" --logtostderr -run elastic ./.
