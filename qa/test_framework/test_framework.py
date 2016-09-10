#!/usr/bin/env python3
# coding: utf-8

import os
import sys
import subprocess
import shutil
import time
import json
import argparse
import traceback
from random import randint
from subprocess import PIPE
from bitcoin import rpc
from bitcoin import SelectParams
from shutil import copyfile

TEST_SWARM_PORT = randint(1024, 65535)
TEST_GATEWAY_PORT = randint(1024, 65535)

BOOTSTRAP_NODES = [
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 0) + "/ipfs/QmVp4tK486CvnamB6K4uhY4vB5sMzEDpCNzeyh9VwBFXhS",
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 1) + "/ipfs/QmWPBKm3sLEPMy8EqrR5nHD2KUzh3TEgfquRjuxHM4h3Pv",
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 2) + "/ipfs/QmPL1X7ZQr2ooQHfuWJwuJndeyaN2DBoYHFg954TUsytvF"
]

BOOTSTAP_MNEMONICS = [
    "today summer matter always angry crumble rib lucky park shoulder police puppy",
    "husband letter control display skin tennis this expand garbage boil pig exchange",
    "resist museum dizzy there pulp suspect dust useless drama grab visa trumpet"
]


class TestFailure(Exception):
    pass


class OpenBazaarTestFramework(object):

    def __init__(self):
        self.nodes = []
        self.bitcoin_api = None

    def setup_nodes(self):
        for i in range(self.num_nodes):
            self.configure_node(i)
            self.start_node(self.nodes[i])

    def setup_network(self):
        if self.bitcoind is not None:
            self.start_bitcoind()
        self.setup_nodes()

    def run_test(self):
        raise NotImplementedError

    def configure_node(self, n):
        dir_path = os.path.join(self.temp_dir, "openbazaar-go", str(n))
        args = [self.binary, "init", "-d", dir_path, "--testnet"]
        if n < 3:
            args.extend(["-m", BOOTSTAP_MNEMONICS[n]])
        process = subprocess.Popen(args, stdout=PIPE)
        self.wait_for_init_success(process)
        with open(os.path.join(dir_path, "config")) as cfg:
            config = json.load(cfg)
        config["Addresses"]["Gateway"] = "/ip4/127.0.0.1/tcp/" + str(TEST_GATEWAY_PORT + n)
        config["Addresses"]["Swarm"] = ["/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + n)]
        config["Bootstrap"] = BOOTSTRAP_NODES
        config["Wallet"]["TrustedPeer"] = "127.0.0.1:18444"
        with open(os.path.join(dir_path, "config"), 'w') as outfile:
            outfile.write(json.dumps(config, indent=4))
        node = {
            "data_dir": dir_path,
            "gateway_url": "http://localhost:" + str(TEST_GATEWAY_PORT + n) + "/",
            "swarm_port": str(TEST_SWARM_PORT + n)
        }
        self.nodes.append(node)

    @staticmethod
    def wait_for_init_success(process):
        while True:
            if process.poll() is not None:
                raise Exception("OpenBazaar node initialization failed")
            output = process.stdout
            for o in output:
                if "OpenBazaar repo initialized" in str(o):
                    return

    def start_node(self, node):
        args = [self.binary, "start", "-d", node["data_dir"], "--regtest"]
        process = subprocess.Popen(args, stdout=PIPE)
        peerId = self.wait_for_start_success(process)
        node["peerId"] = peerId

    @staticmethod
    def wait_for_start_success(process):
        peerId = ""
        while True:
            if process.poll() is not None:
                raise Exception("OpenBazaar node failed to start")
            output = process.stdout
            for o in output:
                if "Peer ID:" in str(o):
                    peerId = str(o)[str(o).index("Peer ID:") + 10:len(str(o)) - 3]
                if "Starting bitcoin wallet..." in str(o):
                    return peerId

    def start_bitcoind(self):
        SelectParams('regtest')
        dir_path = os.path.join(self.temp_dir, "openbazaar-go", "bitcoin")
        if not os.path.exists(dir_path):
            os.makedirs(dir_path)
        btc_conf_file = os.path.join(dir_path, "bitcoin.conf")
        copyfile(os.path.join(os.getcwd(), "testdata", "bitcoin.conf"), btc_conf_file)
        args = [self.bitcoind, "-regtest", "-datadir=" + dir_path]
        process = subprocess.Popen(args, stdout=PIPE)
        self.wait_for_bitcoind_start(process, btc_conf_file)
        self.bitcoin_api.call("generate", 1)

    def wait_for_bitcoind_start(self, process, btc_conf_file):
        while True:
            if process.poll() is not None:
                raise Exception('bitcoind exited with status %i during initialization' % process.returncode)
            try:
                self.bitcoin_api = rpc.Proxy(btc_conf_file=btc_conf_file)
                blocks = self.bitcoin_api.getblockcount()
                break # break out of loop on success
            except Exception:
                time.sleep(0.25)
                continue

    def teardown(self):
        shutil.rmtree(os.path.join(self.temp_dir, "openbazaar-go"))
        if self.bitcoin_api is not None:
            self.bitcoin_api.call("stop")
        time.sleep(2)

    def main(self):
        parser = argparse.ArgumentParser(
                    description="OpenBazaar Test Framework",
                    usage="python3 test_framework.py [options]"
        )
        parser.add_argument('-b', '--binary', required=True, help="the openbazaar-go binary")
        parser.add_argument('-d', '--bitcoind', help="the bitcoind binary")
        parser.add_argument('-t', '--tempdir', action='store_true', help="temp directory to store the data folders", default="/tmp/")
        args = parser.parse_args(sys.argv[1:])
        self.binary = args.binary
        self.temp_dir = args.tempdir
        self.bitcoind = args.bitcoind

        failure = False
        try:
            self.setup_network()
            self.run_test()
        except TestFailure as e:
            print(repr(e))
            failure = True
        except Exception as e:
            print("Unexpected exception caught during testing: " + repr(e))
            traceback.print_tb(sys.exc_info()[2])
            failure = True

        self.teardown()
        if failure:
            sys.exit(1)

