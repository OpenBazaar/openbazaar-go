const assert = require('assert')

const mplex = require('libp2p-mplex')
const toPull = require('stream-to-pull-stream')
const pull = require('pull-stream')
const tcp = require('tcp')

const jsTestData = 'test data from js'
const goTestData = 'test data from go'

function readWrite (stream) {
  pull(
    stream,
    pull.concat((err, data) => {
      if (err) {
        throw err
      }
      let offset = 0
      for (let i = 0; i < 100; i++) {
        let expected = goTestData + ' ' + i
        assert.equal(expected, data.slice(offset, offset + expected.length))
        offset += expected.length
      }
    })
  )
  pull(
    pull.count(99),
    pull.map((i) => jsTestData + ' ' + i),
    stream
  )
}

const listener = tcp.createServer((socket) => {
  let muxer = mplex.listener(toPull.duplex(socket))
  muxer.on('stream', (stream) => {
    readWrite(stream)
  })
  for (let i = 0; i < 100; i++) {
    muxer.newStream((err, stream) => {
      if (err) {
        throw err
      }
      readWrite(stream)
    })
  }
  socket.on('close', () => {
    listener.close()
  })
})

listener.listen(9991)
