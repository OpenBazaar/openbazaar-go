gx:
	go get github.com/whyrusleeping/gx
	go get github.com/whyrusleeping/gx-go

testutils:
	go get golang.org/x/tools/cmd/cover
	go get github.com/onsi/ginkgo/ginkgo
	go get github.com/onsi/gomega

deps: gx testutils
	gx --verbose install --global
	gx-go rewrite
