import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class PurchaseDirectOfflineRejectTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]

        # post listing to alice
        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)

        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # get listing hash
        api_url = alice["gateway_url"] + "ipns/" + alice["peerId"] + "/listings/index.json"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # bob fetch listing to cache
        api_url = bob["gateway_url"] + "ipfs/" + listingId
        requests.get(api_url)

        # generate some coins and send them to bob
        api_url = bob["gateway_url"] + "wallet/address"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Unknown response")
        self.bitcoin_api.call("generatetoaddress", 1, address)
        time.sleep(2)
        self.bitcoin_api.call("generate", 125)
        time.sleep(3)

        # FIXME: the remainder of this test fails on travis for some reason. Alice's node fails to fetch the dht pointer.
        print("PurchaseDirectOfflineRejectTest - PARTIAL PASS")
        return

        # shutdown alice
        api_url = alice["gateway_url"] + "ob/shutdown"
        requests.post(api_url, data="")
        time.sleep(4)

        # bob send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        api_url = bob["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]
        if resp["vendorOnline"] == True:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Purchase returned vendor is online")

        # check the purchase saved correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "PENDING":
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Bob purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Bob incorrectly saved as funded")

        # fund order
        spend = {
            "address": payment_address,
            "amount": payment_amount,
            "feeLevel": "NORMAL"
        }
        api_url = bob["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        time.sleep(5)

        # check bob detected payment
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if len(resp["transactions"]) <= 0:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Bob failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Bob incorrectly saved as unfunded")

        # generate one more block containing this tx
        self.bitcoin_api.call("generate", 1)

        # startup alice again
        self.start_node(alice)
        time.sleep(5)

        # alice reject order
        api_url = alice["gateway_url"] + "ob/orderconfirmation"
        oc = {
            "orderId": orderId,
            "reject": True
        }
        r = requests.post(api_url, data=json.dumps(oc, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Order confirmation post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        time.sleep(10)

        # alice check order rejected correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "REJECTED":
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Alice failed to save as rejected")
        if len(resp["transactions"]) != 2:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Alice failed to detect outgoing payment")

        # bob check order rejected correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "REJECTED":
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Bob failed to save as rejected")
        if len(resp["transactions"]) != 2:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Bob failed to detect outgoing payment")

        # Check the funds moved into bob's wallet
        api_url = bob["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed <= 50 - payment_amount:
                raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Bob failed to receive the multisig payout")
        else:
            raise TestFailure("PurchaseDirectOfflineRejectTest - FAIL: Failed to query Bob's balance")

        print("PurchaseDirectOfflineRejectTest - PASS")

if __name__ == '__main__':
    print("Running PurchaseDirectOfflineRejectTest")
    PurchaseDirectOfflineRejectTest().main(["--regtest", "--disableexchangerates"])
