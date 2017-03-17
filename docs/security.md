Security
========================
The following is a list of security features that it's recommended you take to secure your node.

### Database Encryption

The openbazaar-go daemon stores all of its user data (keys, orders, sales, metadata, etc) in a sqlite database found in the `~/openbazaar2.0/datastore`
directory. This database can be encrypted using [sqlcipher](https://www.zetetic.net/sqlcipher/).

#### Encrypting the database
There are several ways to enable database encryption.

1. Running the openbazaar-go `start` or `init` commands the very first time using the `--password` flag (followed by your password) will encrypt the database with that password. Note:
the password will be visible in your terminal if you launch from the terminal.
2. Running the openbazaar-go `encryptdatabase` at any time will enable you to encrypt the database. (Unlike the previous option, the terminal input will be obfuscated.)

#### Running with an encrypted database
Again two options:

1. Either pass in your password using the `--password` flag. Or
2. Omit the password flag and you will be prompted to enter it in the terminal.

#### Decrypting the database

You can decrypt the database by running the `decryptdatabase` command. Note: this will return it to the unencrypted state on your disk.

### API Authentication

If you are running the openbazaar-go daemon on a remote machine you MUST enable API authentication otherwise anyone will be able to log into your
node, steal your bitcoins, and view your order/sales history. Additionally, you MUST enable SSL (see below) otherwise your authentication credentials
will be sent to the remote node in the clear (unencrypted) ― meaning they could be intercepted by anyone viewing your network traffic. 

The settings to enable authentication are found in the config file located in the `openbazaar2.0` data directory. To enable authentication first set the
JSON-API authentication boolean to true:
```
{
    "JSON-API": {
        "Authenticated": true
    }
}
```

Note: when you run a node on a remote computer you must modify the gateway address in the config. For example: `"Addresses": {"Gateway": "/ip4/0.0.0.0/tcp/4002"}`. If the address is
set to anything other than localhost, the daemon will enable authentication by default for security reasons.

There are two ways for a client to authenticate:

#### Authentication Cookie
The default is via an authentication cookie. On start up the server generates a random cookie and saves it in the data directory as a .cookie file. You need to add this cookie to the header of all requests. For example:
```
cookie: OpenBazaar_Auth_Cookie=2Yc7VZtG/pVKrH5Lp0mKRSEPC4xlm1dGpkbUXLehTUI
```

#### Basic Authentication
Alternatively, you can use basic authentication by setting a username and password in the config file:
```
{
    "JSON-API": {
        "Username": "Aladdin",
        "Password": "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032"
    }
}
```
The password must be saved as the hex-encoded SHA-256 hash of your password. The password you send to authenticate must be the hash preimage as it will hashed and compared to the hash in the config file.

You can have openbazaar-go hash and save the username and password for you by running the `setapicreds` command.

The username and password need to be included in the request header following [RFC 2617](https://www.ietf.org/rfc/rfc2617.txt) where the username and password are encoded as `base64encode(username + ":" + password)`:
```
Authorization: Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==
```
Or included in the request Url:
```
http://username:password@localhost:8080/ob/
```
### SSL
As mentioned above, NEVER allow outside internet access without both enabling authentication and SSL as your authentication creditials will be sent to the remote node unencrypted otherwise.
The instructions to set up SSL can be found in a separate [doc](https://github.com/OpenBazaar/openbazaar-go/blob/master/docs/ssl.md). 

### Restrict Access By IP
You can (and probably should) restrict access to the API to specific IP addresses. To do so you can either enter them in the config file:
```
{
    "JSON-API": {
        "AllowedIPs": ["69.89.31.226"],
    }
}
```
Or pass them in at start up: `openbazaar-go start -a 69.89.31.226`

If `AllowIPs` is set to `[]` in the config file and the `-a` flag is omitted at start up, then all IP addresses will be allowed.
