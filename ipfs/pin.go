package ipfs

import "github.com/ipfs/go-ipfs/commands"


// Recursively un-pin a directory given it's hash.
// This will allow it to be garbage collected.
func UnPinDir(ctx commands.Context, rootHash string) error {
	args := []string{"pin", "rm", rootHash}
	req, cmd, err := NewRequest(ctx, args)
	if err != nil {
		return err
	}
	res := commands.NewResponse(req)
	cmd.Run(req, res)
	if res.Error() != nil {
		return res.Error()
	}
	return nil
}