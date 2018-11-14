#!/usr/bin/env bash

test_description="Test dht command"

. lib/test-lib.sh

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
  iptb init -n $NUM_NODES --bootstrap=none --port=0
'

run_pubsub_tests() {
  test_expect_success 'peer ids' '
    PEERID_0=$(iptb get id 0) &&
    PEERID_2=$(iptb get id 2)
  '
  
  # ipfs pubsub sub
  test_expect_success 'pubsub' '
    echo "testOK" > expected &&
    touch empty &&
    mkfifo wait ||
    test_fsh echo init fail
  
    # ipfs pubsub sub is long-running so we need to start it in the background and
    # wait put its output somewhere where we can access it
    (
      ipfsi 0 pubsub sub --enc=ndpayload testTopic | if read line; then
          echo $line > actual &&
          echo > wait
        fi
    ) &
  '
  
  test_expect_success "wait until ipfs pubsub sub is ready to do work" '
    go-sleep 500ms
  '
  
  test_expect_success "can see peer subscribed to testTopic" '
    ipfsi 1 pubsub peers testTopic > peers_out
  '
  
  test_expect_success "output looks good" '
    echo $PEERID_0 > peers_exp &&
    test_cmp peers_exp peers_out
  '
  
  test_expect_success "publish something" '
    ipfsi 1 pubsub pub testTopic "testOK" &> pubErr
  '
  
  test_expect_success "wait until echo > wait executed" '
    cat wait &&
    test_cmp pubErr empty &&
    test_cmp expected actual
  '
  
  test_expect_success "wait for another pubsub message" '
    echo "testOK2" > expected &&
    mkfifo wait2 ||
    test_fsh echo init fail
  
    # ipfs pubsub sub is long-running so we need to start it in the background and
    # wait put its output somewhere where we can access it
    (
      ipfsi 2 pubsub sub --enc=ndpayload testTopic | if read line; then
          echo $line > actual &&
          echo > wait2
        fi
    ) &
  '
  
  test_expect_success "wait until ipfs pubsub sub is ready to do work" '
    go-sleep 500ms
  '
  
  test_expect_success "publish something" '
    echo "testOK2" | ipfsi 3 pubsub pub testTopic &> pubErr
  '
  
  test_expect_success "wait until echo > wait executed" '
    echo "testOK2" > expected &&
    cat wait2 &&
    test_cmp pubErr empty &&
    test_cmp expected actual
  '

  test_expect_success 'cleanup fifos' '
    rm -f wait wait2
  '
  
}

# Normal tests

startup_cluster $NUM_NODES --enable-pubsub-experiment
run_pubsub_tests
test_expect_success 'stop iptb' '
  iptb stop
'

# Test with some nodes not signing messages.

test_expect_success 'disable signing on node 1' '
  ipfsi 1 config --json Pubsub.DisableSigning true
'

startup_cluster $NUM_NODES --enable-pubsub-experiment
run_pubsub_tests
test_expect_success 'stop iptb' '
  iptb stop
'

# Test strict message verification.

test_expect_success 'enable strict signature verification on node 4' '
  ipfsi 4 config --json Pubsub.StrictSignatureVerification true
'

startup_cluster $NUM_NODES --enable-pubsub-experiment

test_expect_success 'set node 4 to listen on testTopic' '
  ipfsi 4 pubsub sub --enc=ndpayload testTopic > node4_actual &
'

run_pubsub_tests

test_expect_success 'stop iptb' '
  iptb stop
'

test_expect_success 'node 4 only got the signed message' '
  echo "testOK2" > node4_expected &&
  test_cmp node4_actual node4_expected
'

# Test all nodes signing with strict verification

test_expect_success 're-enable signing on node 1' '
  ipfsi 1 config --json Pubsub.DisableSigning false
'

test_expect_success 'enable strict signature verification on all nodes' '
  iptb for-each ipfs config --json Pubsub.StrictSignatureVerification true
'

startup_cluster $NUM_NODES --enable-pubsub-experiment
run_pubsub_tests
test_expect_success 'stop iptb' '
  iptb stop
'

test_done
