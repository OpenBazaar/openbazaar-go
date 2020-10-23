package traversal

import (
	"context"
	"fmt"
	"io"

	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/schema"
)

// init sets all the values in TraveralConfig to reasonable defaults
// if they're currently the zero value.
//
// Note that you're absolutely going to need to replace the
// LinkLoader and LinkNodeBuilderChooser if you want automatic link traversal;
// the defaults return error and/or panic.
func (tc *Config) init() {
	if tc.Ctx == nil {
		tc.Ctx = context.Background()
	}
	if tc.LinkLoader == nil {
		tc.LinkLoader = func(ipld.Link, ipld.LinkContext) (io.Reader, error) {
			return nil, fmt.Errorf("no link loader configured")
		}
	}
	if tc.LinkTargetNodePrototypeChooser == nil {
		tc.LinkTargetNodePrototypeChooser = func(lnk ipld.Link, lnkCtx ipld.LinkContext) (ipld.NodePrototype, error) {
			if tlnkNd, ok := lnkCtx.LinkNode.(schema.TypedLinkNode); ok {
				return tlnkNd.LinkTargetNodePrototype(), nil
			}
			return nil, fmt.Errorf("no LinkTargetNodePrototypeChooser configured")
		}
	}
	if tc.LinkStorer == nil {
		tc.LinkStorer = func(ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
			return nil, nil, fmt.Errorf("no link storer configured")
		}
	}
}

func (prog *Progress) init() {
	if prog.Cfg == nil {
		prog.Cfg = &Config{}
	}
	prog.Cfg.init()
}
