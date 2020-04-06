import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class CompleteDirectOnlineTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        alice = self.nodes[1]
        bob = self.nodes[2]

        # generate some coins and send them to bob
        time.sleep(4)
        api_url = bob["gateway_url"] + "wallet/address/" + self.cointype
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(20)

        # post profile for alice
        with open('testdata/'+ self.vendor_version +'/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = alice["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listing to alice
        with open('testdata/'+ self.vendor_version +'/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        listing_json["metadata"]["acceptedCurrencies"] = ["T" + self.cointype]
        if self.vendor_version == 4:
            listing_json["metadata"]["priceCurrency"] = "t" + self.cointype
        else:
            listing_json["item"]["priceCurrency"]["code"] = "t" + self.cointype

        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        slug = resp["slug"]
        time.sleep(4)

        # get listing hash
        api_url = alice["gateway_url"] + "ob/listings/" + alice["peerId"]
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # bob send order
        with open('testdata/'+ self.buyer_version +'/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        order_json["paymentCoin"] = "t" + self.cointype
        api_url = bob["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]

        # check the purchase saved correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Bob purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Bob incorrectly saved as funded")

        # check the sale saved correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Alice purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Alice incorrectly saved as funded")

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

        api_url = bob["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(30)

        # check bob detected payment
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Bob failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Bob incorrectly saved as unfunded")

        # check alice detected payment
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Alice failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Alice incorrectly saved as unfunded")
        
        # alice send order fulfillment
        with open('testdata/'+ self.vendor_version +'/fulfillment.json') as fulfillment_file:
            fulfillment_json = json.load(fulfillment_file, object_pairs_hook=OrderedDict)
        fulfillment_json["orderId"] = orderId
        fulfillment_json["slug"] = slug
        api_url = alice["gateway_url"] + "ob/orderfulfillment"
        r = requests.post(api_url, data=json.dumps(fulfillment_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Fulfillment post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Fulfillment POST failed. Reason: %s", resp["reason"])
        time.sleep(5)

        # check bob received fulfillment
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "FULFILLED":
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Bob failed to detect order fulfillment")

        # check alice set fulfillment correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "FULFILLED":
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Alice failed to order fulfillment")

        # bob send order completion
        oc = {
            "orderId": orderId,
            "ratings": [
                {
                    "slug": slug,
                    "overall": 4,
                    "quality": 5,
                    "description": 5,
                    "customerService": 4,
                    "deliverySpeed": 3,
                    "review": "I love it!"
                }
            ]
        }
        api_url = bob["gateway_url"] + "ob/ordercompletion"
        r = requests.post(api_url, data=json.dumps(oc, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Completion post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Completion POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # check alice received completion
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "COMPLETED":
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Alice failed to detect order completion")

        # check bob set completion correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "COMPLETED":
            raise TestFailure("CompleteDirectOnlineTest - FAIL: Bob failed to order completion")

        print("CompleteDirectOnlineTest - PASS")


if __name__ == '__main__':
    print("Running CompleteDirectOnlineTest")
    CompleteDirectOnlineTest().main(["--regtest", "--disableexchangerates"])
