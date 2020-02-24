Using a Bitcoind Wallet
========================
The SPV openbazaar-go wallet uses [simplified payment verification](https://bitcoin.org/en/developer-guide#simplified-payment-verification-spv) to validate incoming bitcoin payments. (Note the
SPV wallet is no longer the default wallet used by openbazaar-go and now uses an API-based wallet by default.) The benefit of this operating mode is that it achieves a high level of security without much overhead (bandwidth, CPU, etc) and is suitable for the average user. However, there are some downsides
to SPV that might warrant switching to a different wallent:

1. **Lack of full validation**

  SPV wallets only download the bitcoin block headers (not the full contents of the block) and validate that the proof of work is correct. Additionally, they validate a cryptographic
proof which proves an incoming transaction is in the block. However, since the contents of the blocks are neither downloaded nor validated, an attacker (with a sizable amount of mining power)
could create an invalid block header that appears valid to the SPV wallet and trick it into accepting an invalid payment. The saving grace here is that such blocks are very likely to be orphaned
by the wallet when bitcoin miners fail to build on such block. Unless the attacker controls a majority of the mining power (ie, a 51% attack), simply waiting for a number of confirmations before
treating a payment as valid should be enough to foil this attack. But it should be noted that low confirmation payments are less secure in SPV mode than normal.

2. **Privacy leaks**
  
  SPV wallets use bloom filters to avoid downloading all transactions and instead download (mostly) only those transactions relevant to the wallet. In theory bloom filers
  should provide decent privacy since they don't reveal exactly which transactions your wallet is interested in, however in practice the need to continually update and
  resize the filters causes privacy leaks. 
  
  Upon each start up the wallet makes random outgoing connections to the bitcoin network (and does not accept incoming connections). If these random peers are running patched software which logs
  your activity, they can deduce which bitcoin addresses (and hence transactions) belong to your wallet. There are a few things to say about this:
  
  - The leaks are limited to only those peers you connect to, not the entire world. Unlike the blockchain, there isn't any public database where someone can look up your transaciton history.
  - The leaks don't per se reveal your identity. You can still, theoretically, remain pseudonymous while using an SPV wallet. However, your IP address is visible to the peers which see your bloom 
  filters meaning you would need to take additional steps to conceal your identity (such as using a VPN or Tor). 
  - Finally, if any one of your transactions can be independently linked to your real
  world identity (such as through an in-person trade, or by revealing your shipping address) then you must assume your identity can be linked to all transactions made through the wallet.

The bitcoind wallet, by downloading, validating, and relaying all transactions that come across the network solves both of the above problems. The downside is it's a much more heavyweight application
and consumes a good amount of CPU, memory, storage, and bandwidth. Therefore it is not suitable for all users. 

**WARNING**: If you are using Tor for anonymity it is *highly* recommend you also use bitcoind to avoid any possible privacy leaks through bloom filters.

### Setting Up Bitcoind

The first thing you need to do is get a copy of bitcoind. Any of the competing implementations (Core, Unlimited, Classic) will work for this purpose.
You can downloaded a pre-compiled Core binary [here](https://bitcoin.org/en/download) or build it from source from the github repo. 

Next, if you haven't already down so, create a bitcoind config file. Download the following file and save it in the bitcoind data folder: https://github.com/bitcoin/bitcoin/blob/master/contrib/debian/examples/bitcoin.conf

Edit the config file to set a username and password:
```
#rpcuser=alice
#rpcpassword=DONT_USE_THIS_YOU_WILL_GET_ROBBED_8ak1gI25KFTvjovL3gAM967mies3E=
```
Note: you must remove the # before saving.

Next, edit the following fields in the openbazaar-go config file found in the openbazaar2.0 data folder:
```
"Wallet": {
    "Binary": "/path/to/bitcoind",
    "RPCPassword": "DONT_USE_THIS_YOU_WILL_GET_ROBBED_8ak1gI25KFTvjovL3gAM967mies3E=",
    "RPCUser": "alice",
    "Type": "bitcoind"
  }
```
Obviously replacing the username and password with the username and password you set in the bitcoind config file.

Finally, edit the openbazaar-go config file found within your openbazaar data directory to change the `type` of BTC wallet used from "API" to "SPV".

For example, if your BTC configuration looks like:
```
      "BTC": {
         "API": [
            "https://btc.blockbook.api.openbazaar.org/api"
         ],
         "APITestnet": [
            "https://tbtc.blockbook.api.openbazaar.org/api"
         ],
         "FeeAPI": "https://btc.fees.openbazaar.org",
         "HighFeeDefault": 50,
         "LowFeeDefault": 1,
         "MaxFee": 200,
         "MediumFeeDefault": 10,
         "TrustedPeer": "",
         "Type": "API",
         "WalletOptions": null
      },
```

You would update `"Type": "API",` to become `"Type": "SPV",`.

That's it! Just start openbazaar-go.

### Things to consider
- If bitcoind is running when you start openbazaar-go, it will shut it down and restart it. This is done because bitcoind needs to be run
with a specific set of options so that openbazaar-go can detect incoming payments.
- The SPV wallet is slowly becoming deprecated on this project and may lag behind with updates. If any problems are found, please report them as a [new issue](https://github.com/OpenBazaar/openbazaar-go/issues/new).
