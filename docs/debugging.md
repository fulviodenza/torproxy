# Debugging
Debug the controller is quite easy.

First, build and push to the registry a debug image of the application, this can be performed running the command
```sh
make docker-build-debug
make deploy-debug
```

In the cluster the port 40000 needs to be forwarded to make it reachable from vs code debugger.

This is the suggested `.vscode/launch.json`:
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Controller Manager",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "/workspace",
            "port": 40000,
            "host": "127.0.0.1",
            "showLog": true,
            "trace": "log",
            "logOutput": "rpc",
            "dlvLoadConfig": {
                "followPointers": true,
                "maxVariableRecurse": 1,
                "maxStringLen": 1024,
                "maxArrayValues": 64,
                "maxStructFields": -1
            },
        }
    ]
}
```