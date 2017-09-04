.PHONY: kude2msb clean test

kube2msb: kube2msb.go
	CGO_ENABLED=0 go build --ldflags '-extldflags "-static"'
	strip kube2msb

clean:
	rm -fr kube2msb

test: clean
	go test -v --vmodule=*=4
