Running a Public Gateway
=========================
A public gateway is a server that allows others to view OpenBazaar user data (profiles, listings, etc) in a web browser.

Any node can perform this function with a few simple tweaks to the config file.

First, locate the `config` inside the OpenBazaar2.0 data folder.

Next, set the gateway address to:

```
"Addresses": {
    "Gateway": "/ip4/0.0.0.0/tcp/80",
}
```

(Note the use of port 80. You could use any port you want but if you use port 80 people may omit typing the port in the url.)

It is highly recommended you don't use your public gateway as your personal OpenBazaar node (for example to buy or sell things). So let's do a couple more things to lock it down.


Next turn off OpenBazaar API as you don't want to give public access to this:

```
"OB-API": {
    "Enabled": false,
}
````

Finally when you run the server use the `--disablewallet` option as you wont be needing it.

### Writable Gateway

In addition to allowing others to view OpenBazaar data on your gateway, you may allow them to post data to the network through your node.
When enabled, your node will seed any content that is posted to it. This is useful for allowing users to cross-post content to gateway nodes to ensure data persistence when they go offline.

To enable writing to your gateway set the writable flag in the config file:

```
"Gateway": {
    "Writable": true
}
```
