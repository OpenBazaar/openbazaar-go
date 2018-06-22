package cmd

import (
	"database/sql"
	"encoding/json"

	"context"
	"fmt"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/ipfs"

	"github.com/ipfs/go-ipfs/commands"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/config"
	"io/ioutil"
	"strings"

	"bufio"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"golang.org/x/crypto/ssh/terminal"
	"syscall"
	"time"
)

type Convert struct {
	Password string `short:"p" long:"password" description:"the encryption password if the database is encrypted"`
	DataDir  string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet  bool   `short:"t" long:"testnet" description:"use the test network"`
}

func (x *Convert) Execute(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Please specify the cryptocurrency you wish to convert to. Examples:\nopenbazaar-go convert bitcoin\nopenbazaar-go convert bitcoincash\nopenbazaar-go convert zcash /path/to/zcashd\nopenbazaar-go convert zcash-light\n")
		return nil
	}
	if strings.ToLower(args[0]) == "zcash" && len(args) == 1 {
		fmt.Fprintf(os.Stderr, "When converting to zcash please specify the path to the zcash binary. Example:\nopenbazaar-go convert zcash /path/to/zcashd\n")
		return nil
	}
	if !(strings.ToLower(args[0]) == "bitcoin" || strings.ToLower(args[0]) == "bitcoincash" || strings.ToLower(args[0]) == "zcash" || strings.ToLower(args[0]) == "zcash-light") {
		fmt.Fprintf(os.Stderr, "Unknown currency type: please enter either bitcoin, bitcoincash, zcash, or zcash-light.\n")
		return nil
	}

	var str string
	var cfgtype string
	var currencyCode string
	switch strings.ToLower(args[0]) {
	case "bitcoin":
		str = "Bitcoin"
		cfgtype = "spvwallet"
		currencyCode = "BTC"
	case "bitcoincash":
		str = "Bitcoin Cash"
		cfgtype = "bitcoincash"
		currencyCode = "BCH"
	case "zcash":
		str = "ZCash"
		cfgtype = "zcashd"
		currencyCode = "ZEC"
	case "zcash-light":
		str = "ZCash"
		cfgtype = "zcash-light"
		currencyCode = "ZEC"
	}

	if x.Testnet {
		currencyCode = "T" + currencyCode
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("This command will convert your wallet to %s. It will delete any coins you have in your wallet and will prevent you from completing any outstanding orders. Please make sure you have emptied the wallet and completed all your orders before running this command. You will also need to select a new moderator.\n", str)
	fmt.Println("Are you sure you want to continue (y/n)?")
	yn, _ := reader.ReadString('\n')
	if !(strings.ToLower(yn) == "y\n" || strings.ToLower(yn) == "yes\n" || strings.ToLower(yn)[:1] == "y") {
		fmt.Println("No changes made")
		return nil
	}

	fmt.Println("Are you REALLY sure (y/n)?")
	yn, _ = reader.ReadString('\n')
	if !(strings.ToLower(yn) == "y\n" || strings.ToLower(yn) == "yes\n" || strings.ToLower(yn)[:1] == "y") {
		fmt.Println("No changes made")
		return nil
	}

	fmt.Println("Working...")

	// Set repo path
	repoPath, err := repo.GetRepoPath(x.Testnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}

	// Wipe database tables
	var dbPath string
	if x.Testnet {
		dbPath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		dbPath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	sqlitedb, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	if x.Password != "" {
		p := "pragma key='" + x.Password + "';"
		sqlitedb.Exec(p)
	}

	_, err = sqlitedb.Exec("DELETE FROM txns;")
	if err != nil {
		return err
	}
	_, err = sqlitedb.Exec("DELETE FROM stxos;")
	if err != nil {
		return err
	}
	_, err = sqlitedb.Exec("DELETE FROM utxos;")
	if err != nil {
		return err
	}
	_, err = sqlitedb.Exec("DELETE FROM keys;")
	if err != nil {
		return err
	}

	// Update coin type in config file
	cf, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	json.Unmarshal(cf, &cfgIface)
	cfgObj, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("Invalid config file")
	}

	walletIface, ok := cfgObj["Wallet"]
	if !ok {
		return errors.New("Config file missing wallet field")
	}
	walletCfg, ok := walletIface.(map[string]interface{})
	if !ok {
		return errors.New("Invalid config file")
	}
	walletCfg["Type"] = cfgtype
	if strings.ToLower(args[0]) == "zcash" {
		walletCfg["Binary"] = args[1]
	}
	out, err := json.MarshalIndent(cfgObj, "", "   ")
	if err != nil {
		return err
	}
	f, err := os.Create(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	_, err = f.Write(out)
	if err != nil {
		return err
	}
	f.Close()

	// Update listings with new coin type
	creationDate := time.Now()
	var sqliteDB *db.SQLiteDatastore
	sqliteDB, err = InitializeRepo(repoPath, x.Password, "", x.Testnet, creationDate)
	if err != nil && err != repo.ErrRepoExists {
		return err
	}

	// If the database cannot be decrypted, exit
	if sqliteDB.Config().IsEncrypted() {
		sqliteDB.Close()
		fmt.Print("Database is encrypted, enter your password: ")
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		pw := string(bytePassword)
		sqliteDB, err = InitializeRepo(repoPath, pw, "", x.Testnet, time.Now())
		if err != nil && err != repo.ErrRepoExists {
			return err
		}
		if sqliteDB.Config().IsEncrypted() {
			PrintError("Invalid password")
			os.Exit(3)
		}
	}
	// Get current identity
	identityKey, err := sqliteDB.Config().GetIdentityKey()
	if err != nil {
		PrintError(err.Error())
		return err
	}
	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}

	// IPFS node setup
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		PrintError(err.Error())
		return err
	}
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := r.Config()
	if err != nil {
		PrintError(err.Error())
		return err
	}

	cfg.Identity = identity

	ncfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: true,
		ExtraOpts: map[string]bool{
			"mplex": true,
		},
		DNSResolver: nil,
		Routing:     nil,
	}

	nd, err := ipfscore.NewNode(cctx, ncfg)
	if err != nil {
		PrintError(err.Error())
		return err
	}

	ctx := commands.Context{}
	ctx.Online = false
	ctx.ConfigRoot = repoPath
	ctx.LoadConfig = func(path string) (*config.Config, error) {
		return fsrepo.ConfigAt(repoPath)
	}
	ctx.ConstructNode = func() (*ipfscore.IpfsNode, error) {
		return nd, nil
	}

	files, err := ioutil.ReadDir(path.Join(repoPath, "root", "listings"))
	if err != nil {
		return err
	}

	hashes := make(map[string]string)

	for _, f := range files {
		file, err := ioutil.ReadFile(path.Join(repoPath, "root", "listings", f.Name()))
		if err != nil {
			return err
		}
		sl := new(pb.SignedListing)
		err = jsonpb.UnmarshalString(string(file), sl)
		if err != nil {
			return err
		}
		sl.Listing.Metadata.AcceptedCurrencies = []string{currencyCode}
		sl.Listing.Moderators = []string{}

		serializedListing, err := proto.Marshal(sl.Listing)
		if err != nil {
			return err
		}
		idSig, err := nd.PrivateKey.Sign(serializedListing)
		if err != nil {
			return err
		}
		sl.Signature = idSig

		m := jsonpb.Marshaler{
			EnumsAsInts:  false,
			EmitDefaults: false,
			Indent:       "    ",
			OrigName:     false,
		}
		out, err := m.MarshalToString(sl)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(path.Join(repoPath, "root", "listings", f.Name()), []byte(out), os.ModePerm); err != nil {
			return err
		}
		h, err := ipfs.GetHashOfFile(ctx, path.Join(repoPath, "root", "listings", f.Name()))
		if err != nil {
			return err
		}
		hashes[sl.Listing.Slug] = h
	}

	indexPath := path.Join(repoPath, "root", "listings.json")
	indexBytes, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return err
	}
	var index []core.ListingData

	err = json.Unmarshal(indexBytes, &index)
	if err != nil {
		return err
	}

	for i, l := range index {
		h, ok := hashes[l.Slug]
		if !ok {
			return errors.New("Malformatted index file")
		}
		l.Hash = h
		index[i] = l
	}
	// Write it back to file
	ifile, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer ifile.Close()

	j, jerr := json.MarshalIndent(index, "", "    ")
	if jerr != nil {
		return jerr
	}
	_, werr := ifile.Write(j)
	if werr != nil {
		return werr
	}

	// Update profile
	pro, err := ioutil.ReadFile(path.Join(repoPath, "root", "profile.json"))

	profile := new(pb.Profile)
	err = jsonpb.UnmarshalString(string(pro), profile)
	if err != nil {
		return err
	}
	profile.Currencies = []string{currencyCode}
	if profile.ModeratorInfo != nil {
		profile.ModeratorInfo.AcceptedCurrencies = []string{currencyCode}
		m := jsonpb.Marshaler{
			EnumsAsInts:  false,
			EmitDefaults: false,
			Indent:       "    ",
			OrigName:     false,
		}
		out, err := m.MarshalToString(profile)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(path.Join(repoPath, "root", "profile.json"), []byte(out), os.ModePerm); err != nil {
			return err
		}
	}
	nd.Close()

	// Remove moderators from settings
	settings, err := sqliteDB.Settings().Get()
	if err == nil {
		settings.StoreModerators = &[]string{}
		sqliteDB.Settings().Put(settings)
	}

	// Remove headers.bin
	os.Remove(path.Join(repoPath, "headers.bin"))

	// Remove peers.json
	os.Remove(path.Join(repoPath, "peers.json"))

	fmt.Println("Done")
	return nil
}
