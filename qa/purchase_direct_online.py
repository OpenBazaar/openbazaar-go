import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class PurchaseDirectOnlineTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 2

    def setup_network(self):
        self.setup_nodes()

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]

        # post listing to alice
        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)

        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # get listing hash
        api_url = alice["gateway_url"] + "ipns/" + alice["peerId"] + "/listings/index.json"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # bob send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        api_url = bob["gateway_url"] + "/ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 200:
            print("PurchaseDirectOnlineTest - PASS")
        elif r.status_code == 404:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Purchase post endpoint not found")
        else:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])

if __name__ == '__main__':
    print("Running PurchaseDirectOnlineTest")
    PurchaseDirectOnlineTest().main()
