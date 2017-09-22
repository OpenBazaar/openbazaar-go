package main

import (
	"context"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	bstk "github.com/OpenBazaar/go-blockstackclient"
	"golang.org/x/net/proxy"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/exchange"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/storage/selfhosted"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	ipath "github.com/ipfs/go-ipfs/path"
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	"github.com/natefinch/lumberjack"
	logging "github.com/op/go-logging"

	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/repo/config"

	recpb "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record/pb"
	obns "github.com/OpenBazaar/openbazaar-go/namesys"

	namepb "github.com/ipfs/go-ipfs/namesys/pb"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	homedir "github.com/mitchellh/go-homedir"
)

const (
	testnet          = true
	imageConcurrency = 30
)

var fileLogFormat = logging.MustStringFormatter(`%{time:15:04:05.000} [%{shortfunc}] [%{level}] %{message}`)

func main() {
	// Get repo path to use
	repoPathFlag := flag.String("datadir", "", "The directory to initialize")
	flag.Parse()

	var err error
	repoPath := *repoPathFlag
	if repoPath == "" {
		repoPath, err = getRepoPath(testnet)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create repo structure
	sqliteDB, err := initializeRepo(repoPath, "", "", testnet)
	if err != repo.ErrRepoExists {
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create user-agent file
	ioutil.WriteFile(path.Join(repoPath, "root", "user_agent"), []byte(core.USERAGENT), os.ModePerm)

	// Node setup
	node, err := newNode(repoPath, sqliteDB)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Peer ID: ", node.IpfsNode.Identity.Pretty())

	// Start getting images.
	randomImages := make(chan *pb.Profile_Image, imageConcurrency)
	stopGettingImages := make(chan struct{})
	for i := 0; i < imageConcurrency; i++ {
		go func() {
			for {
				select {
				case <-stopGettingImages:
					return
				default:
					image, err := newRandomImage(node)
					if err != nil {
						log.Fatal(err)
					}
					randomImages <- image
					log.Print("Loaded random image")

					var reloadImage func()
					reloadImage = func() {
						randomImages <- image
						time.AfterFunc(2*time.Second, reloadImage)
					}
					time.AfterFunc(2*time.Second, reloadImage)
				}
			}
		}()
	}

	// Set fake data
	log.Print("Creating profile")
	profile, err := setFakeProfile(node, randomImages)
	if err != nil {
		log.Fatal(err)
	}

	if profile.Vendor {
		listingCount := rand.Intn(1000)
		log.Printf("Creating %d listings", listingCount)
		for i := 0; i < listingCount; i++ {
			err = addFakeListing(node, randomImages)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	close(stopGettingImages)

	// Publish data to IPFS
	log.Print("Publishing to IPFS")
	node.PointerRepublisher.Republish()
	err = node.SeedNode()
	if err != nil {
		log.Fatal(err)
	}

	log.Print("done")
}

func getRepoPath(isTestnet bool) (string, error) {
	// Set default base path and directory name
	path := "~"
	directoryName := "OpenBazaar2.0"

	// Override OS-specific names
	switch runtime.GOOS {
	case "linux":
		directoryName = ".openbazaar2.0"
	case "darwin":
		path = "~/Library/Application Support"
	}

	// Append testnet flag if on testnet
	if testnet {
		directoryName += "-testnet"
	}

	// Join the path and directory name, then expand the home path
	fullPath, err := homedir.Expand(filepath.Join(path, directoryName))
	if err != nil {
		return "", err
	}

	// Return the shortest lexical representation of the path
	return filepath.Clean(fullPath), nil
}

func initializeRepo(dataDir, password, mnemonic string, testnet bool) (*db.SQLiteDatastore, error) {
	// Database
	sqliteDB, err := db.Create(dataDir, password, testnet)
	if err != nil {
		return sqliteDB, err
	}

	// Initialize the IPFS repo if it does not already exist
	err = repo.DoInit(dataDir, 4096, testnet, password, mnemonic, time.Now(), sqliteDB.Config().Init)
	if err != nil {
		return sqliteDB, err
	}

	return sqliteDB, nil
}

func newNode(repoPath string, db *db.SQLiteDatastore) (*core.OpenBazaarNode, error) {
	// Setup IPFS
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}

	cctx := context.Background()

	// Get config and identity info
	cfg, err := r.Config()
	if err != nil {
		return nil, err
	}

	identityKey, err := db.Config().GetIdentityKey()
	if err != nil {
		return nil, err
	}

	cfg.Identity, err = ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return nil, err
	}

	nd, err := ipfscore.NewNode(cctx, &ipfscore.BuildCfg{
		Repo:   r,
		Online: true,
	})
	if err != nil {
		return nil, err
	}

	ctx := commands.Context{
		Online:     true,
		ConfigRoot: repoPath,
		LoadConfig: func(path string) (*config.Config, error) {
			return fsrepo.ConfigAt(repoPath)
		},
		ConstructNode: func() (*ipfscore.IpfsNode, error) {
			return nd, nil
		},
	}

	// Get current directory root hash
	_, ipnskey := namesys.IpnsKeysForID(nd.Identity)
	ival, err := nd.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
	if err != nil {
		return nil, err
	}

	val, ok := ival.([]byte)
	if !ok {
		log.Fatal("Key value is not a []byte.")
		return nil, errors.New("Key value is not a []byte.")
	}
	dhtrec := new(recpb.Record)
	proto.Unmarshal(val, dhtrec)
	e := new(namepb.IpnsEntry)
	proto.Unmarshal(dhtrec.GetValue(), e)

	// Crosspost gateway
	gatewayURLStrings, err := repo.GetCrosspostGateway(path.Join(repoPath, "config"))
	if err != nil {
		return nil, err
	}

	if len(gatewayURLStrings) <= 0 {
		log.Fatal("No gateways")
	}

	var gatewayUrls []*url.URL
	for _, gw := range gatewayURLStrings {
		if gw == "" {
			continue
		}
		u, err := url.Parse(gw)
		if err != nil {
			return nil, err
		}

		gatewayUrls = append(gatewayUrls, u)
	}

	resolverConfig, err := repo.ResolverConfig{}(path.Join(repoPath, "config"))
	if err != nil {
		return nil, err
	}

	wallet, err := newWallet(repoPath, db)
	if err != nil {
		return nil, err
	}

	var torDialer proxy.Dialer

	resolvers := []obns.Resolver{
		bstk.NewBlockStackClient(resolverConfig.Id, torDialer),
	}

	core.Node = &core.OpenBazaarNode{
		Context:            ctx,
		IpfsNode:           nd,
		RootHash:           ipath.Path(e.Value).String(),
		RepoPath:           repoPath,
		Datastore:          db,
		Wallet:             wallet,
		NameSystem:         obns.NewNameSystem(resolvers),
		ExchangeRates:      exchange.NewBitcoinPriceFetcher(torDialer),
		MessageStorage:     selfhosted.NewSelfHostedStorage(repoPath, ctx, gatewayUrls, torDialer),
		CrosspostGateways:  gatewayUrls,
		UserAgent:          core.USERAGENT,
		PointerRepublisher: rep.NewPointerRepublisher(nd, db, func() bool { return false }),
	}

	core.Node.Service = service.New(core.Node, ctx, db)
	core.Node.MessageRetriever = ret.NewMessageRetriever(db, ctx, nd, nil, core.Node.Service, 16, torDialer, []*url.URL{}, core.Node.SendOfflineAck)

	go core.Node.MessageRetriever.Run()
	go core.Node.PointerRepublisher.Run()

	return core.Node, nil
}

func newWallet(repoPath string, db *db.SQLiteDatastore) (*spvwallet.SPVWallet, error) {
	mn, err := db.Config().GetMnemonic()
	if err != nil {
		return nil, err
	}

	walletCfg, err := repo.GetWalletConfig(path.Join(repoPath, "config"))
	if err != nil {
		return nil, err
	}

	ml := logging.MultiLogger(logging.NewBackendFormatter(logging.NewLogBackend(&lumberjack.Logger{
		Filename:   path.Join(repoPath, "logs", "bitcoin.log"),
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
	}, "", 0), fileLogFormat))

	walletConf := &spvwallet.Config{
		DB:        db,
		Params:    &chaincfg.TestNet3Params,
		Mnemonic:  mn,
		UserAgent: "OpenBazaar",
		RepoPath:  repoPath,
		LowFee:    uint64(walletCfg.LowFeeDefault),
		MediumFee: uint64(walletCfg.MediumFeeDefault),
		HighFee:   uint64(walletCfg.HighFeeDefault),
		MaxFee:    uint64(walletCfg.MaxFee),
		Logger:    ml,
	}
	wallet, err := spvwallet.NewSPVWallet(walletConf)
	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func setFakeProfile(node *core.OpenBazaarNode, randomImages chan (*pb.Profile_Image)) (*pb.Profile, error) {
	profile := newRandomProfile(randomImages)
	err := node.UpdateProfile(profile)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func addFakeListing(node *core.OpenBazaarNode, randomImages chan (*pb.Profile_Image)) error {
	ld := newRandomListing(randomImages)

	// Sign
	contract, err := node.SignListing(ld)
	if err != nil {
		return err
	}

	// Update inventory
	err = node.SetListingInventory(ld)
	if err != nil {
		return err
	}

	// Write file to disk
	f, err := os.Create(path.Join(node.RepoPath, "root", "listings", contract.Listing.Slug+".json"))
	if err != nil {
		return err
	}

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(contract)
	if err != nil {
		return err
	}

	if _, err := f.WriteString(out); err != nil {
		return err
	}

	return nil
}

var slugRE = regexp.MustCompile("[^a-z0-9]+")

func slugify(s string) string {
	return strings.Trim(slugRE.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func isNSFW() bool {
	if rand.Intn(3) == 0 {
		return true
	}
	return false
}
