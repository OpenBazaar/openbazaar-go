import requests
import json
from datetime import datetime
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class CachedInventoryTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 2

    def setup_network(self):
        self.setup_nodes()

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]

        with open('testdata/listing.json') as listing_file:
            listing_json = json.load(listing_file, object_pairs_hook=OrderedDict)

        # Create profile
        with open('testdata/profile.json') as profile_file:
            profile_json = json.load(profile_file, object_pairs_hook=OrderedDict)
        api_url = alice["gateway_url"] + "ob/profile"
        r = requests.post(api_url, data=json.dumps(profile_json, indent=4))
        resp = json.loads(r.text)
        if r.status_code != 200:
            raise TestFailure("CachedInventoryTest - FAIL: Profile POST failed. Reason: %s", resp["reason"])

        # Create 3 listings as alice
        for x in range(0, 3):
            api_url = alice["gateway_url"] + "ob/listing"
            r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
            resp = json.loads(r.text)
            if r.status_code != 200:
                raise TestFailure("CachedInventoryTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])

        # Create 1 listing as bob
        api_url = bob["gateway_url"] + "ob/listing"
        r = requests.post(api_url, data=json.dumps(listing_json, indent=4))
        resp = json.loads(r.text)
        if r.status_code != 200:
            raise TestFailure("CachedInventoryTest - FAIL: Listing POST failed. Reason: %s", resp["reason"])

        # Get profile
        api_url = alice["gateway_url"] + "ob/profile"
        r = requests.get(api_url)
        resp = json.loads(r.text)
        if r.status_code != 200:
            raise TestFailure("CachedInventoryTest - FAIL: Profile GET failed. Reason: %s", resp["reason"])
        alicePeerID = resp["peerID"]

        # Get own inventory
        api_url = bob["gateway_url"] + "ob/inventory"
        r = requests.get(api_url)
        resp = json.loads(r.text)
        if r.status_code != 200:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET failed. Reason: %s", resp["reason"])
        if len(resp) != 8:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET should return an object with details for every variant")
        for entry in resp:
            if entry["slug"] != "ron-swanson-tshirt":
                raise TestFailure("CachedInventoryTest - FAIL: Inventory GET should return an object with only 1 slug)")

        # Get alice's inventory as bob
        api_url = bob["gateway_url"] + "ob/inventory/" + alicePeerID
        r = requests.get(api_url)
        resp = json.loads(r.text)
        if r.status_code != 200:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET failed. Reason: %s", resp["reason"])
        if len(resp) != 3:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET did not return the correct number of items")
        if resp["ron-swanson-tshirt"]["inventory"] != 213:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET did not return correct count")
        if not self.assert_correct_time(resp["ron-swanson-tshirt"]["lastUpdated"]):
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET did not return a correct looking timestamp")

        # Get alice's inventory for specific slug as bob
        api_url = bob["gateway_url"] + "ob/inventory/" + alicePeerID + "/ron-swanson-tshirt"
        r = requests.get(api_url)
        resp = json.loads(r.text)
        if r.status_code != 200:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET failed. Reason: %s", resp["reason"])
        if resp["inventory"] != 213:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET did not return correct count")
        if not self.assert_correct_time(resp["lastUpdated"]):
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET did not return a correct looking timestamp")

        # Get a non existant slug from alice's inventory as bob
        api_url = bob["gateway_url"] + "ob/inventory/" + alicePeerID + "/ron-swanson-loves-the-government-tshirt"
        r = requests.get(api_url)
        resp = json.loads(r.text)
        if r.status_code != 500:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET non-existant slug did not return an error")
        if resp["success"]:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET non-existant slug did not return an error")
        if resp["reason"] != "Could not find slug in inventory":
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET non-existant slug failed with the wrong error")

        # Try to get a non existant peer's inventory
        api_url = bob["gateway_url"] + "ob/inventory/Qmf56jASQYk7ccmyUHQssemyQc3YmEqiSos6GubHM3UtNS"
        r = requests.get(api_url)
        resp = json.loads(r.text)
        if r.status_code != 500:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET non-existant peer did not return an error")
        if resp["success"]:
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET non-existant peer did not return an error")
        if resp["reason"] != "Could not resolve name.":
            raise TestFailure("CachedInventoryTest - FAIL: Inventory GET non-existant peer failed with the wrong error")

    def assert_correct_time(self, unixtime):
        return datetime.strptime(unixtime, "%Y-%m-%dT%H:%M:%SZ" ) > datetime(2018, 4, 20)

if __name__ == '__main__':
    print("Running CachedInventoryTest")
    CachedInventoryTest().main()
