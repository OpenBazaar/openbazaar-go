import requests
import json
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class UploadListingTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 1

    def setup_network(self):
        self.setup_nodes()

    def run_test(self):
        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        if self.bitcoincash:
            listing_json["metadata"]["pricingCurrency"] = "tbch"
        api_url = self.nodes[0]["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("UploadListingTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("UploadListingTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        api_url = self.nodes[0]["gateway_url"] + "ob/inventory"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            inv = resp["ron-swanson-tshirt"]
            if inv == None:
                raise TestFailure("UploadListingTest - FAIL: Did not return inventory for listing")
            if inv["inventory"] != 213:
                raise TestFailure("UploadListingTest - FAIL: Returned incorrect amount of inventory: %d", inv["inventory"])
        elif r.status_code == 404:
            raise TestFailure("UploadListingTest - FAIL: Listing post endpoint not found")
        else:
            resp = json.loads(r.text)
            raise TestFailure("UploadListingTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        print("UploadListingTest - PASS")


if __name__ == '__main__':
    print("Running UploadListingTest")
    UploadListingTest().main()
