SSL SETUP FOR JSON API
======================
This guide is for setting up SSL encryption on the openbazaar-go JSON API on Linux-based servers. If you plan on running openbazaar-go on a remote server you MUST use SSL otherwise your authentication information will be sent in the clear, allowing attackers to gain access to your server and steal your bitcoins and OpenBazaar identity (in addition to seeing your purchase/sales history). Follow these three steps exactly to enable SSL.

### Step 1: Generate SSL certificates

If you have an SSL certificate issued to you by a Certificate Authority, you can skip this step.

First, enter the OpenBazaar data directory.
```
cd .openbazaar2.0
```
Next enter the following commands to generate a self-signed server certificate. If running a remote server, on the fourth line, be sure to replace \<server-ip\> with the ip of your remote server.
```
openssl genrsa -out rootCA.key 4096
openssl req -x509 -new -nodes -key rootCA.key -days 1024 -out OpenBazaar.crt -subj "/C=DE/ST=Germany/L=Walldorf/O=SAP SE/OU=Tools/CN=OpenBazaar.crt"
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr -subj "/C=DE/ST=Germany/L=Walldorf/O=SAP SE/OU=Tools/CN=<server-ip>"
openssl x509 -req -in server.csr -CA OpenBazaar.crt -CAkey rootCA.key -CAcreateserial -out server.crt -days 1024
```

The above commands will generate three files that are of interest to us: `server.crt`, `server.key`, and `OpenBazaar.crt`.

### Step 2: Edit the config file

You need to edit the openbazaar-go config file (found in the data folder):
```
nano config
```
And make the following changes to the SSL parameters.
```
"JSON-API": {
    "SSL": true,
    "SSLCert": "/path/to/server.crt",
    "SSLKey": "/path/to/server.key",
  },
```
The SSLCert and SSLKey paramenters require the absolute paths to the server.crt and server.key files we generated above.

If you skipped Step 1 because you have your own SSL cert, then set the paths to your certficate and key files.

### Step 3: Install the CA cert in the operating system of your client's computer.

If you used your own SSL cert issued by a CA, you can skip this step as the OpenBazaar client should recognize it as a valid certificate.

If you followed Step 1 and generated a self-signed certificate you will need to install the `OpenBazaar.crt` in the operating system of the computer on which you plan to run the client. By default self-signed certificates are rejected, which is why you need to install this root certificate.

To download the `OpenBazaar.crt` from your remote server you can use any file transfer program such as `SFTP`.

Once `OpenBazaar.crt` is on your local computer you should just be able to double click it to install it.

From here you can run openbazaar-go as normal. In the client you will need to check `Use SSL` in the server configuration screen.

SSL should now be configured.
