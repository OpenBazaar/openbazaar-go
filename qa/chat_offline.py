import requests
import json
import time
from collections import OrderedDict
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class ChatOfflineTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 3

    def setup_network(self):
        self.setup_nodes()

    def run_test(self):
        alice = self.nodes[1]
        bob = self.nodes[2]

        alice_id = alice["peerId"]
        bob_id = bob["peerId"]

        # shutdown bob
        api_url = bob["gateway_url"] + "ob/shutdown"
        requests.post(api_url, data="")
        time.sleep(30)

        # alice send message
        message = {
            "subject": "",
            "message": "You have the stuff??",
            "peerId": bob_id
        }
        api_url = alice["gateway_url"] + "ob/chat"
        r = requests.post(api_url, data=json.dumps(message, indent=4))
        if r.status_code == 404:
            raise TestFailure("ChatOfflineTest - FAIL: Chat message post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatOfflineTest - FAIL: Chat message POST failed. Reason: %s", resp["reason"])

        # check alice saved message correctly
        api_url = alice["gateway_url"] + "ob/chatmessages/" + bob_id
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatOfflineTest - FAIL: Chat messages GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatOfflineTest - FAIL: Chat messages GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatOfflineTest - FAIL: Did not record outgoing message")

        # check alice saved conversation correctly
        api_url = alice["gateway_url"] + "ob/chatconversations/"
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatOfflineTest - FAIL: Chat messages GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatOfflineTest - FAIL: Chat messages GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatOfflineTest - FAIL: Did not record outgoing message")
        if resp[0]["peerId"] != bob_id:
            raise TestFailure("ChatOfflineTest - FAIL: Did not record new conversation")

        # startup bob again
        self.start_node(bob)
        time.sleep(45)

        # check bob saved message correctly
        api_url = bob["gateway_url"] + "ob/chatmessages/" + alice_id
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatOfflineTest - FAIL: Chat messages GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatOfflineTest - FAIL: Chat messages GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatOfflineTest - FAIL: Did not record outgoing message")

        # check bob saved conversation correctly
        api_url = bob["gateway_url"] + "ob/chatconversations/"
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatOfflineTest - FAIL: Chat messages GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatOfflineTest - FAIL: Chat messages GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatOfflineTest - FAIL: Did not record outgoing message")
        if resp[0]["peerId"] != alice_id:
            raise TestFailure("ChatOfflineTest - FAIL: Did not record new conversation")

        # bob mark as read
        api_url = bob["gateway_url"] + "ob/markchatasread/" + alice_id
        r = requests.post(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatOfflineTest - FAIL: Chat markasread post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatOfflineTest - FAIL: Chat markasread POST failed. Reason: %s", resp["reason"])

        # check bob marked as read correctly
        api_url = bob["gateway_url"] + "ob/chatconversations/" + alice_id
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatOfflineTest - FAIL: Chat conversations GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatOfflineTest - FAIL: Chat conversations GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatOfflineTest - FAIL: Did not record outgoing message")
        if resp[0]["unread"] != 0:
            raise TestFailure("ChatOfflineTest - FAIL: Did not mark message as read")

        print("ChatOfflineTest - PASS")

if __name__ == '__main__':
    print("Running ChatOfflineTest")
    ChatOfflineTest().main(["--regtest", "--disableexchangerates"])