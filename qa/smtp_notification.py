import requests
import json
import time
import subprocess
import re
import os

from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure
from test_framework.smtp_server import SMTP_DUMPFILE


class SMTPTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def run_test(self):
        alice = self.nodes[1]
        bob = self.nodes[2]

        # post profile for alice
        with open('testdata/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = alice["gateway_url"] + "ob/profile"
        requests.post(api_url, data=json.dumps(profile_json, indent=4))

        # configure SMTP notifications
        time.sleep(4)
        api_url = alice["gateway_url"] + "ob/settings"
        smtp = {
            "smtpSettings" : {
                "notifications": True,
                "serverAddress": "0.0.0.0:1024",
                "username": "usr",
                "password": "passwd",
                "senderEmail": "openbazaar@test.org",
                "recipientEmail": "user.openbazaar@test.org"
            }
        }

        r = requests.post(api_url, data=json.dumps(smtp, indent=4))
        if r.status_code == 404:
            raise TestFailure("SMTPTest - FAIL: Settings POST endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SMTPTest - FAIL: Settings POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # check SMTP settings
        api_url = alice["gateway_url"] + "ob/settings"
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("SMTPTest - FAIL: Settings GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SMTPTest - FAIL: Settings GET failed. Reason: %s", resp["reason"])

        # check notifications
        addr = "0.0.0.0:1024"
        class_name = "test_framework.smtp_server.SMTPTestServer"
        proc = subprocess.Popen(["python", "-m", "smtpd", "-n", "-c", class_name, addr])

        # generate some coins and send them to bob
        time.sleep(4)
        api_url = bob["gateway_url"] + "wallet/address/" + self.cointype
        r = requests.get(api_url)
        if r.status_code == 200:
            resp = json.loads(r.text)
            address = resp["address"]
        elif r.status_code == 404:
            raise TestFailure("SMTPTest - FAIL: Address endpoint not found")
        else:
            raise TestFailure("SMTPTest - FAIL: Unknown response")
        self.send_bitcoin_cmd("sendtoaddress", address, 10)
        time.sleep(20)

        # post listing to alice
        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)
        listing_json["metadata"]["pricingCurrency"] = "t" + self.cointype

        api_url = alice["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("SMTPTest - FAIL: Listing post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SMTPTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # get listing hash
        api_url = alice["gateway_url"] + "ipns/" + alice["peerId"] + "/listings.json"
        r = requests.get(api_url)
        if r.status_code != 200:
            raise TestFailure("SMTPTest - FAIL: Couldn't get listing index")
        resp = json.loads(r.text)
        listingId = resp[0]["hash"]

        # bob send order
        with open('testdata/order_direct.json') as order_file:
            order_json = json.load(order_file, object_pairs_hook=OrderedDict)
        order_json["items"][0]["listingHash"] = listingId
        order_json["paymentCoin"] = "t" + self.cointype
        api_url = bob["gateway_url"] + "ob/purchase"
        r = requests.post(api_url, data=json.dumps(order_json, indent=4))
        if r.status_code == 404:
            raise TestFailure("SMTPTest - FAIL: Purchase post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SMTPTest - FAIL: Purchase POST failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        orderId = resp["orderId"]
        payment_address = resp["paymentAddress"]
        payment_amount = resp["amount"]

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
            raise TestFailure("SMTPTest - FAIL: Spend post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SMTPTest - FAIL: Spend POST failed. Reason: %s", resp["reason"])
        time.sleep(20)
        proc.terminate()

        # check notification
        expected = '''From: openbazaar@test.org
To: user.openbazaar@test.org
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8
Subject: [OpenBazaar] Order received

You received an order "Ron Swanson Tshirt".

Order ID: QmNiPgKNq27qQE8fRxMbtDfRcFDEYMH5wDRgdqtqoWBpGg
Buyer: Qmd5qDpcYkHCmkj9pMXU9TKBqEDWgEmtoHD5xjdJgumaHg
Thumbnail: QmS73grfbWgWrNztd8Lns9GCG3jjRNDfcPYg2VYQzKDZSt
Timestamp: 1487699826
'''
        expected_lines = [e for e in expected.splitlines() if not e.startswith('Timestamp:') and not e.startswith('Order ID:')]
        with open(SMTP_DUMPFILE, 'r') as f:
            res_lines = [l.strip() for l in f.readlines() if not l.startswith('Timestamp') and not l.startswith('Order ID:')]
            if res_lines != expected_lines:
                os.remove(SMTP_DUMPFILE)
                raise TestFailure("SMTPTest - FAIL: Incorrect mail data received")
        os.remove(SMTP_DUMPFILE)
        print("SMTPTest - PASS")

if __name__ == '__main__':
    print("Running SMTPTest")
    SMTPTest().main(["--regtest", "--disableexchangerates"])
