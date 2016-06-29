package libbitcoin

// FIXME: this isn't returning the correct unconfirmed balance. If we spend a confirmed
// FIXME: transaction, the change is then technically "unconfirmed" but we don't really
// FIXME: care about that since we originated it. If the spend doesn't confirm we still own
// FIXME: the coins. So unconfirmed change from confirmed spends should return as confirmed.
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
