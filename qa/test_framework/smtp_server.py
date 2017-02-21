import smtpd

SMTP_DUMPFILE = 'mail.dump'


class SMTPTestServer(smtpd.SMTPServer):
    def process_message(self, peer, mailfrom, rcpttos, data, **kwargs):
        with open(SMTP_DUMPFILE, 'w') as f:
            f.write(data)
