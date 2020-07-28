DEPS:=filcrypto.h filcrypto.pc libfilcrypto.a

all: $(DEPS)
.PHONY: all

# Create a file so that parallel make doesn't call `./install-filcrypto` for
# each of the deps
$(DEPS): .install-filcrypto  ;

.install-filcrypto: rust
	./install-filcrypto
	@touch $@

clean:
	rm -rf $(DEPS) .install-filcrypto
	rm -f ./runner
	cd rust && cargo clean && cd ..
.PHONY: clean

go-lint: $(DEPS)
	golangci-lint run -v --concurrency 2 --new-from-rev origin/master
.PHONY: go-lint

shellcheck:
	shellcheck install-filcrypto

lint: shellcheck go-lint

cgo-leakdetect: runner
	valgrind --leak-check=full --show-leak-kinds=definite ./runner
.PHONY: cgo-leakdetect

cgo-gen: $(DEPS)
	c-for-go --ccincl --ccdefs --nostamp filcrypto.yml
.PHONY: cgo-gen

runner: $(DEPS)
	rm -f ./runner
	go build -o ./runner ./cgoleakdetect/
.PHONY: runner
