package libbitcoin

func (w *LibbitcoinWallet) GetBalance() uint64 {
	coins := w.db.Coins().GetAll()
	var value uint64
	for _, c := range(coins) {
		value += uint64(c.Value)
	}
	return value
}
