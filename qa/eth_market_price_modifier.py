import requests
import json
import time
from copy import deepcopy
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class EthMarketPriceModifierTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3
        self.price_modifier = 10.25

    def run_test(self):
        vendor = self.nodes[1]
        buyer = self.nodes[2]

        # generate some coins and send them to buyer
        time.sleep(4)
        api_url = buyer["gateway_url"] + "wallet/address/" + self.cointype
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Unknown response")
        time.sleep(20)

        # post profile for vendor
        with open('testdata/v5/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = vendor["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listings to vendor
        with open('testdata/v5/eth_listing_crypto.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
            listing_json["metadata"]["coinType"] = "TETH"
            listing_json["metadata"]["coinDivisibility"] = 18
            listing_json["metadata"]["acceptedCurrencies"] = ["T" + self.cointype]
            listing_json_with_modifier = deepcopy(listing_json)
            listing_json_with_modifier["item"]["priceModifier"] = self.price_modifier

        api_url = vendor["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        slug = json.loads(r.text)["slug"]

        api_url = vendor["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json_with_modifier, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Listing POST 2 failed. Reason: %s", resp["reason"])
        slug_with_modifier = json.loads(r.text)["slug"]

        # check vendor's local listings and check for modifier
        api_url = vendor["gateway_url"] + "ob/listings"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Couldn't get vendor local listings")
        resp = json.loads(r.text)
        for listing in resp:
            if "modifier" not in listing:
                raise TestFailure("EthMarketPriceModifierTest - FAIL: Vendor's local listings index doesn't include price modifier")

        # check vendor's listings from buyer and check for modifier
        api_url = buyer["gateway_url"] + "ob/listings/" + vendor["peerId"]
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Couldn't get vendor listings from buyer")
        resp = json.loads(r.text)
        for listing in resp:
            if "modifier" not in listing:
                raise TestFailure("EthMarketPriceModifierTest - FAIL: Vendor's listings don't include price modifier from buyer")

        # get listing hashes
        api_url = vendor["gateway_url"] + "ipns/" + vendor["peerId"] + "/listings.json"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listing_id = resp[0]["hash"]
        listing_id_with_modifier = resp[1]["hash"]

        # get second listing and check for modifier
        slug = resp[1]["slug"]
        api_url = buyer["gateway_url"] + "ob/listing/" + vendor["peerId"] + "/" + slug
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Couldn't get vendor listings")
        resp = json.loads(r.text)
        if "priceModifier" not in resp["listing"]["metadata"]:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Listing doesn't include priceModifier")

        # buyer send orders
        with open('testdata/v5/order_crypto.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listing_id
        order_json["paymentCoin"] = "T" + self.cointype
        api_url = buyer["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        payment_address = resp["paymentAddress"]
        payment_amount = int(resp["amount"]["amount"])

        with open('testdata/v5/order_crypto.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listing_id_with_modifier
        order_json["paymentCoin"] = "T" + self.cointype
        api_url = buyer["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthMarketPriceModifierTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        payment_address_with_modifier = resp["paymentAddress"]
        payment_amount_with_modifier = int(resp["amount"]["amount"])

        # Check that modified price is different than regular price
        pct_change = round((payment_amount-payment_amount_with_modifier) / payment_amount * -100, 2)
        if pct_change != self.price_modifier:
            raise TestFailure(f"EthMarketPriceModifierTest - FAIL: Incorrect price modification: wanted {self.price_modifier} but got {pct_change}")

        print("EthMarketPriceModifierTest - PASS")


if __name__ == '__main__':
    print("Running EthMarketPriceModifierTest")
    EthMarketPriceModifierTest().main(["--regtest"])
