package core

import (
	"time"

	"github.com/op/go-logging"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

const (
	scannerTestingInterval = time.Duration(1) * time.Minute
	scannerRegularInterval = time.Duration(10) * time.Minute
)

type inboundMessageScanner struct {
	// PerformTask dependencies
	datastore  repo.Datastore
	service    net.NetworkService
	getHandler func(t pb.Message_MessageType) func(peer.ID, *pb.Message, interface{}) (*pb.Message, error)
	extractID  func([]byte) (*peer.ID, error)
	broadcast  chan repo.Notifier

	// Worker-handling dependencies
	intervalDelay time.Duration
	logger        *logging.Logger
	watchdogTimer *time.Ticker
	stopWorker    chan bool
}

func peerIDExtractor(data []byte) (*peer.ID, error) {
	i := peer.ID(data)
	return &i, nil
}

// StartInboundMsgScanner - start the notifier
func (n *OpenBazaarNode) StartInboundMsgScanner() {
	n.InboundMsgScanner = &inboundMessageScanner{
		datastore:     n.Datastore,
		service:       n.Service,
		getHandler:    n.Service.HandlerForMsgType,
		extractID:     peerIDExtractor,
		broadcast:     n.Broadcast,
		intervalDelay: n.scannerIntervalDelay(),
		logger:        logging.MustGetLogger("inboundMessageScanner"),
	}
	go n.InboundMsgScanner.Run()
}

func (n *OpenBazaarNode) scannerIntervalDelay() time.Duration {
	if n.TestnetEnable {
		return scannerTestingInterval
	}
	return scannerRegularInterval
}

func (scanner *inboundMessageScanner) Run() {
	scanner.watchdogTimer = time.NewTicker(scanner.intervalDelay)
	scanner.stopWorker = make(chan bool)

	// Run once on start, then wait for watchdog
	scanner.PerformTask()
	for {
		select {
		case <-scanner.watchdogTimer.C:
			scanner.PerformTask()
		case <-scanner.stopWorker:
			scanner.watchdogTimer.Stop()
			return
		}
	}
}

func (scanner *inboundMessageScanner) Stop() {
	scanner.stopWorker <- true
	close(scanner.stopWorker)
}

func (scanner *inboundMessageScanner) PerformTask() {
	msgs, err := scanner.datastore.Messages().GetAllErrored()
	if err != nil {
		scanner.logger.Error(err)
		return
	}
	for _, m := range msgs {
		if m.MsgErr == ErrInsufficientFunds.Error() {
			// Get handler for this msg type
			handler := scanner.getHandler(pb.Message_MessageType(m.MessageType))
			if handler == nil {
				scanner.logger.Errorf("err fetching handler for msg: %v", pb.Message_MessageType(m.MessageType))
				continue
			}
			i, err := scanner.extractID(m.PeerPubkey)
			if err != nil {
				scanner.logger.Errorf("Error processing message %s. Type %s: %s", m, m.MessageType, err.Error())
				continue

			}
			msg := new(repo.Message)

			if len(m.Message) > 0 {
				err = msg.UnmarshalJSON(m.Message)
				if err != nil {
					scanner.logger.Errorf("Error processing message %s. Type %s: %s", m, m.MessageType, err.Error())
					continue
				}
			}
			// Dispatch handler
			_, err = handler(*i, &msg.Msg, nil)
			if err != nil {
				scanner.logger.Errorf("%d handle message error from %s: %s", m.MessageType, m.PeerID, err)
				continue
			}
			err = scanner.datastore.Messages().MarkAsResolved(m)
			if err != nil {
				scanner.logger.Errorf("marking message resolved: %s", err)
			}
		}
	}
}
