package ipfs

import (
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/path"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	"io"
	pth "path"
)

const CatTimeout = 30

// Fetch data from IPFS given the hash
func Cat(ctx commands.Context, hash string) ([]byte, error) {
	args := []string{"cat", hash}
	req, cmd, err := NewRequestWithTimeout(ctx, args, CatTimeout)
	if err != nil {
		return nil, err
	}
	res := commands.NewResponse(req)
	cmd.Run(req, res)

	if res.Error() != nil {
		return nil, res.Error()
	}
	resp := res.Output()
	reader := resp.(io.Reader)
	b := make([]byte, res.Length())
	_, err = reader.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func Cat2(ctx commands.Context, hash string) ([]byte, error) {
	cctx, _ := context.WithTimeout(context.Background(), CatTimeout)
	ipfsNode, err := ctx.GetNode()
	if err != nil {
		return nil, err
	}
	reader, err := coreunix.Cat(cctx, ipfsNode, pth.Join("ipfs", hash))
	if err != nil {
		return nil, err
	}
	var b []byte
	_, err = reader.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ResolveThenCat(ctx commands.Context, ipnsPath path.Path) ([]byte, error) {
	var ret []byte
	hash, err := Resolve(ctx, ipnsPath.Segments()[0])
	if err != nil {
		return ret, err
	}
	p := make([]string, len(ipnsPath.Segments()))
	p[0] = hash
	for i := 0; i < len(ipnsPath.Segments())-1; i++ {
		p[i+1] = ipnsPath.Segments()[i+1]
	}
	b, err := Cat(ctx, path.Join(p))
	if err != nil {
		return ret, err
	}
	return b, nil
}
