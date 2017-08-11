.PHONY: kube2consul clean test

kube2consul: kube2consul.go
	CGO_ENABLED=0 go build --ldflags '-extldflags "-static"'
	strip kube2consul

clean:
	rm -fr kube2consul

test: clean
	go test -v --vmodule=*=4
