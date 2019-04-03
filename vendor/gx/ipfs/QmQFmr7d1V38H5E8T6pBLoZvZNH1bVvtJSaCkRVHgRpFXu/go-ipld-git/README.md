Git ipld format
==================

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![](https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square)](http://ipfs.io/)
[![](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)
[![Coverage Status](https://codecov.io/gh/ipfs/go-ipld-git/branch/master/graph/badge.svg)](https://codecov.io/gh/ipfs/go-ipld-git/branch/master)
[![Travis CI](https://travis-ci.org/ipfs/go-ipld-git.svg?branch=master)](https://travis-ci.org/ipfs/go-ipld-git)

> An ipld codec for git objects allowing path traversals across the git graph!

Note: This is WIP and may not be an entirely correct parser.

## Table of Contents

- [Install](#install)
- [About](#about)
- [Contribute](#contribute)
- [License](#license)

## Install

```sh
go get github.com/ipfs/go-ipld-git
```

## About
This is an IPLD codec which handles git objects. Objects are transformed
into IPLD graph in the following way:

* Commit:
```json
{
  "author": {
    "date": "1503667703 +0200",
    "email": "author@mail",
    "name": "Author Name"
  },
  "committer": {
    "date": "1503667703 +0200",
    "email": "author@mail",
    "name": "Author Name"
  },
  "message": "Commit Message\n",
  "parents": [
    <LINK>, <LINK>, ...
  ],
  "tree": <LINK>
}

```

* Tag:
```json
{
  "message": "message\n",
  "object": {
    "/": "z8mWaJGuvHyZf5uwV8TSYbvSbXP6xS9gR"
  },
  "tag": "tagname",
  "tagger": {
    "date": "1503667703 +0200",
    "email": "author@mail",
    "name": "Author Name"
  },
  "type": "commit"
}

```

* Tree:
```json
{
  "file.name": {
    "mode": "100664",
    "hash": <LINK>
  },
  "directoryname": {
    "mode": "40000",
    "hash": <LINK>
  },
  ...
}
```


* Blob:
```json
"<base64 of 'blob <size>\0<data>'>"
```
## Contribute

PRs are welcome!

Small note: If editing the Readme, please conform to the [standard-readme](https://github.com/RichardLitt/standard-readme) specification.

## License

MIT © Jeromy Johnson
