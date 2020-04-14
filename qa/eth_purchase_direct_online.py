import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class EthPurchaseDirectOnlineTest(OpenBazaarTestFramework):

    def __init__(self):
        print("i am in pur init....")
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        vendor = self.nodes[1]
        buyer = self.nodes[2]

        # generate some coins and send them to buyer
        api_url = buyer["gateway_url"] + "wallet/address/" + self.cointype
        print("see the wallet address : ", api_url)
        r = requests.get(api_url)
        print("raw resp : ")
        print(r)
        if r.status_code == 200:
            print("resp : ", json.loads(r.text))
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Unknown response")
        time.sleep(20)

        # post profile for vendor
        with open('testdata/v5/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = vendor["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listing to vendor
        with open('testdata/v5/eth_listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        listing_json["item"]["priceCurrency"]["code"] = "T" + self.cointype
        listing_json["metadata"]["acceptedCurrencies"] = ["T" + self.cointype]

        api_url = vendor["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # get listing hash
        api_url = vendor["gateway_url"] + "ob/listings/" + vendor["peerId"]
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]


        # buyer send order
        with open('testdata/v5/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        order_json["paymentCoin"] = "T" + self.cointype
        api_url = buyer["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]


        # check the purchase saved correctly
        api_url = buyer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Couldn't load order from Buyer")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Buyer purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Buyer incorrectly saved as funded")

        # check the sale saved correctly
        api_url = vendor["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Couldn't load order from Vendor")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Vendor purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Vendor incorrectly saved as funded")

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
        print("orderspend spend ... ", api_url)
        print("payload : ", json.dumps(spend, indent=4))
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check buyer detected payment
        api_url = buyer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Couldn't load order from Buyer")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Buyer failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Buyer incorrectly saved as unfunded")

        # check vendor detected payment
        api_url = vendor["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Couldn't load order from Vendor")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Vendor failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Vendor incorrectly saved as unfunded")

        # buyer send order
        with open('testdata/v5/order_direct_too_much_quantity.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)

        order_json["items"][0]["listingHash"] = listingId
        order_json["paymentCoin"] = "T" + self.cointype
        api_url = buyer["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        resp = json.loads(r.text)
        print("after purchasing too much stuff, the response : ")
        print(r.status_code)
        print(resp)
        if r.status_code == 200:
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Purchase POST should have failed failed.")
        if resp["reason"] != "not enough inventory":
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Purchase POST failed with incorrect reason: %s", resp["reason"])
        if resp["code"] != "ERR_INSUFFICIENT_INVENTORY":
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Purchase POST failed with incorrect code: %s", resp["code"])
        if resp["remainingInventory"] != '6':
            raise TestFailure("EthPurchaseDirectOnlineTest - FAIL: Purchase POST failed with incorrect remainingInventory: %d", resp["remainingInventory"])

        print("EthPurchaseDirectOnlineTest - PASS")


if __name__ == '__main__':
    print("Running EthEthPurchaseDirectOnlineTest")
    EthPurchaseDirectOnlineTest().main(["--regtest", "--disableexchangerates"])
