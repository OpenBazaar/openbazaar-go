import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class PurchaseOfflineErrorTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]

        # post profile for alice
        with open('testdata/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = alice["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listing to alice
        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        if self.bitcoincash:
            listing_json["metadata"]["pricingCurrency"] = "tbch"

        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # get listing hash
        api_url = alice["gateway_url"] + "ipns/" + alice["peerId"] + "/listings.json"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Couldn't get listing index")
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
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(3)

        # shutdown alice
        api_url = alice["gateway_url"] + "ob/shutdown"
        requests.post(api_url, data="")
        time.sleep(4)

        # bob send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId

        # set empty shipping address to trigger error
        order_json["address"] = ""

        api_url = bob["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]
        if resp["vendorOnline"] == True:
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Purchase returned vendor is online")

        # check the purchase saved correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Bob purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Bob incorrectly saved as funded")

        # startup alice again
        self.start_node(alice)
        time.sleep(45)

        # check alice detected processing error
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "PROCESSING_ERROR":
            raise TestFailure("PurchaseOfflineErrorTest - FAIL: Alice failed to detect processing error")

        # check bob detected error message from alice
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "PROCESSING_ERROR":
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Bob failed to set state correctly")

        print("PurchaseOfflineErrorTest - PASS")


if __name__ == '__main__':
    print("Running PurchaseOfflineErrorTest")
    PurchaseOfflineErrorTest().main(["--regtest", "--disableexchangerates"])
