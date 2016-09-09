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

TEST_SWARM_PORT = randint(1024, 65535)
TEST_GATEWAY_PORT = randint(1024, 65535)

BOOTSTRAP_NODES = [
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 0) + "/ipfs/QmTujop5JvTHv99jG4WB739P6FdWYpA1Yxnv58zUhZ1nqX",
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 1) + "/ipfs/QmZHiLDFFCg7f1U65U9icaXvCfmxjZXbjmhai9TtNLPCgH",
    "/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + 2) + "/ipfs/QmfKbCVPt2cHgDuuUUyGkrYYTWge6q97eCiWtX8SYHRSCP"
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

    def setup_nodes(self):
        for i in range(self.num_nodes):
            self.configure_node(i)
            self.start_node(self.nodes[i])

    def setup_network(self):
        self.setup_nodes()

    def run_test(self):
        raise NotImplementedError

    def configure_node(self, n):
        dir_path = os.path.join(self.temp_dir, "openbazaar-go", str(n))
        args = [self.binary, "init", "-d", dir_path]
        if n < 3:
            args.extend(["-m", BOOTSTAP_MNEMONICS[n]])
        process = subprocess.Popen(args, stdout=PIPE)
        self.wait_for_init_success(process)
        with open(os.path.join(dir_path, "config")) as cfg:
            config = json.load(cfg)
        config["Addresses"]["Gateway"] = "/ip4/127.0.0.1/tcp/" + str(TEST_GATEWAY_PORT + n)
        config["Addresses"]["Swarm"] = ["/ip4/127.0.0.1/tcp/" + str(TEST_SWARM_PORT + n)]
        config["Bootstrap"] = BOOTSTRAP_NODES
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
        args = [self.binary, "start", "-d", node["data_dir"], "--disablewallet"]
        process = subprocess.Popen(args, stdout=PIPE)
        self.wait_for_start_success(process)

    @staticmethod
    def wait_for_start_success(process):
        while True:
            if process.poll() is not None:
                raise Exception("OpenBazaar node failed to start")
            output = process.stdout
            for o in output:
                if "Gateway/API server listening" in str(o):
                    return

    def teardown(self):
        shutil.rmtree(os.path.join(self.temp_dir, "openbazaar-go"))

    def main(self):
        parser = argparse.ArgumentParser(
                    description="OpenBazaar Test Framework",
                    usage="python3 test_framework.py [options]"
        )
        parser.add_argument('-b', '--binary', required=True, help="the openbazaar-go binary")
        parser.add_argument('-t', '--tempdir', action='store_true', help="temp directory to store the data folders", default="/tmp/")
        args = parser.parse_args(sys.argv[1:])
        self.binary = args.binary
        self.temp_dir = args.tempdir

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

