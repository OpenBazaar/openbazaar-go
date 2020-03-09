import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class EthPurchaseCryptoListingTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

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
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Unknown response")
        time.sleep(20)

        # post profile for vendor
        with open('testdata/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = vendor["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listing to vendor
        with open('testdata/eth_listing_crypto.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        listing_json["metadata"]["acceptedCurrencies"] = ["T" + self.cointype]

        api_url = vendor["gateway_url"] + "ob/listing"
        print("api_url   ", api_url)
        print(json.dumps(listing_json, indent=4))
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        slug = json.loads(r.text)["slug"]

        # check inventory
        api_url = vendor["gateway_url"] + "ob/inventory"
        r = requests.get(api_url, data=json.dumps(listing_json, indent=4))
        resp = json.loads(r.text)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Inventory get endpoint failed")
        if resp["ether"]["inventory"] != '350000000000000000':
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Inventory incorrect: %d", resp["ether"]["inventory"])

        # get listing hash
        api_url = vendor["gateway_url"] + "ob/listings/" + vendor["peerId"]
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        if resp[0]["coinType"] != "TETH":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Vendor incorrectly saved listings.json without a coinType")
        listingId = resp[0]["hash"]

        # buyer send order
        with open('testdata/order_crypto.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        order_json["paymentCoin"] = "T" + self.cointype
        api_url = buyer["gateway_url"] + "ob/purchase"
        print("before order purchase", api_url)
        print(json.dumps(order_json, indent=4))
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]
        if int(payment_amount["amount"]) <= 0:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Purchase POST failed: paymentAmount is <= 0")

        # check the purchase saved correctly
        api_url = buyer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Couldn't load order from Buyer")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer incorrectly saved as funded")
        if resp["contract"]["vendorListings"][0]["metadata"]["coinType"] != "TETH":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer incorrectly saved without a coinType")
        if resp["contract"]["buyerOrder"]["items"][0]["paymentAddress"] != "crypto_payment_address":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer incorrectly saved without a paymentAddress")

        # check the sale saved correctly
        api_url = vendor["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Couldn't load order from Vendor")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Vendor purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Vendor incorrectly saved as funded")
        if resp["contract"]["vendorListings"][0]["metadata"]["coinType"] != "TETH":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Vendor incorrectly saved without a coinType")
        if resp["contract"]["buyerOrder"]["items"][0]["paymentAddress"] != "crypto_payment_address":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Vendor incorrectly saved without a paymentAddress")
        if resp["contract"]["buyerOrder"]["items"][0]["memo"] != "thanks!":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Vendor incorrectly saved without a memo")

        # fund order
        spend = {
            "currencyCode": "T" + self.cointype,
            "address": payment_address,
            "amount": payment_amount["amount"],
            "feeLevel": "NORMAL",
            "requireAssociateOrder": True,
            "orderID": orderId
        }
        api_url = buyer["gateway_url"] + "ob/orderspend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check buyer detected payment
        api_url = buyer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Couldn't load order from Buyer")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer incorrectly saved as unfunded")
        if resp["contract"]["vendorListings"][0]["metadata"]["coinType"] != "TETH":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer incorrectly saved without a coinType")
        if resp["contract"]["buyerOrder"]["items"][0]["paymentAddress"] != "crypto_payment_address":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer incorrectly saved without a paymentAddress")

        # check vendor detected payment
        time.sleep(120)
        api_url = vendor["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Couldn't load order from Vendor")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Vendor failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Vendor incorrectly saved as unfunded")

        with open('testdata/fulfillment_crypto.json') as fulfillment_file:
            fulfillment_json = json.load(fulfillment_file, object_pairs_hook=OrderedDict)
        fulfillment_json["orderId"] = orderId
        fulfillment_json["slug"] = slug
        api_url = vendor["gateway_url"] + "ob/orderfulfillment"
        r = requests.post(api_url, data=json.dumps(fulfillment_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Fulfillment post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Fulfillment POST failed. Reason: %s", resp["reason"])

        # check buyer received fulfillment
        api_url = buyer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Couldn't load order from Buyer")
        resp = json.loads(r.text)
        if resp["state"] != "FULFILLED":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer failed to detect order fulfillment")
        if resp["contract"]["vendorOrderFulfillment"][0]["cryptocurrencyDelivery"][0]["transactionID"] != "crypto_transaction_id":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Buyer failed to detect transactionID")

        api_url = vendor["gateway_url"] + "ob/inventory"
        r = requests.get(api_url, data=json.dumps(listing_json, indent=4))
        resp = json.loads(r.text)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Inventory get endpoint failed")
        if resp["ether"]["inventory"] != "340000000000000000":
            raise TestFailure("EthPurchaseCryptoListingTest - FAIL: Inventory incorrect: %d", resp["ether"]["inventory"])

        print("EthPurchaseCryptoListingTest - PASS")


if __name__ == '__main__':
    print("Running EthPurchaseCryptoListingTest")
    EthPurchaseCryptoListingTest().main(["--regtest"])
