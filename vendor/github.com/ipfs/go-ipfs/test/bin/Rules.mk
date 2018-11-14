include mk/header.mk

TGTS_$(d) :=

$(d)/pollEndpoint: thirdparty/pollEndpoint
	$(go-build)
TGTS_$(d) += $(d)/pollEndpoint

$(d)/go-sleep: test/dependencies/go-sleep
	$(go-build)
TGTS_$(d) += $(d)/go-sleep

$(d)/go-timeout: test/dependencies/go-timeout
	$(go-build)
TGTS_$(d) += $(d)/go-timeout

$(d)/ma-pipe-unidir: test/dependencies/ma-pipe-unidir
	$(go-build)
TGTS_$(d) += $(d)/ma-pipe-unidir

$(d)/json-to-junit: test/dependencies/json-to-junit
	$(go-build)
TGTS_$(d) += $(d)/json-to-junit

TGTS_GX_$(d) := hang-fds iptb
TGTS_GX_$(d) := $(addprefix $(d)/,$(TGTS_GX_$(d)))

$(TGTS_GX_$(d)):
	go build -i $(go-flags-with-tags) -o "$@" "$(call gx-path,$(notdir $@))"

TGTS_$(d) += $(TGTS_GX_$(d))

# multihash is special
$(d)/multihash:
	go build -i $(go-flags-with-tags) -o "$@" "gx/ipfs/$(shell gx deps find go-multihash)/go-multihash/multihash"
TGTS_$(d) += $(d)/multihash

# cid-fmt is also special
$(d)/cid-fmt:
	go build -i $(go-flags-with-tags) -o "$@" "gx/ipfs/$(shell gx deps find go-cidutil)/go-cidutil/cid-fmt"
TGTS_$(d) += $(d)/cid-fmt

# random is also special
$(d)/random:
	go build -i $(go-flags-with-tags) -o "$@" "gx/ipfs/$(shell gx deps find go-random)/go-random/random"
TGTS_$(d) += $(d)/random

# random-files is also special
$(d)/random-files:
	go build -i $(go-flags-with-tags) -o "$@" "gx/ipfs/$(shell gx deps find go-random-files)/go-random-files/random-files"
TGTS_$(d) += $(d)/random-files


$(TGTS_$(d)): $$(DEPS_GO)

CLEAN += $(TGTS_$(d))

PATH := $(realpath $(d)):$(PATH)

include mk/footer.mk
