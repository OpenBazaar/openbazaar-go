package main

import (
	"os"
	"fmt"
	"sort"
	"path/filepath"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/api"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/mitchellh/go-homedir"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/jessevdk/go-flags"
	"github.com/ipfs/go-ipfs/commands"
	"gx/ipfs/QmYVqhVfbK4BKvbW88Lhm26b3ud14sTBvcm1H7uWUx1Fkp/go-multiaddr-net"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/repo/config"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	ma "gx/ipfs/QmcobAGsCjYt5DXoq9et9L8yR8er7o7Cu3DTvpaq12jYSz/go-multiaddr"
)

type Start struct {
	Port int `short:"p" long:"port" description:"The port to use for p2p network traffic"`
	Daemon bool `short:"d" long:"daemon" description:"run the server in the background as a daemon"`
	Testnet bool `short:"t" long:"testnet" description:"use the test network"`
	LogLevel string `short:"l" long:"loglevel" description:"set the logging level [debug, info, warning, error, critical]"`
	AllowIP []string `short:"a" long:"allowip" description:"only allow API connections from these IPs"`
	RestPort int `short:"r" long:"restport" description:"set the rest API port"`
	WebSocketPort int `short:"w" long:"websocketport" description:"set the websocket API port"`
	PIDFile string `long:"pidfile" description:"name of the PID file if running as daemon"`
}
type Stop struct {}
type Restart struct {}

var startServer Start
var stopServer Stop
var restartServer Restart
var parser = flags.NewParser(nil, flags.Default)

func main() {

	parser.AddCommand("start",
		"start the OpenBazaar-Server",
		"The start command starts the OpenBazaar-Server",
		&startServer)
	parser.AddCommand("stop",
		"shutdown the server and disconnect",
		"The stop command disconnects from peers and shuts down OpenBazaar-Server",
		&stopServer)
	parser.AddCommand("restart",
		"restart the server",
		"The restart command shuts down the server and restarts",
		&restartServer)

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}

func (x *Start) Execute(args []string) error {
	// initalize the ipfs repo if it doesn't already exist
	repoPath := "~/.openbazaar2"
	expPath, _ := homedir.Expand(filepath.Clean(repoPath))
	err := repo.DoInit(os.Stdout, expPath, false, 4096)
	if err != nil && err != repo.ErrRepoExists {
		panic(err)
	}

	// ipfs node setup
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		panic(err)
	}
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()


	ctx := commands.Context{}

	ctx.ConfigRoot = expPath
	ctx.LoadConfig = func(path string) (*config.Config, error) {
		return fsrepo.ConfigAt(expPath)
	}
	ctx.ConstructNode = func () (*core.IpfsNode, error) {
		n, err := core.NewNode(cctx, &core.BuildCfg{
			Online: true,
			Repo:   r,
		})
		return n, err
	}

	ncfg := &core.BuildCfg{
		Repo:   r,
		Online: true,
	}
	nd, err := core.NewNode(cctx, ncfg)
	if err != nil {
		panic(err)
	}

	printSwarmAddrs(nd)

	cfg, err := ctx.GetConfig()
	if err != nil {
		return nil
	}

	var gwErrc <-chan error
	if len(cfg.Addresses.Gateway) > 0 {
		var err error
		err, gwErrc = serveHTTPGateway(ctx)
		if err != nil {
			return nil
		}
	}

	for err := range gwErrc {
		fmt.Println(err)
	}

	return nil
}

// printSwarmAddrs prints the addresses of the host
func printSwarmAddrs(node *core.IpfsNode) {
	var addrs []string
	for _, addr := range node.PeerHost.Addrs() {
		addrs = append(addrs, addr.String())
	}
	sort.Sort(sort.StringSlice(addrs))

	for _, addr := range addrs {
		fmt.Printf("Swarm listening on %s\n", addr)
	}
}

// serveHTTPGateway collects options, creates listener, prints status message and starts serving requests
func serveHTTPGateway(ctx commands.Context) (error, <-chan error) {

	cfg, err := ctx.GetConfig()
	if err != nil {
		return nil, nil
	}

	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway)
	if err != nil {
		return fmt.Errorf("serveHTTPGateway: invalid gateway address: %q (err: %s)", cfg.Addresses.Gateway, err), nil
	}

	writable := cfg.Gateway.Writable

	gwLis, err := manet.Listen(gatewayMaddr)
	if err != nil {
		return fmt.Errorf("serveHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err), nil
	}
	// we might have listened to /tcp/0 - lets see what we are listing on
	gatewayMaddr = gwLis.Multiaddr()

	if writable {
		fmt.Printf("Gateway (writable) server listening on %s\n", gatewayMaddr)
	} else {
		fmt.Printf("Gateway (readonly) server listening on %s\n", gatewayMaddr)
	}

	var opts = []corehttp.ServeOption{
		corehttp.PrometheusCollectorOption("gateway"),
		corehttp.CommandsROOption(ctx),
		corehttp.VersionOption(),
		corehttp.IPNSHostnameOption(),
		corehttp.GatewayOption(writable, cfg.Gateway.PathPrefixes),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	node, err := ctx.ConstructNode()
	if err != nil {
		return fmt.Errorf("serveHTTPGateway: ConstructNode() failed: %s", err), nil
	}
	errc := make(chan error)
	go func() {
		errc <- api.Serve(ctx, node, gwLis.NetListener(), opts...)
		close(errc)
	}()
	return nil, errc
}
