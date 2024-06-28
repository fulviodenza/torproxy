# High level resource definition:

- Bridge: This is used to help censored users to connect to the Tor network.
This is the most basic resource and we foresee to build it in such a fashion:
We will run obfs4proxy image in a pod container and automatically edit the configuration
inside the container to run the bridge:

The `/etc/tor/torrc` file will be configurable starting from a user-defined yaml:
```yaml
apiVersion: 
kind: TorBridgeConfig
metadata:
  name: "bridge"
  namespace: "default"
spec:
  ORPort: 9001 # not suggested as ORPort
  ServerTransportPlugin: "obfs4 exec /usr/bin/osfs4proxy"
  ServertransportListenAddr: "obfs4 0.0.0.0:TODO1" # replace TODO1 with a port 
  ExtOrPort: "auto"
  ContactInfo: "email@address.com"
  Nickname: "nickname" 
```

This resource will contain main info to make the node to run as a tor bridge.
The configuration setup will require a tor restart
More infos here: <a href="https://community.torproject.org/relay/setup/bridge/debian-ubuntu/">Tor Documentation about bridge setup</a>

