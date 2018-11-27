package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"

	mplex "gx/ipfs/QmP5tygNbeAH5Z8sGdrnE9rqqkAVYotam3UAgRWWfL2zur/go-mplex"
)

var jsTestData = "test data from js %d"
var goTestData = "test data from go %d"

func main() {
	conn, err := net.Dial("tcp4", "127.0.0.1:9991")
	if err != nil {
		panic(err)
	}
	sess := mplex.NewMultiplex(conn, true)
	defer sess.Close()

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			s, err := sess.NewStream()
			if err != nil {
				panic(err)
			}
			readWrite(s)
		}()
	}
	for i := 0; i < 100; i++ {
		s, err := sess.Accept()
		if err != nil {
			panic(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			readWrite(s)
		}()
	}
	wg.Wait()
}

func readWrite(s *mplex.Stream) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, err := fmt.Fprintf(s, goTestData, i)
			if err != nil {
				panic(err)
			}
		}
		s.Close()
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			expected := fmt.Sprintf(jsTestData, i)
			actual := make([]byte, len(expected))
			_, err := io.ReadFull(s, actual)
			if err != nil {
				panic(err)
			}
			if expected != string(actual) {
				panic("bad bytes")
			}
		}
		buf, err := ioutil.ReadAll(s)
		if err != nil {
			panic(err)
		}
		if len(buf) > 0 {
			panic("expected EOF")
		}
	}()
	wg.Wait()
}
