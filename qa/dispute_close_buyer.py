import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class DisputeCloseBuyerTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]
        charlie = self.nodes[2]

        # generate some coins and send them to bob
        generated_coins = 10
        time.sleep(4)
        api_url = bob["gateway_url"] + "wallet/address"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, generated_coins)
        time.sleep(20)

        # create a profile for charlie
        pro = {"name": "Charlie"}
        api_url = charlie["gateway_url"] + "ob/profile"
        r = requests.post(api_url, data=json.dumps(pro, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Profile post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Profile POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # make charlie a moderator
        with open('testdata/moderation.json') as listing_file:
            moderation_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        api_url = charlie["gateway_url"] + "ob/moderator"
        r = requests.put(api_url, data=json.dumps(moderation_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Moderator post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Moderator POST failed. Reason: %s", resp["reason"])
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
        if self.bitcoincash:
            listing_json["metadata"]["pricingCurrency"] = "tbch"

        listing_json["moderators"] = [moderatorId]
        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # get listing hash
        api_url = alice["gateway_url"] + "ob/listings/" + alice["peerId"]
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # bob send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        order_json["moderator"] = moderatorId
        api_url = bob["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            self.print_logs(alice, "ob.log")
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]

        # check the purchase saved correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob incorrectly saved as funded")

        # check the sale saved correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_PAYMENT":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Alice purchase saved in incorrect state")
        if resp["funded"] == True:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Alice incorrectly saved as funded")

        # fund order
        spend = {
            "address": payment_address,
            "amount": payment_amount,
            "feeLevel": "NORMAL"
        }
        api_url = bob["gateway_url"] + "wallet/spend"
        r = requests.post(api_url, data=json.dumps(spend, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        # check bob detected payment
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob failed to detect his payment")
        if resp["funded"] == False:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob incorrectly saved as unfunded")

        # check alice detected payment
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "AWAITING_FULFILLMENT":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Alice failed to detect payment")
        if resp["funded"] == False:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Alice incorrectly saved as unfunded")
        
        # Bob open dispute
        dispute = {
            "orderId": orderId,
            "claim": "Bastard ripped me off"
        }
        api_url = bob["gateway_url"] + "ob/opendispute/"
        r = requests.post(api_url, data=json.dumps(dispute, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: OpenDispute post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("DisputeCloseBuyerTest - FAIL: OpenDispute POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # Bob check dispute opened correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text)
        if resp["state"] != "DISPUTED":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob failed to detect his dispute")

        # Alice check dispute opened correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "DISPUTED":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Alice failed to detect the dispute")

        # Charlie check dispute opened correctly
        api_url = charlie["gateway_url"] + "ob/case/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load case from Clarlie")
        resp = json.loads(r.text, object_pairs_hook=OrderedDict)
        if resp["state"] != "DISPUTED":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Charlie failed to detect the dispute")

        # Charlie close dispute
        dispute_resolution = {
            "OrderID": orderId,
            "Resolution": "I'm siding with Bob",
            "BuyerPercentage": 100,
            "VendorPercentage": 0
        }
        api_url = charlie["gateway_url"] + "ob/closedispute/"
        r = requests.post(api_url, data=json.dumps(dispute_resolution, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: CloseDispute post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("DisputeCloseBuyerTest - FAIL: CloseDispute POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # Alice check dispute closed correctly
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text)
        if resp["state"] != "DECIDED":
            self.print_logs(alice, "ob.log")
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Alice failed to detect the dispute resolution")

        # Bob check dispute closed correctly
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text, object_pairs_hook=OrderedDict)
        if resp["state"] != "DECIDED":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob failed to detect the dispute resolution")

        # Charlie check dispute closed correctly
        api_url = charlie["gateway_url"] + "ob/case/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load case from Charlie")
        resp = json.loads(r.text, object_pairs_hook=OrderedDict)
        if resp["state"] != "RESOLVED":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Charlie failed to detect the dispute resolution")

        # Bob relase funds
        release = {
            "OrderID": orderId,
        }
        api_url = bob["gateway_url"] + "ob/releasefunds/"
        r = requests.post(api_url, data=json.dumps(release, indent=4))
        if r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: ReleaseFunds post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("DisputeCloseBuyerTest - FAIL: ReleaseFunds POST failed. Reason: %s", resp["reason"])
        time.sleep(20)

        self.send_bitcoin_cmd("generate", 1)
        time.sleep(5)

        # Check bob received payout
        api_url = bob["gateway_url"] + "wallet/balance"
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            confirmed = int(resp["confirmed"])
            #unconfirmed = int(resp["unconfirmed"])
            if confirmed <= (generated_coins*100000000) - payment_amount:
                raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob failed to detect dispute payout")
        elif r.status_code == 404:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Receive coins endpoint not found")
        else:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Unknown response")

        # Bob check payout transaction recorded
        api_url = bob["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Bob")
        resp = json.loads(r.text, object_pairs_hook=OrderedDict)
        if len(resp["paymentAddressTransactions"]) != 2:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob failed to record payout transaction")
        if resp["state"] != "RESOLVED":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Bob failed to set state to RESOLVED")

        # Alice check payout transaction recorded
        api_url = alice["gateway_url"] + "ob/order/" + orderId
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Couldn't load order from Alice")
        resp = json.loads(r.text, object_pairs_hook=OrderedDict)
        if len(resp["paymentAddressTransactions"]) != 2:
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Alice failed to record payout transaction")
        if resp["state"] != "RESOLVED":
            raise TestFailure("DisputeCloseBuyerTest - FAIL: Alice failed to set state to RESOLVED")

        print("DisputeCloseBuyerTest - PASS")

if __name__ == '__main__':
    print("Running DisputeCloseBuyerTest")
    DisputeCloseBuyerTest().main(["--regtest", "--disableexchangerates"])
