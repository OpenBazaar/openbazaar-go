package db

import (
	"database/sql"
	"sync"
)

type modelStore struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (m *modelStore) Lock()   { m.lock.Lock() }
func (m *modelStore) Unlock() { m.lock.Unlock() }

// Begin returns a *sql.Tx for transactional query support
func (m *modelStore) BeginTransaction() (*sql.Tx, error) {
	return m.db.Begin()
}

// PrepareQuery returns a *sql.Stmt to the wrapped DB
func (m *modelStore) PrepareQuery(query string) (*sql.Stmt, error) {
	return m.db.Prepare(query)
}

// PrepareAndExecuteQuery returns the resulting *sql.Rows for the executed query
func (m *modelStore) PrepareAndExecuteQuery(query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.Query(query, args...)
}

// ExecuteQuery returns the *sql.Result for the executed query without returning Rows
func (m *modelStore) ExecuteQuery(query string, args ...interface{}) (sql.Result, error) {
	return m.db.Exec(query, args...)
}
