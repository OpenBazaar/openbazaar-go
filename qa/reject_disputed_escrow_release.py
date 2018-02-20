import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class RejectDisputedEscrowRelease(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        merchant = self.nodes[0]
        customer = self.nodes[1]
        moderator = self.nodes[2]

        escrow_timeout_hours = 1

        # generate some coins and send them to customer
        time.sleep(4)
        api_url = customer["gateway_url"] + "wallet/address"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Address endpoint not found")
        else:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(20)

        # create a profile for moderator
        pro = {"name": "Moderator"}
        api_url = moderator["gateway_url"] + "ob/profile"
        r = requests.post(api_url, data=json.dumps(pro, indent=4))
        if r.status_code == 404:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Profile post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Profile POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # make moderator a moderator
        with open('testdata/moderation.json') as listing_file:
            moderation_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        api_url = moderator["gateway_url"] + "ob/moderator"
        r = requests.put(api_url, data=json.dumps(moderation_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Moderator post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Moderator POST failed. Reason: %s", resp["reason"])
        moderatorId = moderator["peerId"]
        time.sleep(4)

        # post profile for merchant
        with open('testdata/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = merchant["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # post listing to merchant
        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        if self.bitcoincash:
            listing_json["metadata"]["pricingCurrency"] = "tbch"
        slug = listing_json["slug"]
        listing_json["moderators"] = [moderatorId]
        listing_json["metadata"]["escrowTimeoutHours"] = escrow_timeout_hours
        api_url = merchant["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        slug = resp["slug"]
        time.sleep(4)

        # get listing hash
        api_url = merchant["gateway_url"] + "ipns/" + merchant["peerId"] + "/listings.json"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # customer send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        order_json["moderator"] = moderatorId
        api_url = customer["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            self.print_logs(merchant, "ob.log")
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]

        # check the purchase saved correctly
        api_url = customer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load order from Customer")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Customer purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Customer incorrectly saved as funded")

        # check the sale saved correctly
        api_url = merchant["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load order from Merchant")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Merchant purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Merchant incorrectly saved as funded")

        # fund order
        spend = {
            "address": payment_address,
            "amount": payment_amount,
            "feeLevel": "NORMAL"
        }
        api_url = customer["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check customer detected payment
        api_url = customer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load order from Customer")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Customer failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Customer incorrectly saved as unfunded")

        # check merchant detected payment
        api_url = merchant["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load order from Merchant")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Merchant failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Merchant incorrectly saved as unfunded")

        # merchant send order fulfillment
        with open('testdata/fulfillment.json') as fulfillment_file:
            fulfillment_json = json.load(fulfillment_file, object_pairs_hook=OrderedDict)
        fulfillment_json["slug"] = slug
        fulfillment_json["orderId"] = orderId
        api_url = merchant["gateway_url"] + "ob/orderfulfillment"
        r = requests.post(api_url, data=json.dumps(fulfillment_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Fulfillment post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Fulfillment POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # check customer received fulfillment
        api_url = customer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load order from Customer")
        resp = json.loads(r.text)
        if resp["state"] != "FULFILLED":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Customer failed to detect order fulfillment")

        # check merchant set fulfillment correctly
        api_url = merchant["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load order from Customer")
        resp = json.loads(r.text)
        if resp["state"] != "FULFILLED":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Merchant failed to order fulfillment")

        # Customer open dispute
        dispute = {
            "orderId": orderId,
            "claim": "Bastard ripped me off"
        }
        api_url = customer["gateway_url"] + "ob/opendispute/"
        r = requests.post(api_url, data=json.dumps(dispute, indent=4))
        if r.status_code == 404:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: OpenDispute post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: OpenDispute POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # Customer check dispute opened correctly
        api_url = customer["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load order from Customer")
        resp = json.loads(r.text)
        if resp["state"] != "DISPUTED":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Customer failed to detect his dispute")

        # Merchant check dispute opened correctly
        api_url = merchant["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load case from Marchant")
        resp = json.loads(r.text, object_pairs_hook=OrderedDict)
        if resp["state"] != "DISPUTED":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Merchant failed to detect the dispute")

        # Moderator check dispute opened correctly
        api_url = moderator["gateway_url"] + "ob/case/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Couldn't load case from Moderator")
        resp = json.loads(r.text, object_pairs_hook=OrderedDict)
        if resp["state"] != "DISPUTED":
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Moderator failed to detect the dispute")

        # Merchant attempt to release funds before dispute timeout hits
        release = {
            "OrderID": orderId,
        }
        api_url = merchant["gateway_url"] + "ob/releaseescrow/"
        r = requests.post(api_url, data=json.dumps(release, indent=4))
        if r.status_code == 500:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Release escrow internal server error %s", resp["reason"])
        elif r.status_code != 401:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Failed to raise error when releasing escrow after escrow timeout but before dispute timeout")

        # A hack allows dispute timeout to only be 10 seconds while running the server on testnet.
        time.sleep(20)

        for i in range(6):
            self.send_bitcoin_cmd("generate", 1)
            time.sleep(3)

        # Merchant attempt to release funds after timeout
        release = {
            "OrderID": orderId,
        }
        api_url = merchant["gateway_url"] + "ob/releaseescrow/"
        r = requests.post(api_url, data=json.dumps(release, indent=4))
        if r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("RejectDisputedEscrowRelease - FAIL: Release escrow error %s", resp["reason"])

        time.sleep(20)

        self.send_bitcoin_cmd("generate", 1)
        time.sleep(2)

        # Check the funds moved into merchant's wallet
        api_url = merchant["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            #unconfirmed = int(resp["unconfirmed"])
            if confirmed <= 0:
                raise TestFailure("RefundDirectTest - FAIL: Merchant failed to receive the multisig payout")
        else:
            raise TestFailure("RefundDirectTest - FAIL: Failed to query Merchant's balance")

        print("RejectDisputedEscrowRelease - PASS")

if __name__ == '__main__':
    print("Running RejectDisputedEscrowRelease")
    RejectDisputedEscrowRelease().main(["--regtest", "--disableexchangerates"])
