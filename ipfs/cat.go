package ipfs

import (
	"io"
	"github.com/ipfs/go-ipfs/commands"
)

// Fetch data from IPFS given the hash
func Cat(ctx commands.Context, hash string) ([]byte, error) {
	args := []string{"cat", hash}
	req, cmd, err := NewRequest(ctx, args)
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
