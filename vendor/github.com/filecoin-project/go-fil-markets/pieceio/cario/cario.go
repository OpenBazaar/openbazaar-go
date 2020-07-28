package cario

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	"github.com/ipld/go-ipld-prime"

	"github.com/filecoin-project/go-fil-markets/pieceio"
)

type carIO struct {
}

// NewCarIO returns a new CarIO utility module that wraps go-car
func NewCarIO() pieceio.CarIO {
	return &carIO{}
}

func (c carIO) WriteCar(ctx context.Context, bs pieceio.ReadStore, payloadCid cid.Cid, selector ipld.Node, w io.Writer, userOnNewCarBlocks ...car.OnNewCarBlockFunc) error {
	sc := car.NewSelectiveCar(ctx, bs, []car.Dag{{Root: payloadCid, Selector: selector}})
	return sc.Write(w, userOnNewCarBlocks...)
}

func (c carIO) PrepareCar(ctx context.Context, bs pieceio.ReadStore, payloadCid cid.Cid, selector ipld.Node) (pieceio.PreparedCar, error) {
	sc := car.NewSelectiveCar(ctx, bs, []car.Dag{{Root: payloadCid, Selector: selector}})
	return sc.Prepare()
}

func (c carIO) LoadCar(bs pieceio.WriteStore, r io.Reader) (cid.Cid, error) {
	header, err := car.LoadCar(bs, r)
	if err != nil {
		return cid.Undef, err
	}
	l := len(header.Roots)
	if l == 0 {
		return cid.Undef, fmt.Errorf("invalid header: missing root")
	}
	if l > 1 {
		return cid.Undef, fmt.Errorf("invalid header: contains %d roots (expecting 1)", l)
	}
	return header.Roots[0], nil
}
