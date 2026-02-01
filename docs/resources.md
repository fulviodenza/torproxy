# torproxy resources

I'm designing a few resources which so far still do not exist in the codebase.

So far it exists only one kind of resource since I was writing it as a POC:
- `OnionService`: this resource operates as a tor onion service.
This configuration allow deployments in a kubernetes cluster to be pointed from a hidden service.
```yaml
apiVersion: tor.stack.io/v1beta1
kind: OnionService
metadata:
  name: web-app-onion
  namespace: default
spec:
  socksPort: 9050
  hiddenServicePort: 80
  hiddenServiceTarget: "web-app-svc:80"
```

To make this resource work, we need have deployed in the cluster a deployment like this

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
    spec:
      containers:
      - name: web-app
        image: nginx:latest
        ports:
        - containerPort: 80
```

## Resources in design phase
- `TorRelayConfig`
- `TorNetworkConfig`
- `TorExitRelayConfig`
- `TorSnowflakeConfig`