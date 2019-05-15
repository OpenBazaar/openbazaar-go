package migrations

// MigrationNoOp is used for creating placeholder migrations which
// are implemented in another branch which is yet to be merged.
type MigrationNoOp struct{}

func (MigrationNoOp) Up(path, password string, testEnabled bool) error   { return nil }
func (MigrationNoOp) Down(path, password string, testEnabled bool) error { return nil }
