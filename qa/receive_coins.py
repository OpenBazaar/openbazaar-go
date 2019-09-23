import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class ReceiveCoinsTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 1

    def run_test(self):
        time.sleep(4)
        api_url = self.nodes[0]["gateway_url"] + "wallet/address/" + self.cointype
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("ReceiveCoinsTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("ReceiveCoinsTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(20)
        api_url = self.nodes[0]["gateway_url"] + "wallet/balance/" + self.cointype
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed <= 0:
                raise TestFailure("ReceiveCoinsTest - FAIL: Wallet is empty")
        elif r.status_code == 404:
            raise TestFailure("ReceiveCoinsTest - FAIL: Receive coins endpoint not found")
        else:
            raise TestFailure("ReceiveCoinsTest - FAIL: Unknown response")

        print("ReceiveCoinsTest - PASS")


if __name__ == '__main__':
    print("Running ReceiveCoinsTest")
    ReceiveCoinsTest().main(["--regtest", "--disableexchangerates"])
