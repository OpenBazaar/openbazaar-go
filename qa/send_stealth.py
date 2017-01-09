import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class SendStealthTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]
        charlie = self.nodes[1]
        time.sleep(4)

        # create a profile for bob
        pro = {"name": "Bob"}
        api_url = bob["gateway_url"] + "ob/profile"
        r = requests.post(api_url, data=json.dumps(pro, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseSplitTest - FAIL: Profile post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("DisputeCloseSplitTest - FAIL: Profile POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # send coins to alice
        api_url = alice["gateway_url"] + "wallet/address"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("SendStealthTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(20)

        # send coins to alice
        api_url = charlie["gateway_url"] + "wallet/address"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            charlie_address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("SendStealthTest - FAIL: Unknown response")

        # check alice balance
        api_url = alice["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed <= 0:
                raise TestFailure("SendStealthTest - FAIL: Wallet is empty")
        elif r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Receive coins endpoint not found")
        else:
            raise TestFailure("SendStealthTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("generate", 1)
        time.sleep(3)

        # stealth send to bob
        payment_amount = 1000000
        spend = {
            "peerId": bob["peerId"],
            "amount": payment_amount,
            "feeLevel": "NORMAL"
        }
        api_url = alice["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SendStealthTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check alice balance
        api_url = alice["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed >= 1000000000 - payment_amount:
                raise TestFailure("SendStealthTest - FAIL: Alice failed to record spend")
        elif r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Receive coins endpoint not found")
        else:
            raise TestFailure("SendStealthTest - FAIL: Unknown response")

        # check bob balance
        api_url = bob["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed <= 0:
                raise TestFailure("SendStealthTest - FAIL: Bob failed to detect payment")
        elif r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Receive coins endpoint not found")
        else:
            raise TestFailure("SendStealthTest - FAIL: Unknown response")

        # stealth send to charlie
        charlie_payment_amount = 5000
        spend = {
            "address": charlie_address,
            "amount": charlie_payment_amount,
            "feeLevel": "NORMAL"
        }
        api_url = bob["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SendStealthTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check bob balance
        api_url = bob["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed >= payment_amount - charlie_payment_amount:
                raise TestFailure("SendStealthTest - FAIL: Bob failed to record spend")
        elif r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Receive coins endpoint not found")
        else:
            raise TestFailure("SendStealthTest - FAIL: Unknown response")

        # check charlie balance
        api_url = charlie["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed <= 0:
                raise TestFailure("SendStealthTest - FAIL: Charlie failed to detect payment")
        elif r.status_code == 404:
            raise TestFailure("SendStealthTest - FAIL: Receive coins endpoint not found")
        else:
            raise TestFailure("SendStealthTest - FAIL: Unknown response")

        print("SendStealthTest - PASS")

if __name__ == '__main__':
    print("Running SendStealthTest")
    SendStealthTest().main(["--regtest", "--disableexchangerates"])
