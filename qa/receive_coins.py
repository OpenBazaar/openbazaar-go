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
        api_url = self.nodes[0]["gateway_url"] + "wallet/address"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("ReceiveCoinsTest - FAIL: Listing post endpoint not found")
        else:
            raise TestFailure("ReceiveCoinsTest - FAIL: Unknown response")
        r = self.bitcoin_api.call("generatetoaddress", 1, address)
        time.sleep(1)
        api_url = self.nodes[0]["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed > 0:
                print("ReceiveCoinsTest - PASS")
            else:
                raise TestFailure("ReceiveCoinsTest - FAIL: Wallet is empty")
        elif r.status_code == 404:
            raise TestFailure("ReceiveCoinsTest - FAIL: Listing post endpoint not found")
        else:
            raise TestFailure("ReceiveCoinsTest - FAIL: Unknown response")

if __name__ == '__main__':
    print("Running ReceiveCoinsTest")
    ReceiveCoinsTest().main()
