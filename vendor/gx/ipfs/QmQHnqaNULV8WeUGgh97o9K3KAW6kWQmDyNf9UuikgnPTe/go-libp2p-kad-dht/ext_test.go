package dht

import (
	"context"
	"math/rand"
	"testing"
	"time"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	pb "gx/ipfs/QmQHnqaNULV8WeUGgh97o9K3KAW6kWQmDyNf9UuikgnPTe/go-libp2p-kad-dht/pb"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	mocknet "gx/ipfs/QmUDTcnDp2WssbmiDLC6aYurUeyt7QeRakHUQMxA2mZ5iB/go-libp2p/p2p/net/mock"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
	record "gx/ipfs/Qma9Eqp16mNHDX1EL73pcxhFfzbyXVcAYtaDd1xdmDRDtL/go-libp2p-record"
	routing "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing"
	ggio "gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/io"
)

func TestGetFailures(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	hosts := mn.Hosts()

	d, err := New(ctx, hosts[0])
	if err != nil {
		t.Fatal(err)
	}
	d.Update(ctx, hosts[1].ID())

	// Reply with failures to every message
	hosts[1].SetStreamHandler(d.protocols[0], func(s inet.Stream) {
		time.Sleep(400 * time.Millisecond)
		s.Close()
	})

	// This one should time out
	ctx1, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := d.GetValue(ctx1, "test"); err != nil {
		if merr, ok := err.(u.MultiErr); ok && len(merr) > 0 {
			err = merr[0]
		}

		if err != context.DeadlineExceeded {
			t.Fatal("Got different error than we expected", err)
		}
	} else {
		t.Fatal("Did not get expected error!")
	}

	t.Log("Timeout test passed.")

	// Reply with failures to every message
	hosts[1].SetStreamHandler(d.protocols[0], func(s inet.Stream) {
		defer s.Close()

		pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
		pbw := ggio.NewDelimitedWriter(s)

		pmes := new(pb.Message)
		if err := pbr.ReadMsg(pmes); err != nil {
			panic(err)
		}

		resp := &pb.Message{
			Type: pmes.Type,
		}
		if err := pbw.WriteMsg(resp); err != nil {
			panic(err)
		}
	})

	// This one should fail with NotFound.
	// long context timeout to ensure we dont end too early.
	// the dht should be exhausting its query and returning not found.
	// (was 3 seconds before which should be _plenty_ of time, but maybe
	// travis machines really have a hard time...)
	ctx2, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, err = d.GetValue(ctx2, "test")
	if err != nil {
		if merr, ok := err.(u.MultiErr); ok && len(merr) > 0 {
			err = merr[0]
		}
		if err != routing.ErrNotFound {
			t.Fatalf("Expected ErrNotFound, got: %s", err)
		}
	} else {
		t.Fatal("expected error, got none.")
	}

	t.Log("ErrNotFound check passed!")

	// Now we test this DHT's handleGetValue failure
	{
		typ := pb.Message_GET_VALUE
		str := "hello"

		rec := record.MakePutRecord(str, []byte("blah"))
		req := pb.Message{
			Type:   typ,
			Key:    []byte(str),
			Record: rec,
		}

		s, err := hosts[1].NewStream(context.Background(), hosts[0].ID(), d.protocols[0])
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
		pbw := ggio.NewDelimitedWriter(s)

		if err := pbw.WriteMsg(&req); err != nil {
			t.Fatal(err)
		}

		pmes := new(pb.Message)
		if err := pbr.ReadMsg(pmes); err != nil {
			t.Fatal(err)
		}
		if pmes.GetRecord() != nil {
			t.Fatal("shouldnt have value")
		}
		if pmes.GetProviderPeers() != nil {
			t.Fatal("shouldnt have provider peers")
		}
	}
}

func TestNotFound(t *testing.T) {
	// t.Skip("skipping test to debug another")
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 16)
	if err != nil {
		t.Fatal(err)
	}
	hosts := mn.Hosts()
	d, err := New(ctx, hosts[0])
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range hosts {
		d.Update(ctx, p.ID())
	}

	// Reply with random peers to every message
	for _, host := range hosts {
		host := host // shadow loop var
		host.SetStreamHandler(d.protocols[0], func(s inet.Stream) {
			defer s.Close()

			pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
			pbw := ggio.NewDelimitedWriter(s)

			pmes := new(pb.Message)
			if err := pbr.ReadMsg(pmes); err != nil {
				panic(err)
			}

			switch pmes.GetType() {
			case pb.Message_GET_VALUE:
				resp := &pb.Message{Type: pmes.Type}

				ps := []pstore.PeerInfo{}
				for i := 0; i < 7; i++ {
					p := hosts[rand.Intn(len(hosts))].ID()
					pi := host.Peerstore().PeerInfo(p)
					ps = append(ps, pi)
				}

				resp.CloserPeers = pb.PeerInfosToPBPeers(d.host.Network(), ps)
				if err := pbw.WriteMsg(resp); err != nil {
					panic(err)
				}
			default:
				panic("Shouldnt recieve this.")
			}
		})
	}

	// long timeout to ensure timing is not at play.
	ctx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()
	v, err := d.GetValue(ctx, "hello")
	log.Debugf("get value got %v", v)
	if err != nil {
		if merr, ok := err.(u.MultiErr); ok && len(merr) > 0 {
			err = merr[0]
		}
		switch err {
		case routing.ErrNotFound:
			//Success!
			return
		case u.ErrTimeout:
			t.Fatal("Should not have gotten timeout!")
		default:
			t.Fatalf("Got unexpected error: %s", err)
		}
	}
	t.Fatal("Expected to recieve an error.")
}

// If less than K nodes are in the entire network, it should fail when we make
// a GET rpc and nobody has the value
func TestLessThanKResponses(t *testing.T) {
	// t.Skip("skipping test to debug another")
	// t.Skip("skipping test because it makes a lot of output")

	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	hosts := mn.Hosts()

	d, err := New(ctx, hosts[0])
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i < 5; i++ {
		d.Update(ctx, hosts[i].ID())
	}

	// Reply with random peers to every message
	for _, host := range hosts {
		host := host // shadow loop var
		host.SetStreamHandler(d.protocols[0], func(s inet.Stream) {
			defer s.Close()

			pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
			pbw := ggio.NewDelimitedWriter(s)

			pmes := new(pb.Message)
			if err := pbr.ReadMsg(pmes); err != nil {
				panic(err)
			}

			switch pmes.GetType() {
			case pb.Message_GET_VALUE:
				pi := host.Peerstore().PeerInfo(hosts[1].ID())
				resp := &pb.Message{
					Type:        pmes.Type,
					CloserPeers: pb.PeerInfosToPBPeers(d.host.Network(), []pstore.PeerInfo{pi}),
				}

				if err := pbw.WriteMsg(resp); err != nil {
					panic(err)
				}
			default:
				panic("Shouldnt recieve this.")
			}

		})
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	if _, err := d.GetValue(ctx, "hello"); err != nil {
		switch err {
		case routing.ErrNotFound:
			//Success!
			return
		case u.ErrTimeout:
			t.Fatal("Should not have gotten timeout!")
		default:
			t.Fatalf("Got unexpected error: %s", err)
		}
	}
	t.Fatal("Expected to recieve an error.")
}

// Test multiple queries against a node that closes its stream after every query.
func TestMultipleQueries(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	hosts := mn.Hosts()
	d, err := New(ctx, hosts[0])
	if err != nil {
		t.Fatal(err)
	}

	d.Update(ctx, hosts[1].ID())

	// It would be nice to be able to just get a value and succeed but then
	// we'd need to deal with selectors and validators...
	hosts[1].SetStreamHandler(d.protocols[0], func(s inet.Stream) {
		defer s.Close()

		pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
		pbw := ggio.NewDelimitedWriter(s)

		pmes := new(pb.Message)
		if err := pbr.ReadMsg(pmes); err != nil {
			panic(err)
		}

		switch pmes.GetType() {
		case pb.Message_GET_VALUE:
			pi := hosts[1].Peerstore().PeerInfo(hosts[0].ID())
			resp := &pb.Message{
				Type:        pmes.Type,
				CloserPeers: pb.PeerInfosToPBPeers(d.host.Network(), []pstore.PeerInfo{pi}),
			}

			if err := pbw.WriteMsg(resp); err != nil {
				panic(err)
			}
		default:
			panic("Shouldnt recieve this.")
		}
	})

	// long timeout to ensure timing is not at play.
	ctx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()
	for i := 0; i < 10; i++ {
		if _, err := d.GetValue(ctx, "hello"); err != nil {
			switch err {
			case routing.ErrNotFound:
				//Success!
				continue
			case u.ErrTimeout:
				t.Fatal("Should not have gotten timeout!")
			default:
				t.Fatalf("Got unexpected error: %s", err)
			}
		}
		t.Fatal("Expected to recieve an error.")
	}
}
