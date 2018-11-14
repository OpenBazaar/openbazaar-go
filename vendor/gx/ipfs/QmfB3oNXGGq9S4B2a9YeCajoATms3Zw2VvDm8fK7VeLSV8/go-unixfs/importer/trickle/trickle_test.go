package trickle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"testing"

	ft "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs"
	h "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs/importer/helpers"
	uio "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs/io"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	merkledag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	mdtest "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag/test"
	chunker "gx/ipfs/QmTUTG9Jg9ZRA1EzTPGTDvnwfcfKhDMnqANnP9fe4rSjMR/go-ipfs-chunker"
)

type UseRawLeaves bool

const (
	ProtoBufLeaves UseRawLeaves = false
	RawLeaves      UseRawLeaves = true
)

func runBothSubtests(t *testing.T, tfunc func(*testing.T, UseRawLeaves)) {
	t.Run("leaves=ProtoBuf", func(t *testing.T) { tfunc(t, ProtoBufLeaves) })
	t.Run("leaves=Raw", func(t *testing.T) { tfunc(t, RawLeaves) })
}

func buildTestDag(ds ipld.DAGService, spl chunker.Splitter, rawLeaves UseRawLeaves) (*merkledag.ProtoNode, error) {
	dbp := h.DagBuilderParams{
		Dagserv:   ds,
		Maxlinks:  h.DefaultLinksPerBlock,
		RawLeaves: bool(rawLeaves),
	}

	nd, err := Layout(dbp.New(spl))
	if err != nil {
		return nil, err
	}

	pbnd, ok := nd.(*merkledag.ProtoNode)
	if !ok {
		return nil, merkledag.ErrNotProtobuf
	}

	return pbnd, VerifyTrickleDagStructure(pbnd, VerifyParams{
		Getter:      ds,
		Direct:      dbp.Maxlinks,
		LayerRepeat: layerRepeat,
		RawLeaves:   bool(rawLeaves),
	})
}

//Test where calls to read are smaller than the chunk size
func TestSizeBasedSplit(t *testing.T) {
	runBothSubtests(t, testSizeBasedSplit)
}

func testSizeBasedSplit(t *testing.T, rawLeaves UseRawLeaves) {
	if testing.Short() {
		t.SkipNow()
	}
	bs := chunker.SizeSplitterGen(512)
	testFileConsistency(t, bs, 32*512, rawLeaves)

	bs = chunker.SizeSplitterGen(4096)
	testFileConsistency(t, bs, 32*4096, rawLeaves)

	// Uneven offset
	testFileConsistency(t, bs, 31*4095, rawLeaves)
}

func dup(b []byte) []byte {
	o := make([]byte, len(b))
	copy(o, b)
	return o
}

func testFileConsistency(t *testing.T, bs chunker.SplitterGen, nbytes int, rawLeaves UseRawLeaves) {
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, bs(read), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	r, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuilderConsistency(t *testing.T) {
	runBothSubtests(t, testBuilderConsistency)
}

func testBuilderConsistency(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := 100000
	buf := new(bytes.Buffer)
	io.CopyN(buf, u.NewTimeSeededRand(), int64(nbytes))
	should := dup(buf.Bytes())
	dagserv := mdtest.Mock()
	nd, err := buildTestDag(dagserv, chunker.DefaultSplitter(buf), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}
	r, err := uio.NewDagReader(context.Background(), nd, dagserv)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}

func arrComp(a, b []byte) error {
	if len(a) != len(b) {
		return fmt.Errorf("arrays differ in length. %d != %d", len(a), len(b))
	}
	for i, v := range a {
		if v != b[i] {
			return fmt.Errorf("arrays differ at index: %d", i)
		}
	}
	return nil
}

func TestIndirectBlocks(t *testing.T) {
	runBothSubtests(t, testIndirectBlocks)
}

func testIndirectBlocks(t *testing.T, rawLeaves UseRawLeaves) {
	splitter := chunker.SizeSplitterGen(512)
	nbytes := 1024 * 1024
	buf := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(buf)

	read := bytes.NewReader(buf)

	ds := mdtest.Mock()
	dag, err := buildTestDag(ds, splitter(read), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	reader, err := uio.NewDagReader(context.Background(), dag, ds)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, buf) {
		t.Fatal("Not equal!")
	}
}

func TestSeekingBasic(t *testing.T) {
	runBothSubtests(t, testSeekingBasic)
}

func testSeekingBasic(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := int64(10 * 1024)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 512), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	start := int64(4000)
	n, err := rs.Seek(start, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if n != start {
		t.Fatal("Failed to seek to correct offset")
	}

	out, err := ioutil.ReadAll(rs)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should[start:])
	if err != nil {
		t.Fatal(err)
	}
}

func TestSeekToBegin(t *testing.T) {
	runBothSubtests(t, testSeekToBegin)
}

func testSeekToBegin(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := int64(10 * 1024)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 500), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	n, err := io.CopyN(ioutil.Discard, rs, 1024*4)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4096 {
		t.Fatal("Copy didnt copy enough bytes")
	}

	seeked, err := rs.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if seeked != 0 {
		t.Fatal("Failed to seek to beginning")
	}

	out, err := ioutil.ReadAll(rs)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSeekToAlmostBegin(t *testing.T) {
	runBothSubtests(t, testSeekToAlmostBegin)
}

func testSeekToAlmostBegin(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := int64(10 * 1024)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 500), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	n, err := io.CopyN(ioutil.Discard, rs, 1024*4)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4096 {
		t.Fatal("Copy didnt copy enough bytes")
	}

	seeked, err := rs.Seek(1, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if seeked != 1 {
		t.Fatal("Failed to seek to almost beginning")
	}

	out, err := ioutil.ReadAll(rs)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should[1:])
	if err != nil {
		t.Fatal(err)
	}
}

func TestSeekEnd(t *testing.T) {
	runBothSubtests(t, testSeekEnd)
}

func testSeekEnd(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := int64(50 * 1024)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 500), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	seeked, err := rs.Seek(0, io.SeekEnd)
	if err != nil {
		t.Fatal(err)
	}
	if seeked != nbytes {
		t.Fatal("Failed to seek to end")
	}
}

func TestSeekEndSingleBlockFile(t *testing.T) {
	runBothSubtests(t, testSeekEndSingleBlockFile)
}

func testSeekEndSingleBlockFile(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := int64(100)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 5000), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	seeked, err := rs.Seek(0, io.SeekEnd)
	if err != nil {
		t.Fatal(err)
	}
	if seeked != nbytes {
		t.Fatal("Failed to seek to end")
	}
}

func TestSeekingStress(t *testing.T) {
	runBothSubtests(t, testSeekingStress)
}

func testSeekingStress(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := int64(1024 * 1024)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 1000), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	testbuf := make([]byte, nbytes)
	for i := 0; i < 50; i++ {
		offset := mrand.Intn(int(nbytes))
		l := int(nbytes) - offset
		n, err := rs.Seek(int64(offset), io.SeekStart)
		if err != nil {
			t.Fatal(err)
		}
		if n != int64(offset) {
			t.Fatal("Seek failed to move to correct position")
		}

		nread, err := rs.Read(testbuf[:l])
		if err != nil {
			t.Fatal(err)
		}
		if nread != l {
			t.Fatal("Failed to read enough bytes")
		}

		err = arrComp(testbuf[:l], should[offset:offset+l])
		if err != nil {
			t.Fatal(err)
		}
	}

}

func TestSeekingConsistency(t *testing.T) {
	runBothSubtests(t, testSeekingConsistency)
}

func testSeekingConsistency(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := int64(128 * 1024)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 500), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	out := make([]byte, nbytes)

	for coff := nbytes - 4096; coff >= 0; coff -= 4096 {
		t.Log(coff)
		n, err := rs.Seek(coff, io.SeekStart)
		if err != nil {
			t.Fatal(err)
		}
		if n != coff {
			t.Fatal("wasnt able to seek to the right position")
		}
		nread, err := rs.Read(out[coff : coff+4096])
		if err != nil {
			t.Fatal(err)
		}
		if nread != 4096 {
			t.Fatal("didnt read the correct number of bytes")
		}
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppend(t *testing.T) {
	runBothSubtests(t, testAppend)
}

func testAppend(t *testing.T, rawLeaves UseRawLeaves) {
	nbytes := int64(128 * 1024)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	// Reader for half the bytes
	read := bytes.NewReader(should[:nbytes/2])
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 500), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	dbp := &h.DagBuilderParams{
		Dagserv:   ds,
		Maxlinks:  h.DefaultLinksPerBlock,
		RawLeaves: bool(rawLeaves),
	}

	r := bytes.NewReader(should[nbytes/2:])

	ctx := context.Background()
	nnode, err := Append(ctx, nd, dbp.New(chunker.NewSizeSplitter(r, 500)))
	if err != nil {
		t.Fatal(err)
	}

	err = VerifyTrickleDagStructure(nnode, VerifyParams{
		Getter:      ds,
		Direct:      dbp.Maxlinks,
		LayerRepeat: layerRepeat,
		RawLeaves:   bool(rawLeaves),
	})
	if err != nil {
		t.Fatal(err)
	}

	fread, err := uio.NewDagReader(ctx, nnode, ds)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(fread)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}

// This test appends one byte at a time to an empty file
func TestMultipleAppends(t *testing.T) {
	runBothSubtests(t, testMultipleAppends)
}

func testMultipleAppends(t *testing.T, rawLeaves UseRawLeaves) {
	ds := mdtest.Mock()

	// TODO: fix small size appends and make this number bigger
	nbytes := int64(1000)
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(nil)
	nd, err := buildTestDag(ds, chunker.NewSizeSplitter(read, 500), rawLeaves)
	if err != nil {
		t.Fatal(err)
	}

	dbp := &h.DagBuilderParams{
		Dagserv:   ds,
		Maxlinks:  4,
		RawLeaves: bool(rawLeaves),
	}

	spl := chunker.SizeSplitterGen(500)

	ctx := context.Background()
	for i := 0; i < len(should); i++ {

		nnode, err := Append(ctx, nd, dbp.New(spl(bytes.NewReader(should[i:i+1]))))
		if err != nil {
			t.Fatal(err)
		}

		err = VerifyTrickleDagStructure(nnode, VerifyParams{
			Getter:      ds,
			Direct:      dbp.Maxlinks,
			LayerRepeat: layerRepeat,
			RawLeaves:   bool(rawLeaves),
		})
		if err != nil {
			t.Fatal(err)
		}

		fread, err := uio.NewDagReader(ctx, nnode, ds)
		if err != nil {
			t.Fatal(err)
		}

		out, err := ioutil.ReadAll(fread)
		if err != nil {
			t.Fatal(err)
		}

		err = arrComp(out, should[:i+1])
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestAppendSingleBytesToEmpty(t *testing.T) {
	ds := mdtest.Mock()

	data := []byte("AB")

	nd := new(merkledag.ProtoNode)
	nd.SetData(ft.FilePBData(nil, 0))

	dbp := &h.DagBuilderParams{
		Dagserv:  ds,
		Maxlinks: 4,
	}

	spl := chunker.SizeSplitterGen(500)

	ctx := context.Background()
	nnode, err := Append(ctx, nd, dbp.New(spl(bytes.NewReader(data[:1]))))
	if err != nil {
		t.Fatal(err)
	}

	nnode, err = Append(ctx, nnode, dbp.New(spl(bytes.NewReader(data[1:]))))
	if err != nil {
		t.Fatal(err)
	}

	fread, err := uio.NewDagReader(ctx, nnode, ds)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(fread)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(out, data)
	err = arrComp(out, data)
	if err != nil {
		t.Fatal(err)
	}
}
