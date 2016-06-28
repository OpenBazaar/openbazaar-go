package libbitcoin



import (
	"fmt"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/coinset"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/fastsha256"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
)

type Coin struct {
	TxHash     *wire.ShaHash
	TxIndex    uint32
	TxValue    btcutil.Amount
	TxNumConfs int64
}

func (c *Coin) Hash() *wire.ShaHash   { return c.TxHash }
func (c *Coin) Index() uint32         { return c.TxIndex }
func (c *Coin) Value() btcutil.Amount { return c.TxValue }
func (c *Coin) PkScript() []byte      { return nil }
func (c *Coin) NumConfs() int64       { return c.TxNumConfs }
func (c *Coin) ValueAge() int64       { return int64(c.TxValue) * c.TxNumConfs }

func NewCoin(index int64, value btcutil.Amount, numConfs int64) coinset.Coin {
	h := fastsha256.New()
	h.Write([]byte(fmt.Sprintf("%d", index)))
	hash, _ := wire.NewShaHash(h.Sum(nil))
	c := &Coin{
		TxHash:     hash,
		TxIndex:    0,
		TxValue:    value,
		TxNumConfs: numConfs,
	}
	return coinset.Coin(c)
}

func (w *LibbitcoinWallet) gatherCoins() []coinset.Coin {
	utxos := w.db.Coins().GetAll()
	var coins []coinset.Coin
	for _, u := range(utxos) {
		c := NewCoin(int64(u.Index), btcutil.Amount(int64(u.Value)), 0)
		coins = append(coins, c)
	}
	return coins
}

// TODO: unfinished
func (w *LibbitcoinWallet) Send(amount int64, addr btcutil.Address, fee bitcoin.FeeLevel) error {
	coinSelector := coinset.MinNumberCoinSelector{MaxInputs: 10000, MinChangeAmount: 10000}
	_, err := coinSelector.CoinSelect(btcutil.Amount(amount), w.gatherCoins())
	if err != nil {
		return err
	}
	return nil
}