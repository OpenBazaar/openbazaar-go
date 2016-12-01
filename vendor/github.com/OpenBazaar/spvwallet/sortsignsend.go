package spvwallet

import (
	"encoding/hex"
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
	"time"
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
		if u.Freeze {
			continue
		}
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
	tx, err := w.buildTx(amount, addr, feeLevel)
	if err != nil {
		return err
	}
	// broadcast
	for _, peer := range w.peerGroup {
		peer.NewOutgoingTx(tx)
	}
	return nil
}

func (w *SPVWallet) EstimateFee(ins []TransactionInput, outs []TransactionOutput, feePerByte uint64) uint64 {
	tx := new(wire.MsgTx)
	for _, out := range outs {
		output := wire.NewTxOut(out.Value, out.ScriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false)
	fee := estimatedSize * int(feePerByte)
	return uint64(fee)
}

func (w *SPVWallet) CreateMultisigSignature(ins []TransactionInput, outs []TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]Signature, error) {
	var sigs []Signature
	tx := new(wire.MsgTx)
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return sigs, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		output := wire.NewTxOut(out.Value, out.ScriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false)
	fee := estimatedSize * int(feePerByte)
	feePerOutput := fee / len(tx.TxOut)
	for _, output := range tx.TxOut {
		output.Value -= int64(feePerOutput)
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	signingKey, err := key.ECPrivKey()
	if err != nil {
		return sigs, err
	}

	for i, _ := range tx.TxIn {
		sig, err := txscript.RawTxInSignature(tx, i, redeemScript, txscript.SigHashAll, signingKey)
		if err != nil {
			continue
		}
		bs := Signature{InputIndex: uint32(i), Signature: sig}
		sigs = append(sigs, bs)
	}
	return sigs, nil
}

func (w *SPVWallet) Multisign(ins []TransactionInput, outs []TransactionOutput, sigs1 []Signature, sigs2 []Signature, redeemScript []byte, feePerByte uint64) error {
	tx := new(wire.MsgTx)
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		output := wire.NewTxOut(out.Value, out.ScriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false)
	fee := estimatedSize * int(feePerByte)
	feePerOutput := fee / len(tx.TxOut)
	for _, output := range tx.TxOut {
		output.Value -= int64(feePerOutput)
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	for i, input := range tx.TxIn {
		var sig1 []byte
		var sig2 []byte
		for _, sig := range sigs1 {
			if int(sig.InputIndex) == i {
				sig1 = sig.Signature
			}
		}
		for _, sig := range sigs2 {
			if int(sig.InputIndex) == i {
				sig2 = sig.Signature
			}
		}
		builder := txscript.NewScriptBuilder()
		builder.AddOp(txscript.OP_0)
		builder.AddData(sig1)
		builder.AddData(sig2)
		builder.AddData(redeemScript)
		scriptSig, err := builder.Script()
		if err != nil {
			return err
		}
		input.SignatureScript = scriptSig
	}
	// broadcast
	for _, peer := range w.peerGroup {
		peer.NewOutgoingTx(tx)
	}
	return nil
}

func (w *SPVWallet) SweepMultisig(utxos []Utxo, key *hd.ExtendedKey, redeemScript []byte, feeLevel FeeLevel) error {
	internalAddr := w.CurrentAddress(INTERNAL)
	script, err := txscript.PayToAddrScript(internalAddr)
	if err != nil {
		return err
	}

	var val int64
	var inputs []*wire.TxIn
	additionalPrevScripts := make(map[wire.OutPoint][]byte)
	for _, u := range utxos {
		val += u.Value
		in := wire.NewTxIn(&u.Op, []byte{})
		inputs = append(inputs, in)
		additionalPrevScripts[u.Op] = u.ScriptPubkey
	}
	out := wire.NewTxOut(val, script)

	estimatedSize := EstimateSerializeSize(len(utxos), []*wire.TxOut{out}, false)

	// Calculate the fee
	feePerByte := int(w.GetFeePerByte(feeLevel))
	fee := estimatedSize * feePerByte

	outVal := val - int64(fee)
	if outVal < 0 {
		outVal = 0
	}
	out.Value = outVal

	tx := &wire.MsgTx{
		Version:  wire.TxVersion,
		TxIn:     inputs,
		TxOut:    []*wire.TxOut{out},
		LockTime: 0,
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	// Sign tx
	privKey, err := key.ECPrivKey()
	if err != nil {
		return err
	}

	pk := privKey.PubKey().SerializeCompressed()
	address, err := btc.NewAddressPubKey(pk, w.params)

	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		if address.EncodeAddress() == addr.EncodeAddress() {
			wif, err := btc.NewWIF(privKey, w.params, true)
			if err != nil {
				return nil, false, err
			}
			return wif.PrivKey, wif.CompressPubKey, nil
		}
		return nil, false, errors.New("Not found")
	})
	getScript := txscript.ScriptClosure(func(addr btc.Address) ([]byte, error) {
		return redeemScript, nil
	})

	for i, txIn := range tx.TxIn {
		prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
		script, err := txscript.SignTxOutput(w.params,
			tx, i, prevOutScript, txscript.SigHashAll, getKey,
			getScript, txIn.SignatureScript)
		if err != nil {
			return errors.New("Failed to sign transaction")
		}
		txIn.SignatureScript = script
	}

	// broadcast
	for _, peer := range w.peerGroup {
		peer.NewOutgoingTx(tx)
	}
	return nil
}

func (w *SPVWallet) buildTx(amount int64, addr btc.Address, feeLevel FeeLevel) (*wire.MsgTx, error) {
	// Check for dust
	script, _ := txscript.PayToAddrScript(addr)
	if txrules.IsDustAmount(btc.Amount(amount), len(script), txrules.DefaultRelayFeePerKb) {
		return nil, errors.New("Amount is below dust threshold")
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
	feePerKB := int64(w.GetFeePerByte(feeLevel)) * 1000

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
		return nil, err
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
			return nil, errors.New("Failed to sign transaction")
		}
		txIn.SignatureScript = script
	}
	return authoredTx.Tx, nil
}

type feeCache struct {
	fees        *Fees
	lastUpdated time.Time
}

type Fees struct {
	FastestFee  uint64
	HalfHourFee uint64
	HourFee     uint64
}

var cache *feeCache = &feeCache{}

func (w *SPVWallet) GetFeePerByte(feeLevel FeeLevel) uint64 {
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
	fees := new(Fees)
	if time.Since(cache.lastUpdated) > time.Minute {
		resp, err := http.Get(w.feeAPI)
		if err != nil {
			return defaultFee()
		}

		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&fees)
		if err != nil {
			return defaultFee()
		}
		cache.lastUpdated = time.Now()
		cache.fees = fees
	} else {
		fees = cache.fees
	}
	switch feeLevel {
	case PRIOIRTY:
		if fees.FastestFee > w.maxFee || fees.FastestFee == 0 {
			return w.maxFee
		} else {
			return fees.FastestFee
		}
	case NORMAL:
		if fees.HalfHourFee > w.maxFee || fees.HalfHourFee == 0 {
			return w.maxFee
		} else {
			return fees.HalfHourFee
		}
	case ECONOMIC:
		if fees.HourFee > w.maxFee || fees.HourFee == 0 {
			return w.maxFee
		} else {
			return fees.HourFee
		}
	default:
		return w.normalFee
	}
}
