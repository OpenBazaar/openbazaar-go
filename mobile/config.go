package mobile

// NodeConfig struct of the config parameters to be passed when creating a new node
type NodeConfig struct {

	// Path for the node's data directory
	RepoPath string

	// An API authentication. Authentication is turned on if not "".
	AuthenticationToken string

	// Use testnet
	Testnet bool

	// Add a custom user-agent
	UserAgent string

	// Add a trusted peer for the wallet
	WalletTrustedPeer string

	// Processes to disable
	DisableWallet        bool
	DisableExchangerates bool

	// Run the pprof profiler on port 6060
	Profile bool
}
