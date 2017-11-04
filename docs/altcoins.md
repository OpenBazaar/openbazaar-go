Creating an altcoin integration
===============================

OpenBazaar is designed to be coin-agnostic so it should be possible to use it with your altcoin of choice provided that altcoin
supports basic functionality like multisig escrow. 

There are some caveats, however. At present time the app only supports one coin at a time so enabling an altcoin means disabling 
Bitcoin. This requirement could be changed in the future so that vendors could accept multiple coins and buyers could have multi-coin
wallets, however OpenBazaar is a peer-to-peer application which means each additional wallet will consume substantial resources unlike
a web wallet using a third party backend. An altcoin implementation could conceivably talk to a third party backend, but such an integration
would be more complex than just plugging in the altcoin daemon. 

While multiple wallets is not currently possible, it is possible to run more than one openbazaar-go instance on your machine. This means
you could have one store running Bitcoin and another store running an altcoin. The UI provides a convenient toggle to switch between
running instances. 

### How to integrate your altcoin

The first thing you need to do is create a wallet implementation that conforms to the wallet interface found [here](https://github.com/OpenBazaar/wallet-interface).
The interface *should* be agnostic enough to support most bitcoin derived altcoins, though if you find it isn't just talk to us and we'll see if
we can make the necessary changes. 

The default wallet used openbazaar-go is a custom built SPV wallet based on the btcsuite library. However, there is a second wallet implementation
which talks to bitcoind using the JSON-RPC interface. The code, found [here](https://github.com/OpenBazaar/openbazaar-go/tree/master/bitcoin/bitcoind), could be
used as an example of how to integrate an altcoin. It should just be a matter of cloning the code into a new package and making the necessary changes.

Most likely you will need to fork the [btcrpcclient](https://github.com/btcsuite/btcrpcclient) to make it work with your altcoin as that library expects the data
returned by the JSON-RPC interface to be formatted in a very specific way. For example, `rpcClient.GetNewAddress()` expects a properly formatted Bitcoin address to 
be returned and will thrown an error if it sees an altcoin address. The changes you would need to make to the library should be fairly minimal.

### Exchange rates

OpenBazaar has a convenience option to display prices in the user's domestic fiat currency and to price listings in fiat to avoid exchange rate 
fluctuations. This functionality is provided by the a package implementing the [ExchangeRates](https://github.com/OpenBazaar/openbazaar-go/blob/master/bitcoin/exchangerates.go) interface.

Basically it's just querying an external API to get the exchange rates and returning the results. If you want your altcoin implementation to allow pricing and displaying prices
in fiat, you will need to swap out the default exchange rate implementation with one that returns the exchange rates for your altcoin. If you don't wish to enable
this functionality then you should explicitly disable the exchange rate provider as it will continue to return Bitcoin exchange rates if you do not. 

### How it works

In the wallet interface you'll see this function:
```go
// Returns the type of crytocurrency this wallet implements
CurrencyCode() string
```

All it does is return the currency code which the wallet implements. This function is called whenever creating a new listing. Each listing has a field which shows which currency is accepted.
For example: 
```json
"metadata": {
    "version": 1,
    "contractType": "PHYSICAL_GOOD",
    "format": "FIXED_PRICE",
    "expiry": "2037-12-31T05:00:00.000Z",
    "acceptedCurrency": "btc",
    "pricingCurrency": "USD"
},
```
The currency returned by `CurrencyCode()` will be put in the `acceptedCurrency` field. When someone tries to purchase the listing `CurrencyCode()` is called on the buyer's wallet the return is
checked against the `acceptedCurrency` in the listing. Obviously, is there's no match then the buyer isn't allowed to purchase the listing.

### Hooking it up

For a user perspective, switching out Bitcoin for an altcoin should just be a matter of editing the config file. For example, the default config file looks like this:
```json
"Wallet": {
  "Binary": "",
  "RPCUser": "",
  "RPCPassword": "",
  "TrustedPeer": "",
  "Type": "spvwallet"
}
```
If you wanted to use Zcash, say, you'd set it this:
```json
"Wallet": {
  "Binary": "/usr/bin/zcashd",
  "RPCUser": "alice",
  "RPCPassword": "letmein",
  "TrustedPeer": "",
  "Type": "zcashd"
}
```

The wallet selection switch can be found [in openbazaard.go](https://github.com/OpenBazaar/openbazaar-go/blob/master/openbazaard.go):
```go
switch strings.ToLower(walletCfg.Type) {
	case "spvwallet":
		wallet, err = spvwallet.NewSPVWallet(mn, &params, uint64(walletCfg.MaxFee), uint64(walletCfg.LowFeeDefault), uint64(walletCfg.MediumFeeDefault), uint64(walletCfg.HighFeeDefault), walletCfg.FeeAPI, repoPath, sqliteDB, "OpenBazaar", walletCfg.TrustedPeer, torDialer, ml)
		if err != nil {
			log.Error(err)
			return err
		}
	case "bitcoind":
		if walletCfg.Binary == "" {
			return errors.New("The path to the bitcoind binary must be specified in the config file when using bitcoind")
		}
		usetor := false
		if usingTor && !usingClearnet {
			usetor = true
		}
		wallet = bitcoind.NewBitcoindWallet(mn, &params, repoPath, walletCfg.TrustedPeer, walletCfg.Binary, walletCfg.RPCUser, walletCfg.RPCPassword, usetor, controlPort)
	default:
		log.Fatal("Unknown wallet type")
}
```
The final thing you need to do is add a case for your wallet. Inside that case you might also want to set the exchange rate provider:
```go
exchangeRates = exchange.NewAltcoinExchangeRateProvider(torDialer)
```
or 
```go
exchangeRates = nil
```
To disable the functionality.

### User Interface

You likely will need to either submit a PR or work with the UI developers to get your coin to display properly in the reference UI. The string returned by 
`CurrencyCode()` is passed to the UI via the `GET /ob/config` API call so the UI should know you're not using Bitcoin.


