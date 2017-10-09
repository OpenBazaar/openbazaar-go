import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class CancelDirectOfflineTest(OpenBazaarTestFramework):

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
            raise TestFailure("CancelDirectOfflineTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Unknown response")
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
        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CancelDirectOfflineTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # get listing hash
        api_url = alice["gateway_url"] + "ipns/" + alice["peerId"] + "/listings.json"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # bob fetch listing to cache
        api_url = bob["gateway_url"] + "ipfs/" + listingId
        requests.get(api_url)

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
            raise TestFailure("CancelDirectOfflineTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CancelDirectOfflineTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]
        if resp["vendorOnline"] == True:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Purchase returned vendor is online")

        # check the purchase saved correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("CancelDirectOfflineTest - FAIL: Bob purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Bob incorrectly saved as funded")

        # fund order
        spend = {
            "address": payment_address,
            "amount": payment_amount,
            "feeLevel": "NORMAL"
        }
        api_url = bob["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CancelDirectOfflineTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check bob detected payment
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if len(resp["paymentAddressTransactions"]) <= 0:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Bob failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Bob incorrectly saved as unfunded")
        if resp["state"] != "PENDING":
            raise TestFailure("CancelDirectOfflineTest - FAIL: Bob purchase saved in incorrect state")

        # bob cancel order
        api_url = bob["gateway_url"] + "ob/ordercancel"
        cancel = {"orderId": orderId}
        r = requests.post(api_url, data=json.dumps(cancel, indent=4))
        if r.status_code == 404:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CancelDirectOfflineTest - FAIL: Cancel POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # bob check order canceled correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "CANCELED":
            raise TestFailure("CancelDirectOfflineTest - FAIL: Bob failed to save as canceled")
        if "refundAddressTransaction" not in resp or resp["refundAddressTransaction"] == {}:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Bob failed to detect outgoing payment")

        # startup alice again
        self.start_node(alice)
        self.send_bitcoin_cmd("generate", 1)
        time.sleep(45)

        # check alice detected order
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Couldn't load order from Alice %s", r.status_code)
        resp = json.loads(r.text)
        if resp["state"] != "CANCELED":
            raise TestFailure("CancelDirectOfflineTest - FAIL: Alice failed to detect order cancellation")

        # Check the funds moved into bob's wallet
        api_url = bob["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            #unconfirmed = int(resp["unconfirmed"])
            if confirmed <= 50 - payment_amount:
                raise TestFailure("CancelDirectOfflineTest - FAIL: Bob failed to receive the multisig payout")
        else:
            raise TestFailure("CancelDirectOfflineTest - FAIL: Failed to query Bob's balance")

        print("CancelDirectOfflineTest - PASS")

if __name__ == '__main__':
    print("Running CancelDirectOfflineTest")
    CancelDirectOfflineTest().main(["--regtest", "--disableexchangerates"])
