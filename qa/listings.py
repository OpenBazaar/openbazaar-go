import requests
import json
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class ListingsTest(OpenBazaarTestFramework):
    def __init__(self):
        super().__init__()
        self.num_nodes = 1

    def setup_network(self):
        self.setup_nodes()

    def run_test(self):
        node = self.nodes[0]

        # no listings POSTed
        api_url = node["gateway_url"] + "ob/listings"
        r = requests.get(api_url)
        if r.status_code == 200:
            if len(json.loads(r.text)) == 0:
                pass
            else:
                raise TestFailure("ListingsTest - FAIL: No listings should be returned")
        elif r.status_code == 404:
            raise TestFailure("ListingsTest - FAIL: Listings get endpoint not found")
        else:
            resp = json.loads(r.text)
            raise TestFailure("ListingsTest - FAIL: Listings GET failed. Reason: %s", resp["reason"])

        # POST listing
        with open('testdata/listing.json') as listing_file:
            ljson = json.load(listing_file, object_pairs_hook=OrderedDict)

        ljson["metadata"]["pricingCurrency"] = self.currency
        api_url = node["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(ljson, indent=4))
        if r.status_code == 200:
            pass
        elif r.status_code == 404:
            raise TestFailure("ListingsTest - FAIL: Listing post endpoint not found")
        else:
            resp = json.loads(r.text)
            raise TestFailure("ListingsTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])

        # one listing POSTed
        api_url = node["gateway_url"] + "ob/listings"
        r = requests.get(api_url)
        if r.status_code == 200:
            if len(json.loads(r.text)) == 1:
                print("ListingsTest - PASS")
            else:
                raise TestFailure("ListingsTest - FAIL: One listing should be returned")
        elif r.status_code == 404:
            raise TestFailure("ListingsTest - FAIL: Listings get endpoint not found")
        else:
            resp = json.loads(r.text)
            raise TestFailure("ListingsTest - FAIL: Listings GET failed. Reason: %s", resp["reason"])


if __name__ == '__main__':
    print("Running ListingTest")
    ListingsTest().main()
