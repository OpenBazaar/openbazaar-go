import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class CompleteModeratedWithTimeout(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]
        charlie = self.nodes[2]

        # generate some coins and send them to bob
        time.sleep(4)
        api_url = bob["gateway_url"] + "wallet/address/" + self.cointype
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Address endpoint not found")
        else:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(20)

        # create a profile for charlie
        pro = {"name": "Charlie"}
        api_url = charlie["gateway_url"] + "ob/profile"
        r = requests.post(api_url, data=json.dumps(pro, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Profile post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Profile POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # make charlie a moderator
        with open('testdata/moderation.json') as listing_file:
            moderation_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        api_url = charlie["gateway_url"] + "ob/moderator"
        r = requests.put(api_url, data=json.dumps(moderation_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Moderator post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Moderator POST failed. Reason: %s", resp["reason"])
        moderatorId = charlie["peerId"]
        time.sleep(4)

        # post profile for alice
        with open('testdata/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = alice["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listing to alice
        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        listing_json["metadata"]["pricingCurrency"] = "t" + self.cointype
        slug = listing_json["slug"]
        listing_json["moderators"] = [moderatorId]
        listing_json["metadata"]["escrowTimeoutHours"] = 1000
        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        slug = resp["slug"]
        time.sleep(4)

        # get listing hash
        api_url = alice["gateway_url"] + "ob/listings/" + alice["peerId"]
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # bob send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        order_json["moderator"] = moderatorId
        order_json["paymentCoin"] = "t" + self.cointype
        api_url = bob["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            self.print_logs(alice, "ob.log")
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]

        # check the purchase saved correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Bob purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Bob incorrectly saved as funded")

        # check the sale saved correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Alice purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Alice incorrectly saved as funded")

        # fund order
        spend = {
            "wallet": self.cointype,
            "address": payment_address,
            "amount": payment_amount,
            "feeLevel": "NORMAL"
        }
        api_url = bob["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check bob detected payment
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Bob failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Bob incorrectly saved as unfunded")

        # check alice detected payment
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Alice failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Alice incorrectly saved as unfunded")

        # alice send order fulfillment
        with open('testdata/fulfillment.json') as fulfillment_file:
            fulfillment_json = json.load(fulfillment_file, object_pairs_hook=OrderedDict)
        fulfillment_json["slug"] = slug
        fulfillment_json["orderId"] = orderId
        api_url = alice["gateway_url"] + "ob/orderfulfillment"
        r = requests.post(api_url, data=json.dumps(fulfillment_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Fulfillment post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Fulfillment POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # check bob received fulfillment
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "FULFILLED":
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Bob failed to detect order fulfillment")

        # check alice set fulfillment correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "FULFILLED":
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Alice failed to order fulfillment")

        # bob send order completion
        with open('testdata/completion.json') as completion_file:
            completion_json = json.load(completion_file, object_pairs_hook=OrderedDict)
        completion_json["orderId"] = orderId
        completion_json["ratings"][0]["slug"] = slug
        api_url = bob["gateway_url"] + "ob/ordercompletion"
        r = requests.post(api_url, data=json.dumps(completion_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Completion post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Completion POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # check alice received completion
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "COMPLETED":
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Alice failed to detect order completion")

        # check bob set completion correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "COMPLETED":
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Bob failed to order completion")

        self.send_bitcoin_cmd("generate", 1)
        time.sleep(2)

        # Check the funds moved into alice's wallet
        api_url = alice["gateway_url"] + "wallet/balance/" + self.cointype
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            #unconfirmed = int(resp["unconfirmed"])
            if confirmed <= 0:
                raise TestFailure("CompleteModeratedWithTimeout - FAIL: Alice failed to receive the multisig payout")
        else:
            raise TestFailure("CompleteModeratedWithTimeout - FAIL: Failed to query Alice's balance")

        print("CompleteModeratedWithTimeout - PASS")

if __name__ == '__main__':
    print("Running CompleteModeratedWithTimeout")
    CompleteModeratedWithTimeout().main(["--regtest", "--disableexchangerates"])
