package chunk

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	util "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
)

func TestRabinChunking(t *testing.T) {
	data := make([]byte, 1024*1024*16)
	util.NewTimeSeededRand().Read(data)

	r := NewRabin(bytes.NewReader(data), 1024*256)

	var chunks [][]byte

	for {
		chunk, err := r.NextBytes()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}

		chunks = append(chunks, chunk)
	}

	fmt.Printf("average block size: %d\n", len(data)/len(chunks))

	unchunked := bytes.Join(chunks, nil)
	if !bytes.Equal(unchunked, data) {
		fmt.Printf("%d %d\n", len(unchunked), len(data))
		t.Fatal("data was chunked incorrectly")
	}
}

func chunkData(t *testing.T, data []byte) map[string]blocks.Block {
	r := NewRabin(bytes.NewReader(data), 1024*256)

	blkmap := make(map[string]blocks.Block)

	for {
		blk, err := r.NextBytes()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}

		b := blocks.NewBlock(blk)
		blkmap[b.Cid().KeyString()] = b
	}

	return blkmap
}

func TestRabinChunkReuse(t *testing.T) {
	data := make([]byte, 1024*1024*16)
	util.NewTimeSeededRand().Read(data)

	ch1 := chunkData(t, data[1000:])
	ch2 := chunkData(t, data)

	var extra int
	for k := range ch2 {
		_, ok := ch1[k]
		if !ok {
			extra++
		}
	}

	if extra > 2 {
		t.Log("too many spare chunks made")
	}
}
