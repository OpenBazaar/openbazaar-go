package migrations

// Migration021 migrates the listing index file to use the new style (Qm) hashes
// for listings rather than the old CID (z) style hashes.
type Migration021 struct{}

func (Migration021) Up(repoPath string, dbPassword string, testnet bool) error {

	return nil
}

func (Migration021) Down(repoPath string, dbPassword string, testnet bool) error {
	// Down migration is a no-op (outside of updating the version).
	// We can't calculate the old hashes because the go-ipfs is not configured to
	// do so.
	return writeRepoVer(repoPath, 21)
}
