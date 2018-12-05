package cmd

import (
	"io/ioutil"
	"path"
	"time"

	ipfsrepo "github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	"github.com/OpenBazaar/openbazaar-go/schema"
)

// StartConfig - hold the required configs
type StartConfig struct {
	ConfigData        []byte
	ipfsConfig        *config.Config
	apiConfig         *schema.APIConfig
	torConfig         *schema.TorConfig
	dataSharingConfig *schema.DataSharing
	resolverConfig    *schema.ResolverConfig
	walletsConfig     *schema.WalletsConfig
	republishInterval time.Duration
	dropboxToken      string
}

func initConfigFiles() (*StartConfig, error) {
	var err error
	var configFile []byte
	mainCfg := StartConfig{}
	configFile, err = ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		log.Error("read config:", err)
		return nil, err
	}
	mainCfg.ConfigData = configFile

	apiConfig, err := schema.GetAPIConfig(configFile)
	if err != nil {
		log.Error("scan api config:", err)
		return &mainCfg, err
	}
	mainCfg.apiConfig = apiConfig

	torConfig, err := schema.GetTorConfig(configFile)
	if err != nil {
		log.Error("scan tor config:", err)
		return &mainCfg, err
	}
	mainCfg.torConfig = torConfig

	dataSharingConfig, err := schema.GetDataSharing(configFile)
	if err != nil {
		log.Error("scan data sharing config:", err)
		return &mainCfg, err
	}
	mainCfg.dataSharingConfig = dataSharingConfig

	dropboxToken, err := schema.GetDropboxApiToken(configFile)
	if err != nil {
		log.Error("scan dropbox api token:", err)
		return &mainCfg, err
	}
	mainCfg.dropboxToken = dropboxToken

	resolverConfig, err := schema.GetResolverConfig(configFile)
	if err != nil {
		log.Error("scan resolver config:", err)
		return &mainCfg, err
	}
	mainCfg.resolverConfig = resolverConfig

	republishInterval, err := schema.GetRepublishInterval(configFile)
	if err != nil {
		log.Error("scan republish interval config:", err)
		return &mainCfg, err
	}
	mainCfg.republishInterval = republishInterval

	walletsConfig, err := schema.GetWalletsConfig(configFile)
	if err != nil {
		log.Error("scan wallets config:", err)
		return &mainCfg, err
	}
	mainCfg.walletsConfig = walletsConfig

	ipfsConfig, err := getIPFSConfig()
	if err != nil {
		return &mainCfg, err
	}
	mainCfg.ipfsConfig = ipfsConfig

	return &mainCfg, nil
}

func getIPFSConfig() (*config.Config, error) {
	var err error
	nodeRepo, err = getIPFSRepo()
	if err != nil {
		return nil, err
	}

	cfg, err := nodeRepo.Config()
	if err != nil {
		log.Error("get repo config:", err)
		return nil, err
	}
	return cfg, nil
}

func getIPFSRepo() (ipfsrepo.Repo, error) {
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		log.Error("open repo:", err)
		return nil, err
	}
	return repo, nil
}

func (cfg *StartConfig) setAPIConfigAuthenticated() {
	if cfg.apiConfig.Enabled {
		cfg.apiConfig.Authenticated = true
	}
}
