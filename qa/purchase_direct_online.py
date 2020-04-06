import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class PurchaseDirectOnlineTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        vendor = self.nodes[1]
        buyer = self.nodes[2]

        # generate some coins and send them to buyer
        api_url = buyer["gateway_url"] + "wallet/address/" + self.cointype
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(20)

        # post profile for vendor
        with open('testdata/'+ self.vendor_version +'/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = vendor["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listing to vendor
        with open('testdata/'+ self.vendor_version +'/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        listing_json["metadata"]["acceptedCurrencies"] = ["t" + self.cointype]
        if self.vendor_version == 4:
            listing_json["metadata"]["priceCurrency"] = "t" + self.cointype
        else:
            listing_json["item"]["priceCurrency"]["code"] = "t" + self.cointype

        api_url = vendor["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # get listing hash
        api_url = vendor["gateway_url"] + "ob/listings/" + vendor["peerId"]
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]


        # buyer send order
        with open('testdata/'+ self.buyer_version +'/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        order_json["paymentCoin"] = "t" + self.cointype
        api_url = buyer["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]

        # check the purchase saved correctly
        api_url = buyer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Couldn't load order from Buyer")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Buyer purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Buyer incorrectly saved as funded")

        # check the sale saved correctly
        api_url = vendor["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Couldn't load order from Vendor")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Vendor purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Vendor incorrectly saved as funded")

        # fund order
        spend = {
            "currencyCode": "T" + self.cointype,
            "address": payment_address,
            "amount": payment_amount["amount"],
            "feeLevel": "NORMAL",
            "requireAssociateOrder": False
        }
        if self.buyer_version == 4:
            spend["amount"] = payment_amount
            spend["wallet"] = "T" + self.cointype

        api_url = buyer["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check buyer detected payment
        api_url = buyer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Couldn't load order from Buyer")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Buyer failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Buyer incorrectly saved as unfunded")

        # check vendor detected payment
        api_url = vendor["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Couldn't load order from Vendor")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Vendor failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Vendor incorrectly saved as unfunded")

        # buyer send order
        with open('testdata/'+ self.buyer_version +'/order_direct_too_much_quantity.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)

        order_json["items"][0]["listingHash"] = listingId
        order_json["paymentCoin"] = "t" + self.cointype
        api_url = buyer["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        resp = json.loads(r.text)
        if r.status_code == 200:
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Purchase POST should have failed failed.")
        if resp["reason"] != "not enough inventory":
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Purchase POST failed with incorrect reason: %s", resp["reason"])
        if resp["code"] != "ERR_INSUFFICIENT_INVENTORY":
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Purchase POST failed with incorrect code: %s", resp["code"])
        if resp["remainingInventory"] != "6":
            raise TestFailure("PurchaseDirectOnlineTest - FAIL: Purchase POST failed with incorrect remainingInventory: %d", resp["remainingInventory"])

        print("PurchaseDirectOnlineTest - PASS")


if __name__ == '__main__':
    print("Running PurchaseDirectOnlineTest")
    PurchaseDirectOnlineTest().main(["--regtest", "--disableexchangerates"])
