import requests
import json
import time
import subprocess
import re

from test_framework.test_framework import OpenBazaarTestFramework, TestFailure, SMTP_DUMPFILE



class SMTPTest(OpenBazaarTestFramework):

    def __init__(self):
        super().__init__()
        self.num_nodes = 2

    def run_test(self):
        alice = self.nodes[0]
        bob = self.nodes[1]

        # configure SMTP notifications
        api_url = alice["gateway_url"] + "ob/settings"
        smtp = {
            "smtpSettings" : {
                "notifications": True,
                "serverAddress": "0.0.0.0:1025",
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
        time.sleep(1)

        # check SMTP settings
        api_url = alice["gateway_url"] + "ob/settings"
        r = requests.get(api_url)
        if r.status_code == 404:
            raise TestFailure("SMTPTest - FAIL: Settings GET endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SMTPTest - FAIL: Settings GET failed. Reason: %s", resp["reason"])
        

        # Bob pings Alice
        addr = "0.0.0.0:1025"
        class_name = "test_framework.test_framework.SMTPTestServer"
        proc = subprocess.Popen(["python", "-m", "smtpd", "-n", "-c", class_name, addr])
        message = {
            "subject": "Pinging you",
            "message": "Ping",
            "peerId": alice["peerId"]
        }
        api_url = bob["gateway_url"] + "ob/chat"
        r = requests.post(api_url, data=json.dumps(message, indent=4))
        if r.status_code == 404:
            raise TestFailure("SMTPTest - FAIL: Chat message POST endpoint not found")
        elif r.status_code != 200:
            resp = json.loads(r.text)
            raise TestFailure("SMTPTest - FAIL: Chat message POST failed. Reason: %s", resp["reason"])
        time.sleep(1)
        proc.terminate()

        # check notification
        expected = '''From: openbazaar@test.org
To: user.openbazaar@test.org
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8
Subject: [OpenBazaar] New chat message received

New chat message from "QmS5svqgGwFxwY9W5nXBUh1GJ7x8tqpkYfD4kB3MG7mPRv"

Time: <time>
Subject: Pinging you
Ping
'''
        expected_lines = [e for e in expected.splitlines() if not e.startswith('Time:')]
        with open(SMTP_DUMPFILE, 'r') as f:
            res_lines = [l.strip() for l in f.readlines() if not l.startswith('Time:')]
            if res_lines != expected_lines:
                raise TestFailure("SMTPTest - FAIL: Incorrect mail data received")

        print("SMTPTest - PASS")


def run_with_timeout(timeout, default, f, *args, **kwargs):
    if not timeout:
        return f(*args, **kwargs)
    try:
        timeout_timer = Timer(timeout, _thread.interrupt_main)
        timeout_timer.start()
        result = f(*args, **kwargs)
        return result
    except KeyboardInterrupt:
        return default
    finally:
        timeout_timer.cancel()


if __name__ == '__main__':
    print("Running SMTPTest")
    SMTPTest().main(["--regtest", "--disableexchangerates"])
