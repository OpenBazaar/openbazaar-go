package chunker_test

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	chunker "gx/ipfs/QmZooytqEoUwQjv7KzH4d3xyJnyvD3AWJaCDMYt5pbCtua/chunker"
)

func parseDigest(s string) []byte {
	d, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return d
}

type chunk struct {
	Length uint64
	CutFP  uint64
	Digest []byte
}

// polynomial used for all the tests below
const testPol = chunker.Pol(0x3DA3358B4DC173)

// created for 32MB of random data out of math/rand's Uint32() seeded by
// constant 23
//
// chunking configuration:
// window size 64, avg chunksize 1<<20, min chunksize 1<<19, max chunksize 1<<23
// polynom 0x3DA3358B4DC173
var chunks1 = []chunk{
	chunk{1579758, 0x001d06f777d00000, parseDigest("2931d6dff887a8597b333fd13fcade3ee0c11ad747c5a0eb5ebe7f9d86ab4e42")},
	chunk{1089328, 0x001c99190f500000, parseDigest("c8a84f85953a2bfbc5555d43e570f5ffa8ca7f0db3a2bc6e47f9e5da06130313")},
	chunk{614316, 0x0002e95018f00000, parseDigest("2f99d99fd70f6bcbe01a945c81a5bb355ea8d82a368c82b6bbd57c81df0173db")},
	chunk{1183251, 0x00187291a9d00000, parseDigest("56db29489e7b8f81e9076d451db131a2501264452509ec5dd2732c5ca3d043da")},
	chunk{2079238, 0x00150d9af0e00000, parseDigest("9cd438ed06933b4d3e1ccbfef9e5b26d3e98171c557ed56a841d20d6b31cc1bb")},
	chunk{1282082, 0x0001d7df88a00000, parseDigest("63284dafb2da8040f01ee8c68e431d0e5c63fbc4c94e72826af3bbf9458edfee")},
	chunk{1656779, 0x001881427eb00000, parseDigest("31b07bddd5d4baf0c7a9d77e66b5134595a4b57ed9e10864dbae9a6cc8511361")},
	chunk{1053264, 0x0006e8249a400000, parseDigest("b630ea907fcae93b3b2da07de81d885eadb44b5a61e9d8d34d256c42acd54faf")},
	chunk{1028060, 0x00179e806ee00000, parseDigest("f91c9aa5ad51515ba9d2b22b88ce6064a5bca7d11a6233d641aaf4226b26f991")},
	chunk{896166, 0x001a90cfa7900000, parseDigest("b3b0faf1476b77ae20aecec498faa99d8743b6057d99cb35f8b8f3e178970f32")},
	chunk{1633016, 0x001c77ba2dc00000, parseDigest("7dfc1baf511bb19d003d7fde2718296681133a51995b4b7497381b4c7551f0ff")},
	chunk{1051769, 0x001c7ee3e8900000, parseDigest("e5cd24f70553b4cec0ff26e756a634d374c50280b21ed199d7dcc6f6c9d5b185")},
	chunk{5719810, 0x00004e5c0ca00000, parseDigest("21b576c8340813e1a3b517b5727ed256e3a13a060fa3b5a2b05279fc625dddce")},
	chunk{2490321, 0x0014eedcd5900000, parseDigest("47adb6a6cbf2ecfe18e1b90f03b19f8155bb7222d5ca264698c1f94666224043")},
	chunk{544210, 0x00063fcba2b00000, parseDigest("2a55f181b50a2af4efa85e2cb43479980490519537b85b945238cea33d682233")},
	chunk{892726, 0x000b6da997300000, parseDigest("10c413cd7eddfd4d5577984cc137fe25d59b357fe282aa4d6b085482eefc1127")},
	chunk{1147747, 0x000745edc1700000, parseDigest("c39a798ca1516b0bf41a925de18bc225df2c51078d0c5330a4230c0919c64a32")},
	chunk{1964963, 0x000a609a0f900000, parseDigest("e44ad9856707b191144ee597b845f8253906c3e1b4112cc28c49bf196e5f5be7")},
	chunk{798928, 0x000ed5aa9e200000, parseDigest("7bb618d427aa47512c414a682496e861c81c905deac8434fc55c1caf34655501")},
	chunk{2585098, 0x00108d5fd0500000, parseDigest("678291ddd056c63af055f206bf59c4e773036e2f67564edd3c159729586d1275")},
	chunk{975521, 0x000603e82f500000, parseDigest("faa1244cca467335cd3c04e4a9f9c2d72fa49b7e9d6c6392a68923d1372a16c0")},
	chunk{1288081, 0x00097e6e5e2e0920, parseDigest("857eab0067640c83275d628d2f22c027855afd8aa816bb1f15e6233d19fa8cea")},
}

// test if nullbytes are correctly split, even if length is a multiple of MinSize.
var chunks2 = []chunk{
	chunk{chunker.MinSize, 0, parseDigest("07854d2fef297a06ba81685e660c332de36d5d18d546927d30daad6d7fda1541")},
	chunk{chunker.MinSize, 0, parseDigest("07854d2fef297a06ba81685e660c332de36d5d18d546927d30daad6d7fda1541")},
	chunk{chunker.MinSize, 0, parseDigest("07854d2fef297a06ba81685e660c332de36d5d18d546927d30daad6d7fda1541")},
	chunk{chunker.MinSize, 0, parseDigest("07854d2fef297a06ba81685e660c332de36d5d18d546927d30daad6d7fda1541")},
}

func testWithData(t *testing.T, chnker *chunker.Chunker, testChunks []chunk) []*chunker.Chunk {
	chunks := []*chunker.Chunk{}

	pos := uint64(0)
	for i, chunk := range testChunks {
		c, err := chnker.Next()

		if err != nil {
			t.Fatalf("Error returned with chunk %d: %v", i, err)
		}

		if c == nil {
			t.Fatalf("Nil chunk returned")
		}

		if c != nil {
			if c.Start != pos {
				t.Fatalf("Start for chunk %d does not match: expected %d, got %d",
					i, pos, c.Start)
			}

			if c.Length != chunk.Length {
				t.Fatalf("Length for chunk %d does not match: expected %d, got %d",
					i, chunk.Length, c.Length)
			}

			if c.Cut != chunk.CutFP {
				t.Fatalf("Cut fingerprint for chunk %d/%d does not match: expected %016x, got %016x",
					i, len(chunks)-1, chunk.CutFP, c.Cut)
			}

			if c.Digest != nil && !bytes.Equal(c.Digest, chunk.Digest) {
				t.Fatalf("Digest fingerprint for chunk %d/%d does not match: expected %02x, got %02x",
					i, len(chunks)-1, chunk.Digest, c.Digest)
			}

			pos += c.Length
			chunks = append(chunks, c)
		}
	}

	c, err := chnker.Next()

	if c != nil {
		t.Fatal("additional non-nil chunk returned")
	}

	if err != io.EOF {
		t.Fatal("wrong error returned after last chunk")
	}

	return chunks
}

func getRandom(seed, count int) []byte {
	buf := make([]byte, count)

	rnd := rand.New(rand.NewSource(23))
	for i := 0; i < count; i += 4 {
		r := rnd.Uint32()
		buf[i] = byte(r)
		buf[i+1] = byte(r >> 8)
		buf[i+2] = byte(r >> 16)
		buf[i+3] = byte(r >> 24)
	}

	return buf
}

func TestChunker(t *testing.T) {
	// setup data source
	buf := getRandom(23, 32*1024*1024)
	ch := chunker.New(bytes.NewReader(buf), testPol, sha256.New(), chunker.AvgSize, chunker.MinSize, chunker.MaxSize)
	chunks := testWithData(t, ch, chunks1)

	// test reader
	for i, c := range chunks {
		rd := c.Reader(bytes.NewReader(buf))

		h := sha256.New()
		n, err := io.Copy(h, rd)
		if err != nil {
			t.Fatalf("io.Copy(): %v", err)
		}

		if uint64(n) != chunks1[i].Length {
			t.Fatalf("reader returned wrong number of bytes: expected %d, got %d",
				chunks1[i].Length, n)
		}

		d := h.Sum(nil)
		if !bytes.Equal(d, chunks1[i].Digest) {
			t.Fatalf("wrong hash returned: expected %02x, got %02x",
				chunks1[i].Digest, d)
		}
	}

	// setup nullbyte data source
	buf = bytes.Repeat([]byte{0}, len(chunks2)*chunker.MinSize)
	ch = chunker.New(bytes.NewReader(buf), testPol, sha256.New(), chunker.AvgSize, chunker.MinSize, chunker.MaxSize)

	testWithData(t, ch, chunks2)
}

func TestChunkerWithRandomPolynomial(t *testing.T) {
	// setup data source
	buf := getRandom(23, 32*1024*1024)

	// generate a new random polynomial
	start := time.Now()
	p, err := chunker.RandomPolynomial()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("generating random polynomial took %v", time.Since(start))

	start = time.Now()
	ch := chunker.New(bytes.NewReader(buf), p, sha256.New(), chunker.AvgSize, chunker.MinSize, chunker.MaxSize)
	t.Logf("creating chunker took %v", time.Since(start))

	// make sure that first chunk is different
	c, err := ch.Next()

	if c.Cut == chunks1[0].CutFP {
		t.Fatal("Cut point is the same")
	}
	if c.Length == chunks1[0].Length {
		t.Fatal("Length is the same")
	}
	if bytes.Equal(c.Digest, chunks1[0].Digest) {
		t.Fatal("Digest is the same")
	}

}

func TestChunkerWithoutHash(t *testing.T) {
	// setup data source
	buf := getRandom(23, 32*1024*1024)

	ch := chunker.New(bytes.NewReader(buf), testPol, nil, chunker.AvgSize, chunker.MinSize, chunker.MaxSize)
	chunks := testWithData(t, ch, chunks1)

	// test reader
	for i, c := range chunks {
		rd := c.Reader(bytes.NewReader(buf))

		buf2, err := ioutil.ReadAll(rd)
		if err != nil {
			t.Fatalf("io.Copy(): %v", err)
		}

		if uint64(len(buf2)) != chunks1[i].Length {
			t.Fatalf("reader returned wrong number of bytes: expected %d, got %d",
				chunks1[i].Length, uint(len(buf2)))
		}

		if uint64(len(buf2)) != chunks1[i].Length {
			t.Fatalf("wrong number of bytes returned: expected %02x, got %02x",
				chunks[i].Length, len(buf2))
		}

		if !bytes.Equal(buf[c.Start:c.Start+c.Length], buf2) {
			t.Fatalf("invalid data for chunk returned: expected %02x, got %02x",
				buf[c.Start:c.Start+c.Length], buf2)
		}
	}

	// setup nullbyte data source
	buf = bytes.Repeat([]byte{0}, len(chunks2)*chunker.MinSize)
	ch = chunker.New(bytes.NewReader(buf), testPol, sha256.New(), chunker.AvgSize, chunker.MinSize, chunker.MaxSize)

	testWithData(t, ch, chunks2)
}

func benchmarkChunker(b *testing.B, hash hash.Hash) {
	size := 10 * 1024 * 1024
	rd := bytes.NewReader(getRandom(23, size))

	b.ResetTimer()
	b.SetBytes(int64(size))

	var chunks int
	for i := 0; i < b.N; i++ {
		chunks = 0

		rd.Seek(0, 0)
		ch := chunker.New(rd, testPol, hash, chunker.AvgSize, chunker.MinSize, chunker.MaxSize)

		for {
			_, err := ch.Next()

			if err == io.EOF {
				break
			}

			if err != nil {
				b.Fatalf("Unexpected error occurred: %v", err)
			}

			chunks++
		}
	}

	b.Logf("%d chunks, average chunk size: %d bytes", chunks, size/chunks)
}

func BenchmarkChunkerWithSHA256(b *testing.B) {
	benchmarkChunker(b, sha256.New())
}

func BenchmarkChunkerWithMD5(b *testing.B) {
	benchmarkChunker(b, md5.New())
}

func BenchmarkChunker(b *testing.B) {
	benchmarkChunker(b, nil)
}

func BenchmarkNewChunker(b *testing.B) {
	p, err := chunker.RandomPolynomial()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		chunker.New(bytes.NewBuffer(nil), p, nil, chunker.AvgSize, chunker.MinSize, chunker.MaxSize)
	}
}
