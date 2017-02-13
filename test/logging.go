package test

import (
	"os"

	logging "github.com/op/go-logging"
)

func NewLogger() logging.LeveledBackend {
	return logging.MultiLogger(logging.NewLogBackend(os.Stderr, "", 0))
}
