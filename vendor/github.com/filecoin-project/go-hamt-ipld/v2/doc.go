/*
Package hamt provides a reference implementation of the IPLD HAMT used in the
Filecoin blockchain. It includes some optional flexibility such that it may be
used for other purposes outside of Filecoin.

HAMT is a "hash array mapped trie"
https://en.wikipedia.org/wiki/Hash_array_mapped_trie. This implementation
extends the standard form by including buckets for the key/value pairs at
storage leaves and CHAMP mutation semantics
https://michael.steindorfer.name/publications/oopsla15.pdf. The CHAMP invariant
and mutation rules provide us with the ability to maintain canonical forms
given any set of keys and their values, regardless of insertion order and
intermediate data insertion and deletion. Therefore, for any given set of keys
and their values, a HAMT using the same parameters and CHAMP semantics, the
root node should always produce the same content identifier (CID).

Algorithm Overview

The HAMT algorithm hashes incoming keys and uses incrementing subsections of
that hash digest at each level of its tree structure to determine the placement
of either the entry or a link to a child node of the tree. A `bitWidth`
determines the number of bits of the hash to use for index calculation at each
level of the tree such that the root node takes the first `bitWidth` bits of
the hash to calculate an index and as we move lower in the tree, we move along
the hash by `depth x bitWidth` bits. In this way, a sufficiently randomizing
hash function will generate a hash that provides a new index at each level of
the data structure. An index comprising `bitWidth` bits will generate index
values of `[ 0, 2^bitWidth )`. So a `bitWidth` of 8 will generate indexes of 0
to 255 inclusive.

Each node in the tree can therefore hold up to `2^bitWidth` elements of data,
which we store in an array. In the this HAMT and the IPLD HashMap we store
entries in buckets. A `Set(key, value)` mutation where the index generated at
the root node for the hash of key denotes an array index that does not yet
contain an entry, we create a new bucket and insert the key / value pair entry.
In this way, a single node can theoretically hold up to
`2^bitWidth x bucketSize` entries, where `bucketSize` is the maximum number of
elements a bucket is allowed to contain ("collisions"). In practice, indexes do
not distribute with perfect randomness so this maximum is theoretical. Entries
stored in the node's buckets are stored in key-sorted order.

Parameters

This HAMT implementation:

• Fixes the `bucketSize` to 3.

• Defaults the `bitWidth` to 8, however within Filecoin it uses 5

• Defaults the hash algorithm to the 64-bit variant of Murmur3-x64

Further Reading

The algorithm used here is identical to that of the IPLD HashMap algorithm
specified at
https://github.com/ipld/specs/blob/master/data-structures/hashmap.md. The
specific parameters used by Filecoin and the DAG-CBOR block layout differ from
the specification and are defined at
https://github.com/ipld/specs/blob/master/data-structures/hashmap.md#Appendix-Filecoin-hamt-variant.
*/
package hamt
