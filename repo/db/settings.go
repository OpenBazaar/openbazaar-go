package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type SettingsDB struct {
	db   *sql.DB
	lock sync.RWMutex
}

func (s *SettingsDB) Put(settings repo.SettingsData) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(&settings, "", "    ")
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert or replace into config(key, value) values(?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec("settings", string(b))
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (s *SettingsDB) Get() (repo.SettingsData, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	settings := repo.SettingsData{}
	stmt, err := s.db.Prepare("select value from config where key=?")
	if err != nil {
		return settings, err
	}
	defer stmt.Close()
	var settingsBytes []byte
	err = stmt.QueryRow("settings").Scan(&settingsBytes)
	if err != nil {
		return settings, err
	}
	err = json.Unmarshal(settingsBytes, &settings)
	if err != nil {
		return settings, err
	}
	return settings, nil
}

func (s *SettingsDB) Update(settings repo.SettingsData) error {
	current, err := s.Get()
	if err != nil {
		return errors.New("Not Found")
	}
	if settings.PaymentDataInQR == nil {
		settings.PaymentDataInQR = current.PaymentDataInQR
	}
	if settings.ShowNotifications == nil {
		settings.ShowNotifications = current.ShowNotifications
	}
	if settings.ShowNsfw == nil {
		settings.ShowNsfw = current.ShowNsfw
	}
	if settings.ShippingAddresses == nil {
		settings.ShippingAddresses = current.ShippingAddresses
	}
	if settings.LocalCurrency == nil {
		settings.LocalCurrency = current.LocalCurrency
	}
	if settings.Country == nil {
		settings.Country = current.Country
	}
	if settings.Language == nil {
		settings.Language = current.Language
	}
	if settings.TermsAndConditions == nil {
		settings.TermsAndConditions = current.TermsAndConditions
	}
	if settings.RefundPolicy == nil {
		settings.RefundPolicy = current.RefundPolicy
	}
	if settings.BlockedNodes == nil {
		settings.BlockedNodes = current.BlockedNodes
	}
	if settings.StoreModerators == nil {
		settings.StoreModerators = current.StoreModerators
	}
	if settings.MisPaymentBuffer == nil {
		settings.MisPaymentBuffer = current.MisPaymentBuffer
	}
	if settings.SMTPSettings == nil {
		settings.SMTPSettings = current.SMTPSettings
	}
	err = s.Put(settings)
	if err != nil {
		return err
	}
	return nil
}

// Delete removes all settings from the database. It's a destructive action that should be used with care.
func (s *SettingsDB) Delete() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	stmt, err := s.db.Prepare("delete from config where key = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec("settings")

	return err
}
