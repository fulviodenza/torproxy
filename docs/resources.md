# torproxy resources

I'm designing a few resources which so far still do not exist in the codebase.

So far it exists only one kind of resource since I was writing it as a POC:
- `TorBridgeConfig`: this resource operates as a tor bridge.
This configuration allow outgoing pod's traffic to be hidden over the tor network redeployng the same Pod with a container added. This container will be the tor-bridge configuration that will manage the hide of the outgoing traffic injecting a tor bridge configuration into the pod's filesystem.
    ```
    apiVersion: tor.stack.io/v1beta1
    kind: TorBridgeConfig
    metadata:
    labels:
        app.kubernetes.io/name: torproxy
        app.kubernetes.io/managed-by: kustomize
    name: torbridgeconfig-sample
    spec:
    orPort: 9001
    dirPort: 9030
    socksPort: 9050
    serverTransportListenAddr: "obfs4 0.0.0.0:9050"
    extOrPort: auto
    image: dperson/torproxy
    nickname: torpod1
    contactInfo: noinfo@email.com
    ```

## Resources in design phase
- `TorRelayConfig`
- `TorNetworkConfig`
- `TorExitRelayConfig`
- `TorSnowflakeConfig`