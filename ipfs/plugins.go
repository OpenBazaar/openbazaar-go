package ipfs

import (
	"github.com/ipfs/go-ipfs/plugin/loader"
	"sync"
)

var pluginOnce sync.Once

// InstallDatabasePlugins installs the default database plugins
// used by openbazaar-go. This function is guarded by a sync.Once
// so it isn't accidentally called more than once.
func InstallDatabasePlugins() {
	pluginOnce.Do(func() {
		loader, err := loader.NewPluginLoader("")
		if err != nil {
			panic(err)
		}
		err = loader.Initialize()
		if err != nil {
			panic(err)
		}

		err = loader.Inject()
		if err != nil {
			panic(err)
		}
	})
}
