package merkledag_test

import (
	"bytes"
	"context"
	"testing"

	. "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	mdtest "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag/test"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
)

func TestStableCID(t *testing.T) {
	nd := &ProtoNode{}
	nd.SetData([]byte("foobar"))
	nd.SetLinks([]*ipld.Link{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	})
	expected, err := cid.Decode("QmSN3WED2xPLbYvBbfvew2ZLtui8EbFYYcbfkpKH5jwG9C")
	if err != nil {
		t.Fatal(err)
	}
	if !nd.Cid().Equals(expected) {
		t.Fatalf("Got CID %s, expected CID %s", nd.Cid(), expected)
	}
}

func TestRemoveLink(t *testing.T) {
	nd := &ProtoNode{}
	nd.SetLinks([]*ipld.Link{
		{Name: "a"},
		{Name: "b"},
		{Name: "a"},
		{Name: "a"},
		{Name: "c"},
		{Name: "a"},
	})

	err := nd.RemoveNodeLink("a")
	if err != nil {
		t.Fatal(err)
	}

	if len(nd.Links()) != 2 {
		t.Fatal("number of links incorrect")
	}

	if nd.Links()[0].Name != "b" {
		t.Fatal("link order wrong")
	}

	if nd.Links()[1].Name != "c" {
		t.Fatal("link order wrong")
	}

	// should fail
	err = nd.RemoveNodeLink("a")
	if err != ipld.ErrNotFound {
		t.Fatal("should have failed to remove link")
	}

	// ensure nothing else got touched
	if len(nd.Links()) != 2 {
		t.Fatal("number of links incorrect")
	}

	if nd.Links()[0].Name != "b" {
		t.Fatal("link order wrong")
	}

	if nd.Links()[1].Name != "c" {
		t.Fatal("link order wrong")
	}
}

func TestFindLink(t *testing.T) {
	ctx := context.Background()

	ds := mdtest.Mock()
	ndEmpty := new(ProtoNode)
	err := ds.Add(ctx, ndEmpty)
	if err != nil {
		t.Fatal(err)
	}

	kEmpty := ndEmpty.Cid()

	nd := &ProtoNode{}
	nd.SetLinks([]*ipld.Link{
		{Name: "a", Cid: kEmpty},
		{Name: "c", Cid: kEmpty},
		{Name: "b", Cid: kEmpty},
	})

	err = ds.Add(ctx, nd)
	if err != nil {
		t.Fatal(err)
	}

	lnk, err := nd.GetNodeLink("b")
	if err != nil {
		t.Fatal(err)
	}

	if lnk.Name != "b" {
		t.Fatal("got wrong link back")
	}

	_, err = nd.GetNodeLink("f")
	if err != ErrLinkNotFound {
		t.Fatal("shouldnt have found link")
	}

	_, err = nd.GetLinkedNode(context.Background(), ds, "b")
	if err != nil {
		t.Fatal(err)
	}

	outnd, err := nd.UpdateNodeLink("b", nd)
	if err != nil {
		t.Fatal(err)
	}

	olnk, err := outnd.GetNodeLink("b")
	if err != nil {
		t.Fatal(err)
	}

	if olnk.Cid.String() == kEmpty.String() {
		t.Fatal("new link should have different hash")
	}
}

func TestNodeCopy(t *testing.T) {
	nd := &ProtoNode{}
	nd.SetLinks([]*ipld.Link{
		{Name: "a"},
		{Name: "c"},
		{Name: "b"},
	})

	nd.SetData([]byte("testing"))

	ond := nd.Copy().(*ProtoNode)
	ond.SetData(nil)

	if nd.Data() == nil {
		t.Fatal("should be different objects")
	}
}

func TestJsonRoundtrip(t *testing.T) {
	nd := new(ProtoNode)
	nd.SetLinks([]*ipld.Link{
		{Name: "a"},
		{Name: "c"},
		{Name: "b"},
	})
	nd.SetData([]byte("testing"))

	jb, err := nd.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	nn := new(ProtoNode)
	err = nn.UnmarshalJSON(jb)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(nn.Data(), nd.Data()) {
		t.Fatal("data wasnt the same")
	}

	if !nn.Cid().Equals(nd.Cid()) {
		t.Fatal("objects differed after marshaling")
	}
}
