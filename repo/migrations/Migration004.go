package migrations

import (
	"database/sql"
	"path"

	_ "github.com/mutecomm/go-sqlcipher"
	"os"
)

type Migration004 struct{}

func (Migration004) Up(repoPath string, dbPassword string, testnet bool) error {
	var dbPath string
	if testnet {
		dbPath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		dbPath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	if dbPassword != "" {
		p := "pragma key='" + dbPassword + "';"
		db.Exec(p)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("ALTER TABLE sales ADD COLUMN needsSync integer;")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec()
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("5"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}

func (Migration004) Down(repoPath string, dbPassword string, testnet bool) error {
	var dbPath string
	if testnet {
		dbPath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		dbPath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	if dbPassword != "" {
		p := "pragma key='" + dbPassword + "';"
		db.Exec(p)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt1, err := tx.Prepare("ALTER TABLE sales RENAME TO temp_sales;")
	if err != nil {
		return err
	}
	defer stmt1.Close()
	_, err = stmt1.Exec()
	if err != nil {
		tx.Rollback()
		return err
	}
	stmt2, err := tx.Prepare(`create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob);`)
	if err != nil {
		return err
	}
	defer stmt2.Close()
	_, err = stmt2.Exec()
	if err != nil {
		tx.Rollback()
		return err
	}
	stmt3, err := tx.Prepare(`INSERT INTO sales SELECT orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions FROM temp_sales;`)
	if err != nil {
		return err
	}
	defer stmt3.Close()
	_, err = stmt3.Exec()
	if err != nil {
		tx.Rollback()
		return err
	}
	stmt4, err := tx.Prepare(`DROP TABLE temp_sales;`)
	if err != nil {
		return err
	}
	defer stmt4.Close()
	_, err = stmt4.Exec()
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("4"))
	if err != nil {
		return err
	}
	f1.Close()
	return nil
}
