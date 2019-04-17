Install and Run Multiple Servers
=========================

### Init the New Data Directory

In your server directory, use the datadir option to init the server with a new data directory. This will create the data directory if it doesn't already exist, and generate all the files needed to run the server from that directory. 

Example:
Windows: `go run openbazaard.go init -d=c://path/to/data/directory`
Linux/MacOS: `go run openbazaard.go init -d=/path/to/data/directory`

### Change the Ports in the Config File

In the new data directory, open the config file change the default 4001 and 9005 ports in the Addresses object, and save the file. You can change them to any unused, valid port number. 

Example:
```
"Addresses": {
      "API": "",
      "Gateway": "/ip4/127.0.0.1/tcp/4102",
      "Swarm": [
         "/ip4/0.0.0.0/tcp/4101",
         "/ip6/::/tcp/4101",
         "/ip4/0.0.0.0/tcp/9105/ws",
         "/ip6/::/tcp/9105/ws"
      ]
   },
   ```

### Run the Server

You can now run an instance of the server from the new data directory with the daradir option. Multiple instances can be run simultaneously, one for each data directory you've created.

Example:
Windows: `go run openbazaard.go start -d=c://path/to/data/directory`
Linux/MacOS: `go run openbazaard.go start -d=/path/to/data/directory`
