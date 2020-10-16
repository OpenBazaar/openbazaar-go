go-hamt-ipld
==================

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](https://protocol.ai/)
[![Travis CI](https://travis-ci.org/filecoin-project/go-hamt-ipld.svg?branch=master)](https://travis-ci.org/filecoin-project/go-hamt-ipld)

**This package is a reference implementation of the IPLD HAMT used in the
Filecoin blockchain.** It includes some optional flexibility such that it may
be used for other purposes outside of Filecoin.

HAMT is a ["hash array mapped trie"](https://en.wikipedia.org/wiki/Hash_array_mapped_trie).
This implementation extends the standard form by including buckets for the
key/value pairs at storage leaves and [CHAMP mutation semantics](https://michael.steindorfer.name/publications/oopsla15.pdf).
The CHAMP invariant and mutation rules provide us with the ability to maintain
canonical forms given any set of keys and their values, regardless of insertion
order and intermediate data insertion and deletion. Therefore, for any given
set of keys and their values, a HAMT using the same parameters and CHAMP
semantics, the root node should always produce the same content identifier
(CID).

**See https://godoc.org/github.com/filecoin-project/go-hamt-ipld for more information and
API details.**

## License

MIT © Whyrusleeping
