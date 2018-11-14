package traverse

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	mdag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	mdagtest "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag/test"

	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
)

func TestDFSPreNoSkip(t *testing.T) {
	ds := mdagtest.Mock()
	opts := Options{Order: DFSPre, DAG: ds}

	testWalkOutputs(t, newFan(t, ds), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
`))

	testWalkOutputs(t, newLinkedList(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))

	testWalkOutputs(t, newBinaryTree(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
2 /a/aa/aab
1 /a/ab
2 /a/ab/aba
2 /a/ab/abb
`))

	testWalkOutputs(t, newBinaryDAG(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
`))
}

func TestDFSPreSkip(t *testing.T) {
	ds := mdagtest.Mock()
	opts := Options{Order: DFSPre, SkipDuplicates: true, DAG: ds}

	testWalkOutputs(t, newFan(t, ds), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
`))

	testWalkOutputs(t, newLinkedList(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))

	testWalkOutputs(t, newBinaryTree(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
2 /a/aa/aab
1 /a/ab
2 /a/ab/aba
2 /a/ab/abb
`))

	testWalkOutputs(t, newBinaryDAG(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))
}

func TestDFSPostNoSkip(t *testing.T) {
	ds := mdagtest.Mock()
	opts := Options{Order: DFSPost, DAG: ds}

	testWalkOutputs(t, newFan(t, ds), opts, []byte(`
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
0 /a
`))

	testWalkOutputs(t, newLinkedList(t, ds), opts, []byte(`
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
0 /a
`))

	testWalkOutputs(t, newBinaryTree(t, ds), opts, []byte(`
2 /a/aa/aaa
2 /a/aa/aab
1 /a/aa
2 /a/ab/aba
2 /a/ab/abb
1 /a/ab
0 /a
`))

	testWalkOutputs(t, newBinaryDAG(t, ds), opts, []byte(`
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
0 /a
`))
}

func TestDFSPostSkip(t *testing.T) {
	ds := mdagtest.Mock()
	opts := Options{Order: DFSPost, SkipDuplicates: true, DAG: ds}

	testWalkOutputs(t, newFan(t, ds), opts, []byte(`
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
0 /a
`))

	testWalkOutputs(t, newLinkedList(t, ds), opts, []byte(`
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
0 /a
`))

	testWalkOutputs(t, newBinaryTree(t, ds), opts, []byte(`
2 /a/aa/aaa
2 /a/aa/aab
1 /a/aa
2 /a/ab/aba
2 /a/ab/abb
1 /a/ab
0 /a
`))

	testWalkOutputs(t, newBinaryDAG(t, ds), opts, []byte(`
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
0 /a
`))
}

func TestBFSNoSkip(t *testing.T) {
	ds := mdagtest.Mock()
	opts := Options{Order: BFS, DAG: ds}

	testWalkOutputs(t, newFan(t, ds), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
`))

	testWalkOutputs(t, newLinkedList(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))

	testWalkOutputs(t, newBinaryTree(t, ds), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
2 /a/aa/aaa
2 /a/aa/aab
2 /a/ab/aba
2 /a/ab/abb
`))

	testWalkOutputs(t, newBinaryDAG(t, ds), opts, []byte(`
0 /a
1 /a/aa
1 /a/aa
2 /a/aa/aaa
2 /a/aa/aaa
2 /a/aa/aaa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
`))
}

func TestBFSSkip(t *testing.T) {
	ds := mdagtest.Mock()
	opts := Options{Order: BFS, SkipDuplicates: true, DAG: ds}

	testWalkOutputs(t, newFan(t, ds), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
`))

	testWalkOutputs(t, newLinkedList(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))

	testWalkOutputs(t, newBinaryTree(t, ds), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
2 /a/aa/aaa
2 /a/aa/aab
2 /a/ab/aba
2 /a/ab/abb
`))

	testWalkOutputs(t, newBinaryDAG(t, ds), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))
}

func testWalkOutputs(t *testing.T, root ipld.Node, opts Options, expect []byte) {
	expect = bytes.TrimLeft(expect, "\n")

	buf := new(bytes.Buffer)
	walk := func(current State) error {
		s := fmt.Sprintf("%d %s\n", current.Depth, current.Node.(*mdag.ProtoNode).Data())
		t.Logf("walk: %s", s)
		buf.Write([]byte(s))
		return nil
	}

	opts.Func = walk
	if err := Traverse(root, opts); err != nil {
		t.Error(err)
		return
	}

	actual := buf.Bytes()
	if !bytes.Equal(actual, expect) {
		t.Error("error: outputs differ")
		t.Logf("expect:\n%s", expect)
		t.Logf("actual:\n%s", actual)
	} else {
		t.Logf("expect matches actual:\n%s", expect)
	}
}

func newFan(t *testing.T, ds ipld.DAGService) ipld.Node {
	a := mdag.NodeWithData([]byte("/a"))
	addLink(t, ds, a, child(t, ds, a, "aa"))
	addLink(t, ds, a, child(t, ds, a, "ab"))
	addLink(t, ds, a, child(t, ds, a, "ac"))
	addLink(t, ds, a, child(t, ds, a, "ad"))
	return a
}

func newLinkedList(t *testing.T, ds ipld.DAGService) ipld.Node {
	a := mdag.NodeWithData([]byte("/a"))
	aa := child(t, ds, a, "aa")
	aaa := child(t, ds, aa, "aaa")
	aaaa := child(t, ds, aaa, "aaaa")
	aaaaa := child(t, ds, aaaa, "aaaaa")
	addLink(t, ds, aaaa, aaaaa)
	addLink(t, ds, aaa, aaaa)
	addLink(t, ds, aa, aaa)
	addLink(t, ds, a, aa)
	return a
}

func newBinaryTree(t *testing.T, ds ipld.DAGService) ipld.Node {
	a := mdag.NodeWithData([]byte("/a"))
	aa := child(t, ds, a, "aa")
	ab := child(t, ds, a, "ab")
	addLink(t, ds, aa, child(t, ds, aa, "aaa"))
	addLink(t, ds, aa, child(t, ds, aa, "aab"))
	addLink(t, ds, ab, child(t, ds, ab, "aba"))
	addLink(t, ds, ab, child(t, ds, ab, "abb"))
	addLink(t, ds, a, aa)
	addLink(t, ds, a, ab)
	return a
}

func newBinaryDAG(t *testing.T, ds ipld.DAGService) ipld.Node {
	a := mdag.NodeWithData([]byte("/a"))
	aa := child(t, ds, a, "aa")
	aaa := child(t, ds, aa, "aaa")
	aaaa := child(t, ds, aaa, "aaaa")
	aaaaa := child(t, ds, aaaa, "aaaaa")
	addLink(t, ds, aaaa, aaaaa)
	addLink(t, ds, aaaa, aaaaa)
	addLink(t, ds, aaa, aaaa)
	addLink(t, ds, aaa, aaaa)
	addLink(t, ds, aa, aaa)
	addLink(t, ds, aa, aaa)
	addLink(t, ds, a, aa)
	addLink(t, ds, a, aa)
	return a
}

func addLink(t *testing.T, ds ipld.DAGService, a, b ipld.Node) {
	to := string(a.(*mdag.ProtoNode).Data()) + "2" + string(b.(*mdag.ProtoNode).Data())
	if err := ds.Add(context.Background(), b); err != nil {
		t.Error(err)
	}
	if err := a.(*mdag.ProtoNode).AddNodeLink(to, b.(*mdag.ProtoNode)); err != nil {
		t.Error(err)
	}
}

func child(t *testing.T, ds ipld.DAGService, a ipld.Node, name string) ipld.Node {
	return mdag.NodeWithData([]byte(string(a.(*mdag.ProtoNode).Data()) + "/" + name))
}
