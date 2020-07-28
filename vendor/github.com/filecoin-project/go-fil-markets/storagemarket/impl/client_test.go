package storageimpl_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	storageimpl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
)

func TestClient_Configure(t *testing.T) {
	c := &storageimpl.Client{}
	assert.Equal(t, time.Duration(0), c.PollingInterval())

	c.Configure(storageimpl.DealPollingInterval(123 * time.Second))

	assert.Equal(t, 123*time.Second, c.PollingInterval())
}
