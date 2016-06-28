package libbitcoin

func (w *LibbitcoinWallet) GetBalance() (unconfirmed uint64, confirmed uint64) {
	coins := w.db.Coins().GetAll()
	for _, c := range(coins) {
		height, err := w.db.Transactions().GetHeight(c.Txid)
		if err != nil {
			continue
		}
		if height == 0 {
			unconfirmed += uint64(c.Value)
		} else {
			confirmed += uint64(c.Value)
		}
	}
	return unconfirmed, confirmed
}
