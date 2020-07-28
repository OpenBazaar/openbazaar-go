package blockio

import (
	"context"
	"errors"
	"io"

	"github.com/ipld/go-ipld-prime"
	dagpb "github.com/ipld/go-ipld-prime-proto"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/ipld/go-ipld-prime/traversal/selector"
)

type state struct {
	isDone         bool
	currentLink    ipld.Link
	currentContext ipld.LinkContext
}

type nextResponse struct {
	input io.Reader
	err   error
}

// Traverser is a class to perform a selector traversal that stops every time a new block is loaded
// and waits for manual input (in the form of advance or error)
type Traverser struct {
	root           ipld.Link
	selector       ipld.Node
	currentLink    ipld.Link
	currentContext ipld.LinkContext
	isDone         bool
	awaitRequest   chan struct{}
	stateChan      chan state
	responses      chan nextResponse
}

func (t *Traverser) checkState(ctx context.Context) {
	select {
	case <-t.awaitRequest:
		select {
		case <-ctx.Done():
		case newState := <-t.stateChan:
			t.isDone = newState.isDone
			t.currentLink = newState.currentLink
			t.currentContext = newState.currentContext
		}
	default:
	}
}

// NewTraverser creates a new traverser
func NewTraverser(root ipld.Link, selector ipld.Node) *Traverser {
	return &Traverser{
		root:         root,
		selector:     selector,
		awaitRequest: make(chan struct{}, 1),
		stateChan:    make(chan state, 1),
		responses:    make(chan nextResponse),
	}
}

func (t *Traverser) writeDone(ctx context.Context) {
	select {
	case <-ctx.Done():
	case t.stateChan <- state{true, nil, ipld.LinkContext{}}:
	}
}

// Start initiates the traversal (run in a go routine because the regular
// selector traversal expects a call back)
func (t *Traverser) Start(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case t.awaitRequest <- struct{}{}:
	}
	go func() {
		var chooser traversal.LinkTargetNodeStyleChooser = dagpb.AddDagPBSupportToChooser(func(ipld.Link, ipld.LinkContext) (ipld.NodeStyle, error) {
			return basicnode.Style.Any, nil
		})
		loader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
			select {
			case <-ctx.Done():
				return nil, errors.New("Context cancelled")
			case t.stateChan <- state{false, lnk, lnkCtx}:
			}
			select {
			case <-ctx.Done():
				return nil, errors.New("Context cancelled")
			case response := <-t.responses:
				return response.input, response.err
			}
		}
		style, err := chooser(t.root, ipld.LinkContext{})
		if err != nil {
			t.writeDone(ctx)
			return
		}
		builder := style.NewBuilder()
		err = t.root.Load(ctx, ipld.LinkContext{}, builder, loader)
		if err != nil {
			t.writeDone(ctx)
			return
		}
		nd := builder.Build()
		sel, err := selector.ParseSelector(t.selector)
		if err != nil {
			t.writeDone(ctx)
			return
		}
		_ = traversal.Progress{
			Cfg: &traversal.Config{
				Ctx:                        ctx,
				LinkLoader:                 loader,
				LinkTargetNodeStyleChooser: chooser,
			},
		}.WalkAdv(nd, sel, func(traversal.Progress, ipld.Node, traversal.VisitReason) error { return nil })
		t.writeDone(ctx)
	}()

}

// IsComplete returns true if a traversal is complete
func (t *Traverser) IsComplete(ctx context.Context) bool {
	t.checkState(ctx)
	return t.isDone
}

// CurrentRequest returns the current block load waiting to be fulfilled in order
// to advance further
func (t *Traverser) CurrentRequest(ctx context.Context) (ipld.Link, ipld.LinkContext) {
	t.checkState(ctx)
	return t.currentLink, t.currentContext
}

// Advance advances the traversal with an io.Reader for the next requested block
func (t *Traverser) Advance(ctx context.Context, reader io.Reader) error {
	if t.IsComplete(ctx) {
		return errors.New("cannot advance when done")
	}
	select {
	case <-ctx.Done():
		return errors.New("context cancelled")
	case t.awaitRequest <- struct{}{}:
	}
	select {
	case <-ctx.Done():
		return errors.New("context cancelled")
	case t.responses <- nextResponse{reader, nil}:
	}
	return nil
}

// Error aborts the traversal with an error
func (t *Traverser) Error(ctx context.Context, err error) {
	if t.IsComplete(ctx) {
		return
	}
	select {
	case <-ctx.Done():
		return
	case t.awaitRequest <- struct{}{}:
	}
	select {
	case <-ctx.Done():
	case t.responses <- nextResponse{nil, err}:
	}
}
