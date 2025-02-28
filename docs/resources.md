# torproxy resources

I'm designing a few resources which so far still do not exist in the codebase.

So far it exists only one kind of resource since I was writing it as a POC:
- `TorBridgeConfig`: this resource operates as a tor bridge.
This configuration allow outgoing pod's traffic to be hidden over the tor network redeployng the same Pod with a container added. This container will be the tor-bridge configuration that will manage the hide of the outgoing traffic injecting a tor bridge configuration into the pod's filesystem.

```yaml
apiVersion: tor.stack.io/v1beta1
kind: TorBridgeConfig
metadata:
  name: torbridgeconfig-sample
  labels:
    app.kubernetes.io/name: torproxy
    app.kubernetes.io/managed-by: kustomize
spec:
  image: dperson/torproxy
  orPort: 9001
  dirPort: 9030
  socksPort: 9050
  hiddenServiceDir: /var/lib/tor/hidden_service/
  hiddenServicePort: 80
  hiddenServiceTarget: 127.0.0.1:80    
```

To check the onion address after a hidden pod is created: 
```sh
kubectl exec -it nginx-pod-hidden-rtam -c tor-bridge -- cat /var/lib/tor/hidden_service/hostname
> xfsjyowyulww3wl3c5ibjozzpbm5tkfknsjbbym5h5eo2uhgmsnkkjad.onion
```

## Resources in design phase
- `TorRelayConfig`
- `TorNetworkConfig`
- `TorExitRelayConfig`
- `TorSnowflakeConfig`