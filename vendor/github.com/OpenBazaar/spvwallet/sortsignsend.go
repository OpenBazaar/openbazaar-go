package spvwallet

import (
	"encoding/json"
	"errors"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	btc "github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/bloom"
	"github.com/btcsuite/btcutil/coinset"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcutil/txsort"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"net/http"
)

func (p *Peer) PongBack(nonce uint64) {
	mpong := wire.NewMsgPong(nonce)

	p.outMsgQueue <- mpong
	return
}
func (p *Peer) UpdateFilterAndSend() {
	filt, err := p.TS.GimmeFilter()
	if err != nil {
		log.Errorf("Filter creation error: %s\n", err.Error())
		return
	}
	// send filter
	p.SendFilter(filt)
	log.Debugf("Sent filter to %s\n", p.con.RemoteAddr().String())
}

func (p *Peer) SendFilter(f *bloom.Filter) {
	p.outMsgQueue <- f.MsgFilterLoad()
	return
}

func (p *Peer) NewOutgoingTx(tx *wire.MsgTx) error {
	txid := tx.TxHash()
	// assign height of zero for txs we create

	p.OKMutex.Lock()
	p.OKTxids[txid] = 0
	p.OKMutex.Unlock()

	_, err := p.TS.Ingest(tx, 0) // our own tx; don't keep track of false positives
	if err != nil {
		return err
	}
	// make an inv message instead of a tx message to be polite
	iv1 := wire.NewInvVect(wire.InvTypeTx, &txid)
	invMsg := wire.NewMsgInv()
	err = invMsg.AddInvVect(iv1)
	if err != nil {
		return err
	}
	log.Debugf("Broadcasting tx %s to %s", tx.TxHash().String(), p.con.RemoteAddr().String())
	p.outMsgQueue <- invMsg
	return nil
}

// Rebroadcast sends an inv message of all the unconfirmed txs the db is
// aware of.  This is called after every sync.  Only txids so hopefully not
// too annoying for nodes.
func (p *Peer) Rebroadcast() {
	// get all unconfirmed txs
	invMsg, err := p.TS.GetPendingInv()
	if err != nil {
		log.Errorf("Rebroadcast error: %s", err.Error())
	}
	if len(invMsg.InvList) == 0 { // nothing to broadcast, so don't
		return
	}
	p.outMsgQueue <- invMsg
	return
}

type Coin struct {
	TxHash       *chainhash.Hash
	TxIndex      uint32
	TxValue      btc.Amount
	TxNumConfs   int64
	ScriptPubKey []byte
}

func (c *Coin) Hash() *chainhash.Hash { return c.TxHash }
func (c *Coin) Index() uint32         { return c.TxIndex }
func (c *Coin) Value() btc.Amount     { return c.TxValue }
func (c *Coin) PkScript() []byte      { return c.ScriptPubKey }
func (c *Coin) NumConfs() int64       { return c.TxNumConfs }
func (c *Coin) ValueAge() int64       { return int64(c.TxValue) * c.TxNumConfs }

func NewCoin(txid []byte, index uint32, value btc.Amount, numConfs int64, scriptPubKey []byte) coinset.Coin {
	shaTxid, _ := chainhash.NewHash(txid)
	c := &Coin{
		TxHash:       shaTxid,
		TxIndex:      index,
		TxValue:      value,
		TxNumConfs:   numConfs,
		ScriptPubKey: scriptPubKey,
	}
	return coinset.Coin(c)
}

func (w *SPVWallet) gatherCoins() map[coinset.Coin]*hd.ExtendedKey {
	height, _ := w.state.GetDBSyncHeight()
	utxos, _ := w.db.Utxos().GetAll()
	m := make(map[coinset.Coin]*hd.ExtendedKey)
	for _, u := range utxos {
		var confirmations int32
		if u.AtHeight > 0 {
			confirmations = height - u.AtHeight
		}
		c := NewCoin(u.Op.Hash.CloneBytes(), u.Op.Index, btc.Amount(u.Value), int64(confirmations), u.ScriptPubkey)
		key, err := w.state.GetKeyForScript(u.ScriptPubkey)
		if err != nil {
			continue
		}
		m[c] = key
	}
	return m
}

func (w *SPVWallet) Spend(amount int64, addr btc.Address, feeLevel FeeLevel) error {
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
		coinSelector := coinset.MaxValueAgeCoinSelector{MaxInputs: 10000, MinChangeAmount: btc.Amount(10000)}
		coins, err := coinSelector.CoinSelect(target, coins)
		if err != nil {
			return total, inputs, scripts, errors.New("insuffient funds")
		}
		additionalPrevScripts = make(map[wire.OutPoint][]byte)
		additionalKeysByAddress = make(map[string]*btc.WIF)
		for _, c := range coins.Coins() {
			total += c.Value()
			outpoint := wire.NewOutPoint(c.Hash(), c.Index())
			in := wire.NewTxIn(outpoint, []byte{})
			in.Sequence = 0 // Opt-in RBF so we can bump fees
			inputs = append(inputs, in)
			additionalPrevScripts[*outpoint] = c.PkScript()
			key := coinMap[c]
			addr, err := key.Address(w.params)
			if err != nil {
				continue
			}
			privKey, err := key.ECPrivKey()
			if err != nil {
				continue
			}
			wif, _ := btc.NewWIF(privKey, w.params, true)
			additionalKeysByAddress[addr.EncodeAddress()] = wif
		}
		return total, inputs, scripts, nil
	}

	// Get the fee per kilobyte
	feePerKB := int64(w.getFeePerByte(feeLevel)) * 1000

	// outputs
	out := wire.NewTxOut(amount, script)

	// Create change source
	changeSource := func() ([]byte, error) {
		addr := w.CurrentAddress(INTERNAL)
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return []byte{}, err
		}
		return script, nil
	}

	authoredTx, err := txauthor.NewUnsignedTransaction([]*wire.TxOut{out}, btc.Amount(feePerKB), inputSource, changeSource)
	if err != nil {
		return err
	}

	// BIP 69 sorting
	txsort.InPlaceSort(authoredTx.Tx)

	// Sign tx
	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
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

	// broadcast
	for _, peer := range w.peerGroup {
		peer.NewOutgoingTx(authoredTx.Tx)
	}
	log.Infof("Broadcasting tx %s to network", authoredTx.Tx.TxHash().String())
	return nil
}

type FeeLevel int

const (
	PRIOIRTY = 0
	NORMAL   = 1
	ECONOMIC = 2
)

func (w *SPVWallet) getFeePerByte(feeLevel FeeLevel) uint64 {
	defaultFee := func() uint64 {
		switch feeLevel {
		case PRIOIRTY:
			return w.priorityFee
		case NORMAL:
			return w.normalFee
		case ECONOMIC:
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
	case PRIOIRTY:
		if fees.FastestFee > w.maxFee {
			return w.maxFee
		} else {
			return fees.FastestFee
		}
	case NORMAL:
		if fees.HalfHourFee > w.maxFee {
			return w.maxFee
		} else {
			return fees.HalfHourFee
		}
	case ECONOMIC:
		if fees.HourFee > w.maxFee {
			return w.maxFee
		} else {
			return fees.HourFee
		}
	default:
		return w.normalFee
	}
}
