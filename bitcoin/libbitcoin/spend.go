package libbitcoin



import (
	"net/http"
	"encoding/json"
	"errors"
	"bytes"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/coinset"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/tyler-smith/go-bip32"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcutil/txsort"
	btc "github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcd/btcec"
	"encoding/hex"
)

type Coin struct {
	TxHash       *wire.ShaHash
	TxIndex      uint32
	TxValue      btc.Amount
	TxNumConfs   int64
	ScriptPubKey []byte
}

func (c *Coin) Hash() *wire.ShaHash   { return c.TxHash }
func (c *Coin) Index() uint32         { return c.TxIndex }
func (c *Coin) Value() btc.Amount     { return c.TxValue }
func (c *Coin) PkScript() []byte      { return c.ScriptPubKey }
func (c *Coin) NumConfs() int64       { return c.TxNumConfs }
func (c *Coin) ValueAge() int64       { return int64(c.TxValue) * c.TxNumConfs }

func NewCoin(txid []byte, index int64, value btc.Amount, numConfs int64, scriptPubKey []byte) coinset.Coin {
	shaTxid, _ := wire.NewShaHash(txid)
	c := &Coin{
		TxHash:       shaTxid,
		TxIndex:      0,
		TxValue:      value,
		TxNumConfs:   numConfs,
		ScriptPubKey: scriptPubKey,
	}
	return coinset.Coin(c)
}

func (w *LibbitcoinWallet) gatherCoins() map[coinset.Coin]*bip32.Key {
	utxos := w.db.Coins().GetAll()
	m := make(map[coinset.Coin]*bip32.Key)
	for _, u := range(utxos) {
		sha, _ := wire.NewShaHashFromStr(hex.EncodeToString(u.Txid))
		c := NewCoin(sha.Bytes(), int64(u.Index), btc.Amount(int64(u.Value)), 0, u.ScriptPubKey)
		key, err := w.db.Keys().GetKeyForScript(u.ScriptPubKey)
		if err != nil {
			continue
		}
		m[c] = key
	}
	return m
}

func (w *LibbitcoinWallet) Spend(amount int64, addr btc.Address, feeLevel bitcoin.FeeLevel) error {
	// Check for dust
	script, _ := txscript.PayToAddrScript(addr)
	if txrules.IsDustAmount(btc.Amount(amount), len(script), txrules.DefaultRelayFeePerKb) {
		return errors.New("Amount is below dust threshold")
	}

	var additionalPrevScripts map[wire.OutPoint][]byte
	var additionalKeysByAddress map[string]*btc.WIF

	// Create input source
	coinMap := w.gatherCoins()
	coins := make([]coinset.Coin, 0, len(coinMap))
	for k := range coinMap {
		coins = append(coins, k)
	}
	inputSource := func(target btc.Amount) (total btc.Amount, inputs []*wire.TxIn, scripts [][]byte, err error) {
		// TODO: maybe change the coin selection algorithm? We're using min coins right now because
		// TODO: we don't know the number of confirmations on each coin without querying the libbitcoin server.
		coinSelector := coinset.MinNumberCoinSelector{MaxInputs: 10000, MinChangeAmount: btc.Amount(10000)}
		coins, err := coinSelector.CoinSelect(target, coins)
		if err != nil {
			return total, inputs, scripts, errors.New("insuffient funds")
		}
		additionalPrevScripts = make(map[wire.OutPoint][]byte)
		additionalKeysByAddress = make(map[string]*btc.WIF)
		for _, c := range(coins.Coins()) {
			total += c.Value()
			outpoint := wire.NewOutPoint(c.Hash(), c.Index())
			in := wire.NewTxIn(outpoint, []byte{})
			in.Sequence = 0 // Opt-in RBF so we can bump fees
			inputs = append(inputs, in)
			additionalPrevScripts[*outpoint] = c.PkScript()
			key := coinMap[c]
			addr, _ := btc.NewAddressPubKey(key.PublicKey().Key, w.params)
			pk, _ := btcec.PrivKeyFromBytes(btcec.S256(), key.Key)
			wif, _ := btc.NewWIF(pk, w.params, true)
			additionalKeysByAddress[addr.AddressPubKeyHash().EncodeAddress()] = wif
		}
		return total, inputs, scripts, nil
	}

	// Get the fee per kilobyte
	feePerKB := int64(w.getFeePerByte(feeLevel)) * 1000

	// outputs
	out := wire.NewTxOut(amount, script)

	// Create change source
	changeSource := func() ([]byte, error) {
		addr := w.GetCurrentAddress(bitcoin.CHANGE)
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return []byte{}, err
		}
		return script, nil
	}

	authoredTx, err := txauthor.NewUnsignedTransaction([]*wire.TxOut{out,}, btc.Amount(feePerKB), inputSource, changeSource)
	if err != nil {
		return err
	}

	// BIP 69 sorting
	txsort.InPlaceSort(authoredTx.Tx)

	// Sign tx
	getKey := txscript.KeyClosure(func(addr btc.Address) (
	*btcec.PrivateKey, bool, error) {
		addrStr := addr.EncodeAddress()
		wif := additionalKeysByAddress[addrStr]
		return wif.PrivKey, wif.CompressPubKey, nil
	})
	getScript := txscript.ScriptClosure(func(
		addr btc.Address) ([]byte, error) {
		return []byte{}, nil
	})
	for i, txIn := range authoredTx.Tx.TxIn {
		prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
		script, err := txscript.SignTxOutput(w.params,
			authoredTx.Tx, i, prevOutScript, txscript.SigHashAll, getKey,
			getScript, txIn.SignatureScript)
		if err != nil {
			return errors.New("Failed to sign transaction")
		}
		txIn.SignatureScript = script
	}

	serializedTx := new(bytes.Buffer)
	authoredTx.Tx.Serialize(serializedTx)
	log.Notice(hex.EncodeToString(serializedTx.Bytes()))
	w.Client.Broadcast(serializedTx.Bytes(), func(i interface{}, err error){
		if err == nil {
			log.Infof("Broadcast tx %s to bitcoin network\n", authoredTx.Tx.TxSha().String())
		} else {
			log.Errorf("Failed to broadcast tx, reason: %s\n", err)
		}
	})

	// Update the db
	w.ProcessTransaction(btc.NewTx(authoredTx.Tx), 0)
	return nil
}

func (w *LibbitcoinWallet) getFeePerByte(feeLevel bitcoin.FeeLevel) uint64 {
	defaultFee := func() uint64 {
		switch feeLevel {
		case bitcoin.PRIOIRTY:
			return w.priorityFee
		case bitcoin.NORMAL:
			return w.normalFee
		case bitcoin.ECONOMIC:
			return w.economicFee
		default:
			return w.normalFee
		}
	}
	if w.feeAPI == "" {
		return defaultFee()
	}

	resp, err := http.Get(w.feeAPI)
	if err != nil {
		return defaultFee()
	}

	defer resp.Body.Close()

	type Fees struct {
		FastestFee  uint64
		HalfHourFee uint64
		HourFee     uint64
	}
	fees := new(Fees)
	err = json.NewDecoder(resp.Body).Decode(&fees)
	if err != nil {
		return defaultFee()
	}
	switch feeLevel {
	case bitcoin.PRIOIRTY:
		if fees.FastestFee > w.maxFee {
			return w.maxFee
		} else {
			return fees.FastestFee
		}
	case bitcoin.NORMAL:
		if fees.HalfHourFee > w.maxFee {
			return w.maxFee
		} else {
			return fees.HalfHourFee
		}
	case bitcoin.ECONOMIC:
		if fees.HourFee > w.maxFee {
			return w.maxFee
		} else {
			return fees.HourFee
		}
	default:
		return w.normalFee
	}
}