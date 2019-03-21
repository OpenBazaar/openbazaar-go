import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class PatchProfileTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 2

    def run_test(self):
        alice = self.nodes[1]
        api_url = alice["gateway_url"] + "ob/profile"
        not_found = TestFailure("PatchProfileTest - FAIL: Profile post endpoint not found")

        # create profile
        pro = {"name": "Alice", "nsfw": True, "about": "some stuff"}
        r = requests.post(api_url, data=json.dumps(pro, indent=4))
        if r.status_code == 404:
            raise not_found
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PatchProfileTest - FAIL: Profile POST failed. Reason: %s", resp["reason"])
        time.sleep(4)

        # patch profile
        pro_patch = {"nsfw": False, "about": "new stuff"}
        r = requests.patch(api_url, data=json.dumps(pro_patch, indent=4))
        if r.status_code == 404:
            raise not_found
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PatchProfileTest - FAIL: Profile PATCH failed. Reason: %s", resp["reason"])

        # check profile
        r = requests.get(api_url)
        if r.status_code == 404:
            raise not_found
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PatchProfileTest - FAIL: Profile GET failed. Reason: %s", resp["reason"])
        else:
            resp = json.loads(r.text)
            if resp["name"] != "Alice" or resp["nsfw"] != False or resp["about"] != "new stuff":
                raise TestFailure("PatchProfileTest - FAIL: Incorrect result of profile PATCH")

        print("PatchProfileTest - PASS")


if __name__ == '__main__':
    print("Running PatchProfileTest")
    PatchProfileTest().main(["--regtest", "--disableexchangerates"])
