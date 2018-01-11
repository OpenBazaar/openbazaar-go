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
import requests
from random import randint
from subprocess import PIPE
from bitcoin import rpc
from bitcoin import SelectParams
from shutil import copyfile

TEST_SWARM_PORT = randint(1024, 65535)
TEST_GATEWAY_PORT = randint(1024, 65535)

BOOTSTRAP_NODES = [
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 0) + "/ipfs/Qmdo6RpKtSqk73gUwaiaPkq6gWk49y3NCPCQbVsM9XTma3",
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 1) + "/ipfs/QmVQzkdi3Fq6LRFG9UNqDZfSry67weCZV6ZL26QVx64UFy",
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 2) + "/ipfs/Qmd5qDpcYkHCmkj9pMXU9TKBqEDWgEmtoHD5xjdJgumaHg"
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

    def send_bitcoin_cmd(self, *args):
        try:
            return self.bitcoin_api.call(*args)
        except BrokenPipeError:
            self.bitcoin_api = rpc.Proxy(btc_conf_file=self.btc_config)
            return self.send_bitcoin_cmd(*args)

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
        to_boostrap = []
        for node in BOOTSTRAP_NODES:
            if config["Addresses"]["Swarm"][0] not in node:
                to_boostrap.append(node)
        config["Bootstrap-testnet"] = to_boostrap
        config["Wallet"]["TrustedPeer"] = "127.0.0.1:18444"
        config["Wallet"]["FeeAPI"] = ""
        config["Crosspost-gateways"] = []
        config["Swarm"]["DisableNatPortMap"] = True

        if self.bitcoincash:
            config["Wallet"]["Type"] = "bitcoincash"

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
        args = [self.binary, "start", "-v", "-d", node["data_dir"], *self.options]
        process = subprocess.Popen(args, stdout=PIPE)
        peerId = self.wait_for_start_success(process, node)
        node["peerId"] = peerId

    @staticmethod
    def wait_for_start_success(process, node):
        peerId = ""
        while True:
            if process.poll() is not None:
                raise Exception("OpenBazaar node failed to start")
            output = process.stdout
            for o in output:
                if "Peer ID:" in str(o):
                    peerId = str(o)[str(o).index("Peer ID:") + 10:len(str(o)) - 3]
                if "Gateway/API server listening" in str(o):
                    return peerId

    def start_bitcoind(self):
        SelectParams('regtest')
        dir_path = os.path.join(self.temp_dir, "openbazaar-go", "bitcoin")
        if not os.path.exists(dir_path):
            os.makedirs(dir_path)
        btc_conf_file = os.path.join(dir_path, "bitcoin.conf")
        copyfile(os.path.join(os.getcwd(), "testdata", "bitcoin.conf"), btc_conf_file)
        self.btc_config = btc_conf_file
        args = [self.bitcoind, "-regtest", "-datadir=" + dir_path, "-debug=net"]
        process = subprocess.Popen(args, stdout=PIPE)
        self.wait_for_bitcoind_start(process, btc_conf_file)
        self.init_blockchain()

    def init_blockchain(self):
        self.send_bitcoin_cmd("generate", 1)
        self.bitcoin_address = self.send_bitcoin_cmd("getnewaddress")
        self.send_bitcoin_cmd("generatetoaddress", 1, self.bitcoin_address)
        self.send_bitcoin_cmd("generate", 435)

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

    def print_logs(self, node, log):
        f = open(os.path.join(node["data_dir"], "logs", log), 'r')
        file_contents = f.read()
        print()
        print("~~~~~~~~~~~~~~~~~~~~~~ " + log + " ~~~~~~~~~~~~~~~~~~~~~~")
        print (file_contents)
        print()
        f.close()

    def teardown(self):
        for n in self.nodes:
            requests.post(n["gateway_url"] + "ob/shutdown")
        time.sleep(2)
        if self.bitcoin_api is not None:
            try:
                self.send_bitcoin_cmd("stop")
            except BrokenPipeError:
                pass
        time.sleep(10)

    def main(self, options=["--disablewallet", "--testnet", "--disableexchangerates"]):
        parser = argparse.ArgumentParser(
                    description="OpenBazaar Test Framework",
                    usage="python3 test_framework.py [options]"
        )
        parser.add_argument('-b', '--binary', required=True, help="the openbazaar-go binary")
        parser.add_argument('-d', '--bitcoind', help="the bitcoind binary")
        parser.add_argument('-t', '--tempdir', action='store_true', help="temp directory to store the data folders", default="/tmp/")
        parser.add_argument('-c', '--bitcoincash', help="test with bitcoin cash", action='store_true', default=False)
        args = parser.parse_args(sys.argv[1:])
        self.binary = args.binary
        self.temp_dir = args.tempdir
        self.bitcoind = args.bitcoind
        self.bitcoincash = args.bitcoincash
        self.options = options

        try:
            shutil.rmtree(os.path.join(self.temp_dir, "openbazaar-go"))
        except:
            pass

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
