package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	ps "gx/ipfs/QmWp2mA7eab53PS4NdyW4uvBf73ZB7wSB7eN64K5pS1AKg/go-peerstream"

	tpt "github.com/libp2p/go-tcp-transport"
	yamux "github.com/whyrusleeping/go-smux-yamux"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
)

func die(err error) {
	fmt.Fprintf(os.Stderr, "error: %s\n", err)
	os.Exit(1)
}

func main() {
	// create a new Swarm
	swarm := ps.NewSwarm(yamux.DefaultTransport)
	defer swarm.Close()

	// tell swarm what to do with a new incoming streams.
	// EchoHandler just echos back anything they write.
	swarm.SetStreamHandler(ps.EchoHandler)

	tr := tpt.NewTCPTransport()
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/12001")
	if err != nil {
		die(err)
	}
	l, err := tr.Listen(addr)
	if err != nil {
		die(err)
	}

	if _, err := swarm.AddListener(l); err != nil {
		die(err)
	}

	dialer, err := tr.Dialer(addr)
	if err != nil {
		die(err)
	}
	nc, err := dialer.Dial(addr)
	if err != nil {
		die(err)
	}

	c, err := swarm.AddConn(nc)
	if err != nil {
		die(err)
	}

	nRcvStream := 0
	bio := bufio.NewReader(os.Stdin)
	swarm.SetStreamHandler(func(s *ps.Stream) {
		log("handling new stream %d", nRcvStream)
		nRcvStream++

		line, err := bio.ReadString('\n')
		if err != nil {
			die(err)
		}
		_ = line
		// line = "read: " + line
		// s.Write([]byte(line))
		s.Close()
	})

	nSndStream := 0
	for {
		<-time.After(200 * time.Millisecond)
		_, err := swarm.NewStreamWithConn(c)
		if err != nil {
			die(err)
		}
		log("sender got new stream %d", nSndStream)
		nSndStream++
	}
}

func log(s string, ifs ...interface{}) {
	fmt.Fprintf(os.Stderr, s+"\n", ifs...)
}
