package migrations

import (
	"database/sql"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	_ "github.com/mutecomm/go-sqlcipher"
)

type Migration010 struct{}

func (Migration010) Up(repoPath string, dbPassword string, testnet bool) (err error) {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return err
	}

	err = migration010UpdateTablesCoins(db, "cases", "caseID", "COALESCE(buyerContract, vendorContract) AS contract")
	if err != nil {
		return err
	}

	err = migration010UpdateTablesCoins(db, "sales", "orderID", "contract")
	if err != nil {
		return err
	}

	err = migration010UpdateTablesCoins(db, "purchases", "orderID", "contract")
	if err != nil {
		return err
	}

	err = writeRepoVer(repoPath, 11)
	if err != nil {
		return err
	}

	return nil
}

func (Migration010) Down(repoPath string, dbPassword string, testnet bool) error {
	// Down migration is a no-op (outsie of updating the version)
	// We can't know which entries columns used to be blank so we can't blank only
	// those entries
	return writeRepoVer(repoPath, 10)
}

func migration010UpdateTablesCoins(db *sql.DB, table string, idColumn string, contractColumn string) error {
	type coinset struct {
		paymentCoin string
		coinType    string
	}

	// Get all records for table and store the coinset for each entry
	rows, err := db.Query("SELECT " + idColumn + ", " + contractColumn + " FROM " + table + " WHERE paymentCoin = '' or coinType = '';")
	if err != nil {
		return err
	}
	defer rows.Close()

	coinsToSet := map[string]coinset{}
	for rows.Next() {
		var orderID, marshaledContract string
		err = rows.Scan(&orderID, &marshaledContract)
		if err != nil {
			return err
		}

		if marshaledContract == "" {
			continue
		}

		contract := &pb.RicardianContract{}
		if err := jsonpb.UnmarshalString(marshaledContract, contract); err != nil {
			return err
		}

		coinsToSet[orderID] = coinset{
			coinType:    coinTypeForContract(contract),
			paymentCoin: paymentCoinForContract(contract),
		}
	}

	// Update each row with the coins
	err = withTransaction(db, func(tx *sql.Tx) error {
		for id, coins := range coinsToSet {
			_, err := tx.Exec(
				"UPDATE "+table+" SET coinType = ?, paymentCoin = ? WHERE "+idColumn+" = ?",
				coins.coinType,
				coins.paymentCoin,
				id)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
