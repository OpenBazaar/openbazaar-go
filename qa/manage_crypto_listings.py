import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class ManageCryptoListingsTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 1

    def run_test(self):
        vendor = self.nodes[0]

        # post profile for vendor
        with open('testdata/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = vendor["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # check index
        r = requests.get(vendor["gateway_url"] + "ob/listings")
        resp = json.loads(r.text)
        if len(resp) != 0:
            raise TestFailure("ManageCryptoListingsTest - FAIL: Incorrect listing count: %d", len(resp))

        # post listing to vendor
        with open('testdata/listing_crypto.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)

        api_url = vendor["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ManageCryptoListingsTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        slug = json.loads(r.text)["slug"]

        # check index
        r = requests.get(vendor["gateway_url"] + "ob/listings")
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ManageCryptoListingsTest - FAIL: Incorrect listing count: %d", len(resp))
        for listing in resp:
            if listing['contractType'] == 'CRYPTOCURRENCY':
                if listing["coinType"] != "ETH":
                    raise TestFailure("ManageCryptoListingsTest - FAIL: coinType incorrect: %s", listing["coinType"])

        # delete listing
        api_url = vendor["gateway_url"] + "ob/listing/"+slug
        r = requests.delete(api_url)
        if r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ManageCryptoListingsTest - FAIL: Listing DELETE failed. Reason: %s", resp["reason"])

        # check index
        r = requests.get(vendor["gateway_url"] + "ob/listings")
        resp = json.loads(r.text)
        if len(resp) != 0:
            raise TestFailure("ManageCryptoListingsTest - FAIL: Incorrect listing count: %d", len(resp))

        print("ManageCryptoListingsTest - PASS")

if __name__ == '__main__':
    print("Running ManageCryptoListingsTest")
    ManageCryptoListingsTest().main(["--regtest", "--disableexchangerates"])
