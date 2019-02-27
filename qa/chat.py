import requests
import json
from test_framework.test_framework import OpenBazaarTestFramework, TestFailure


class ChatTest(OpenBazaarTestFramework):

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

        # alice send message
        message = {
            "subject": "",
            "message": "You have the stuff??",
            "peerId": bob_id
        }
        api_url = alice["gateway_url"] + "ob/chat"
        r = requests.post(api_url, data=json.dumps(message, indent=4))
        if r.status_code == 404:
            raise TestFailure("ChatTest - FAIL: Chat message post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatTest - FAIL: Chat message POST failed. Reason: %s", resp["reason"])

        # check alice saved message correctly
        api_url = alice["gateway_url"] + "ob/chatmessages/" + bob_id
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatTest - FAIL: Chat messages GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatTest - FAIL: Chat messages GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatTest - FAIL: Did not record outgoing message")

        # check alice saved conversation correctly
        api_url = alice["gateway_url"] + "ob/chatconversations/"
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatTest - FAIL: Chat messages GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatTest - FAIL: Chat messages GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatTest - FAIL: Did not record outgoing message")
        if resp[0]["peerId"] != bob_id:
            raise TestFailure("ChatTest - FAIL: Did not record new conversation")

        # check bob saved message correctly
        api_url = bob["gateway_url"] + "ob/chatmessages/" + alice_id
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatTest - FAIL: Chat messages GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatTest - FAIL: Chat messages GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatTest - FAIL: Did not record outgoing message")

        # check bob saved conversation correctly
        api_url = bob["gateway_url"] + "ob/chatconversations/"
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatTest - FAIL: Chat messages GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatTest - FAIL: Chat messages GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatTest - FAIL: Did not record outgoing message")
        if resp[0]["peerId"] != alice_id:
            raise TestFailure("ChatTest - FAIL: Did not record new conversation")

        # bob mark as read
        api_url = bob["gateway_url"] + "ob/markchatasread/" + alice_id
        r = requests.post(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatTest - FAIL: Chat markasread post endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatTest - FAIL: Chat markasread POST failed. Reason: %s", resp["reason"])

        # check bob marked as read correctly
        api_url = bob["gateway_url"] + "ob/chatconversations/" + bob_id
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("ChatTest - FAIL: Chat conversations GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("ChatTest - FAIL: Chat conversations GET failed. Reason: %s", resp["reason"])
        resp = json.loads(r.text)
        if len(resp) != 1:
            raise TestFailure("ChatTest - FAIL: Did not record outgoing message")
        if resp[0]["unread"] != 0:
            raise TestFailure("ChatTest - FAIL: Did not mark message as read")

        print("ChatTest - PASS")

if __name__ == '__main__':
    print("Running ChatTest")
    ChatTest().main(["--regtest", "--disableexchangerates"])
