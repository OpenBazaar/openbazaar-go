import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class PatchProfileTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 1

    def run_test(self):
        alice = self.nodes[0]
        api_url = alice["gateway_url"] + "ob/profile"
        not_found = TestFailure("PatchProfileTest - FAIL: Profile post endpoint not found")

        # create profile
        pro = {"name": "Alice", "nsfw": True, "email": "alice@example.com"}
        r = requests.post(api_url, data=json.dumps(pro, indent=4))
        if r.status_code == 404:
            raise not_found
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PatchProfileTest - FAIL: Profile POST failed. Reason: %s", r["reason"])
        time.sleep(4)

        # patch profile
        pro_patch = {"nsfw": False, "email": "alice777@example.com"}
        r = requests.patch(api_url, data=json.dumps(pro_patch, indent=4))
        if r.status_code == 404:
            raise not_found
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PatchProfileTest - FAIL: Profile PATCH failed. Reason: %s", r["reason"])

        # check profile
        r = requests.get(api_url)
        if r.status_code == 404:
            raise not_found
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("PatchProfileTest - FAIL: Profile GET failed. Reason: %s", r["reason"])
        else:
            resp = json.loads(r.text)
            if resp["name"] != "Alice" or resp["nsfw"] != False or resp["email"] != "alice777@example.com":
                raise TestFailure("PatchProfileTest - FAIL: Incorrect result of profile PATCH")

        print("PatchProfileTest - PASS")


if __name__ == '__main__':
    print("Running PatchProfileTest")
    PatchProfileTest().main(["--regtest", "--disableexchangerates"])
