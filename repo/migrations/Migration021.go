package migrations

import "fmt"

type Migration021 struct{}

func (Migration021) Up(repoPath, dbPassword string, testnet bool) error {
	if err := writeRepoVer(repoPath, 22); err != nil {
		return fmt.Errorf("bumping repover to 22: %s", err.Error())
	}
	return nil
}

func (Migration021) Down(repoPath, dbPassword string, testnet bool) error {
	if err := writeRepoVer(repoPath, 21); err != nil {
		return fmt.Errorf("dropping repover to 21: %s", err.Error())
	}
	return nil
}
