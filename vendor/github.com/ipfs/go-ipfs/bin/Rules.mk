include mk/header.mk

dist_root_$(d)="/ipfs/QmPrXH9jRVwvd7r5MC5e6nV4uauQGzLk1i2647Ye9Vbbwe"

$(d)/gx: $(d)/gx-v0.14.0
$(d)/gx-go: $(d)/gx-go-v1.9.0

TGTS_$(d) := $(d)/gx $(d)/gx-go
DISTCLEAN += $(wildcard $(d)/gx-v*) $(wildcard $(d)/gx-go-v*) $(d)/tmp

PATH := $(realpath $(d)):$(PATH)

$(TGTS_$(d)):
	rm -f $@$(?exe)
ifeq ($(WINDOWS),1)
	cp $^$(?exe) $@$(?exe)
else
	ln -s $(notdir $^) $@
endif

bin/gx-v%:
	@echo "installing gx $(@:bin/gx-%=%)"
	bin/dist_get $(dist_root_bin) gx $@ $(@:bin/gx-%=%)

bin/gx-go-v%:
	@echo "installing gx-go $(@:bin/gx-go-%=%)"
	@bin/dist_get $(dist_root_bin) gx-go $@ $(@:bin/gx-go-%=%)

CLEAN += $(TGTS_$(d))
include mk/footer.mk
