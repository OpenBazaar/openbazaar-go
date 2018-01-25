import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class OutOfInventoryTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]

        # generate some coins and send them to bob
        time.sleep(4)
        api_url = bob["gateway_url"] + "wallet/address"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("OutOfInventoryTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("OutOfInventoryTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(20)

        # post profile for alice
        with open('testdata/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = alice["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listing to alice
        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        if self.bitcoincash:
            listing_json["metadata"]["pricingCurrency"] = "tbch"
        listing_json["item"]["skus"][4]["quantity"] = 0

        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("OutOfInventoryTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("OutOfInventoryTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # get listing hash
        api_url = alice["gateway_url"] + "ipns/" + alice["peerId"] + "/listings.json"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("OutOfInventoryTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # bob send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        api_url = bob["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("OutOfInventoryTest - FAIL: Purchase post endpoint not found")
        elif r.status_code == 200:
            raise TestFailure("OutOfInventoryTest - FAIL: Purchase POST should have returned an error")

        # shutdown alice
        api_url = alice["gateway_url"] + "ob/shutdown"
        requests.post(api_url, data="")
        time.sleep(4)

        # bob send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        api_url = bob["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]
        if resp["vendorOnline"] == True:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Purchase returned vendor is online")

        # check the purchase saved correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Bob purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Bob incorrectly saved as funded")

        # fund order
        spend = {
            "address": payment_address,
            "amount": payment_amount,
            "feeLevel": "NORMAL"
        }
        api_url = bob["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check bob detected payment
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if len(resp["paymentAddressTransactions"]) <= 0:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Bob failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Bob incorrectly saved as unfunded")
        if resp["state"] != "PENDING":
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Bob purchase saved in incorrect state")

        # generate one more block containing this tx
        self.send_bitcoin_cmd("generate", 1)

        # startup alice again
        self.start_node(alice)
        time.sleep(45)

        # check alice detected order and payment
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "PENDING":
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Alice failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Alice incorrectly saved as unfunded")

        # check alice balance is zero
        api_url = alice["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            unconfirmed = int(resp["unconfirmed"])
            if confirmed + unconfirmed > 0:
                raise TestFailure("PurchaseDirectOfflineTest - FAIL: Alice should have zero balance at this point")
        else:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Failed to query Alice's balance")
        time.sleep(1)

        # alice confirm offline order
        api_url = alice["gateway_url"] + "ob/orderconfirmation"
        oc = {
            "orderId": orderId,
            "reject": False
        }
        r = requests.post(api_url, data=json.dumps(oc, indent=4))
        if r.status_code == 404:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Order confirmation post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: order confirmation POST failed. Reason: %s", resp["reason"])
        time.sleep(10)

        self.send_bitcoin_cmd("generate", 1)
        time.sleep(2)

        # Check the funds moved into alice's wallet
        api_url = alice["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            # unconfirmed = int(resp["unconfirmed"])
            if confirmed <= 0:
                raise TestFailure("PurchaseDirectOfflineTest - FAIL: Alice failed to receive the multisig payout")
        else:
            raise TestFailure("PurchaseDirectOfflineTest - FAIL: Failed to query Alice's balance")

        # check bob detected order confirmation and outgoing transaction
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("OutOfInventoryTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("OutOfInventoryTest - FAIL: Bob failed to set state correctly")
        if resp["funded"] == False:
            raise TestFailure("OutOfInventoryTest - FAIL: Bob incorrectly saved as unfunded")

        # check alice set state correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("OutOfInventoryTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("OutOfInventoryTest - FAIL: Alice failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("OutOfInventoryTest - FAIL: Alice incorrectly saved as unfunded")

        print("OutOfInventoryTest - PASS")

if __name__ == '__main__':
    print("Running OutOfInventoryTest")
    OutOfInventoryTest().main(["--regtest", "--disableexchangerates"])
