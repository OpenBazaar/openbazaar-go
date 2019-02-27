import requests
import json
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class ListingsTest(OpenBazaarTestFramework):
    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def setup_network(self):
        self.setup_nodes()

    def run_test(self):
        vendor = self.nodes[1]
        browser = self.nodes[2]

        currency = "tbtc"

        # no listings POSTed
        api_url = vendor["gateway_url"] + "ob/listings"
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
        ljson["metadata"]["pricingCurrency"] = "T" + self.cointype
        currency = "T" + self.cointype
        api_url = vendor["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(ljson, indent=4))
        if r.status_code == 200:
            pass
        elif r.status_code == 404:
            raise TestFailure("ListingsTest - FAIL: Listing post endpoint not found")
        else:
            resp = json.loads(r.text)
            raise TestFailure("ListingsTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])

        # one listing POSTed and index returning correct data
        api_url = vendor["gateway_url"] + "ob/listings"
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ListingsTest - FAIL: Listings get endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ListingsTest - FAIL: Listings GET failed. Reason: %s", resp["reason"])

        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ListingsTest - FAIL: One listing should be returned")

        listing = resp[0]
        if currency.lower() not in listing["acceptedCurrencies"]:
            raise TestFailure("ListingsTest - FAIL: Listing should have acceptedCurrencies")

        # listing show endpoint returning correct data
        slug = listing["slug"]
        api_url = vendor["gateway_url"] + "ob/listing/" + slug
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ListingsTest - FAIL: Listings get endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ListingsTest - FAIL: Listings GET failed. Reason: %s", resp["reason"])

        resp = json.loads(r.text)
        if currency.lower() not in resp["listing"]["metadata"]["acceptedCurrencies"]:
            raise TestFailure("ListingsTest - FAIL: Listing should have acceptedCurrences in metadata")

        # check vendor's index from another node
        api_url = browser["gateway_url"] + "ob/listings/" + vendor["peerId"]
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ListingsTest - FAIL: Listings get endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ListingsTest - FAIL: Listings GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ListingsTest - FAIL: One listing should be returned")
        if currency.lower() not in resp[0]["acceptedCurrencies"]:
            raise TestFailure("ListingsTest - FAIL: Listing should have acceptedCurrences")

        # check listing show page from another node
        api_url = vendor["gateway_url"] + "ob/listing/" + vendor["peerId"] + "/" + slug
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ListingsTest - FAIL: Listings get endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ListingsTest - FAIL: Listings GET failed. Reason: %s", resp["reason"])

        resp = json.loads(r.text)
        if currency.lower() not in resp["listing"]["metadata"]["acceptedCurrencies"]:
            raise TestFailure("ListingsTest - FAIL: Listing should have acceptedCurrences in metadata")

        print("ListingsTest - PASS")

if __name__ == '__main__':
    print("Running ListingTest")
    ListingsTest().main()
