package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/op/go-logging"
	"github.com/tyler-smith/go-bip39"
)

const RepoVersion = "15"

var log = logging.MustGetLogger("repo")
var ErrRepoExists = errors.New("IPFS configuration file exists. Reinitializing would overwrite your keys. Use -f to force overwrite.")

func DoInit(repoRoot string, nBitsForKeypair int, testnet bool, password string, mnemonic string, creationDate time.Time, dbInit func(string, []byte, string, time.Time) error) error {
	nodeSchema, err := schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        repoRoot,
		TestModeEnabled: testnet,
		Mnemonic:        mnemonic,
	})
	if err != nil {
		return err
	}
	if nodeSchema.BuildSchemaDirectories(); err != nil {
		return err
	}

	if fsrepo.IsInitialized(repoRoot) {
		err := MigrateUp(repoRoot, password, testnet)
		if err != nil {
			return err
		}
		return ErrRepoExists
	}

	if err := checkWriteable(repoRoot); err != nil {
		return err
	}

	conf := schema.MustDefaultConfig()

	if mnemonic == "" {
		mnemonic, err = createMnemonic(bip39.NewEntropy, bip39.NewMnemonic)
		if err != nil {
			return err
		}
	}
	seed := bip39.NewSeed(mnemonic, "Secret Passphrase")
	fmt.Printf("Generating Ed25519 keypair...")
	identityKey, err := ipfs.IdentityKeyFromSeed(seed, nBitsForKeypair)
	if err != nil {
		return err
	}
	fmt.Printf("Done\n")

	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}

	log.Infof("Initializing OpenBazaar node at %s\n", repoRoot)
	if err := fsrepo.Init(repoRoot, conf); err != nil {
		return err
	}
	conf.Identity = identity

	if err := addConfigExtensions(repoRoot); err != nil {
		return err
	}

	if err := dbInit(mnemonic, identityKey, password, creationDate); err != nil {
		return err
	}

	f, err := os.Create(path.Join(repoRoot, "repover"))
	if err != nil {
		return err
	}
	_, werr := f.Write([]byte(RepoVersion))
	if werr != nil {
		return werr
	}
	return initializeIpnsKeyspace(repoRoot, identityKey)
}

func checkWriteable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// Directory exists, make sure we can write to it
		testfile := path.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("Unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// Directory does not exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("Cannot write to %s, incorrect permissions", err)
	}

	return err
}

func initializeIpnsKeyspace(repoRoot string, privKeyBytes []byte) error {
	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}
	cfg, err := r.Config()
	if err != nil {
		log.Error(err)
		return err
	}
	identity, err := ipfs.IdentityFromKey(privKeyBytes)
	if err != nil {
		return err
	}

	cfg.Identity = identity
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	err = nd.SetupOfflineRouting()
	if err != nil {
		return err
	}

	return namesys.InitializeKeyspace(ctx, nd.Namesys, nd.Pinning, nd.PrivateKey)
}

func addConfigExtensions(repoRoot string) error {
	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}
	var (
		w = schema.WalletConfig{
			Type:             "spvwallet",
			MaxFee:           2000,
			FeeAPI:           "https://btc.fees.openbazaar.org",
			HighFeeDefault:   160,
			MediumFeeDefault: 60,
			LowFeeDefault:    20,
			TrustedPeer:      "",
		}
		ws = schema.WalletsConfig{
			BTC: schema.CoinConfig{
				Type:             "API",
				API:              "https://btc.bloqapi.net/insight-api",
				APITestnet:       "https://test-insight.bitpay.com/api",
				MaxFee:           200,
				FeeAPI:           "https://btc.fees.openbazaar.org",
				HighFeeDefault:   50,
				MediumFeeDefault: 10,
				LowFeeDefault:    1,
			},
			BCH: schema.CoinConfig{
				Type:             "API",
				API:              "https://bch-insight.bitpay.com/api",
				APITestnet:       "https://test-bch-insight.bitpay.com/api",
				MaxFee:           200,
				HighFeeDefault:   10,
				MediumFeeDefault: 5,
				LowFeeDefault:    1,
			},
			LTC: schema.CoinConfig{
				Type:             "API",
				API:              "https://insight.litecore.io/api",
				APITestnet:       "https://testnet.litecore.io/api",
				MaxFee:           200,
				HighFeeDefault:   20,
				MediumFeeDefault: 10,
				LowFeeDefault:    5,
			},
			ZEC: schema.CoinConfig{
				Type:             "API",
				API:              "https://zcashnetwork.info/api",
				APITestnet:       "https://explorer.testnet.z.cash/api",
				MaxFee:           200,
				HighFeeDefault:   20,
				MediumFeeDefault: 10,
				LowFeeDefault:    5,
			},
		}

		a = schema.APIConfig{
			Enabled:     true,
			AllowedIPs:  []string{},
			HTTPHeaders: nil,
		}

		ds = schema.DataSharing{
			AcceptStoreRequests: false,
			PushTo:              schema.DataPushNodes,
		}

		t = schema.TorConfig{}

		resolvers = schema.ResolverConfig{
			Id: "https://resolver.onename.com/",
		}
	)
	if err := r.SetConfigKey("Wallet", w); err != nil {
		return err
	}
	if err := r.SetConfigKey("Wallets", ws); err != nil {
		return err
	}
	if err := r.SetConfigKey("DataSharing", ds); err != nil {
		return err
	}
	if err := r.SetConfigKey("Resolvers", resolvers); err != nil {
		return err
	}
	if err := r.SetConfigKey("Bootstrap-testnet", schema.BootstrapAddressesTestnet); err != nil {
		return err
	}
	if err := r.SetConfigKey("Dropbox-api-token", ""); err != nil {
		return err
	}
	if err := r.SetConfigKey("RepublishInterval", "24h"); err != nil {
		return err
	}
	if err := r.SetConfigKey("JSON-API", a); err != nil {
		return err
	}
	if err := r.SetConfigKey("Tor-config", t); err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}

func createMnemonic(newEntropy func(int) ([]byte, error), newMnemonic func([]byte) (string, error)) (string, error) {
	entropy, err := newEntropy(128)
	if err != nil {
		return "", err
	}
	mnemonic, err := newMnemonic(entropy)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}

/* Returns the directory to store repo data in.
   It depends on the OS and whether or not we are on testnet. */
func GetRepoPath(isTestnet bool) (string, error) {
	paths, err := schema.NewCustomSchemaManager(schema.SchemaContext{
		TestModeEnabled: isTestnet,
	})
	if err != nil {
		return "", err
	}
	return paths.DataPath(), nil
}
