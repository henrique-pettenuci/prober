# PROBER

This is a simple application to answer Kubernetes probes that could be used to teach about probes and their behavior

## Configurations

| Environment Variable  | Description                                          | Default Value |
|-----------------------|------------------------------------------------------|---------------|
| STARTUP_PROBE_DELAY   | Delay in seconds to startup probe return an answer   | 0             |
| READINESS_PROBE_DELAY | Delay in seconds to readiness probe return an answer | 0             |
| LIVENESS_PROBE_DELAY  | Delay in seconds to liveness probe return an answer  | 0             |

## API

| Path                 | METHOD | Description                                     |
|----------------------|--------|-------------------------------------------------|
| /startup             | GET    | Return 200 after delay defined on configuration |
| /readiness           | GET    | Return 200 after delay defined on configuration |
| /liveness            | GET    | Return 200 after delay defined on configuration |
| /config              | POST   | Update probes delay                             |
| /delay/:seconds      | GET    | Return 200 after X seconds of delay             |
| /graceDelay/:seconds | GET    | Return 200 after X seconds but handle shutdown  |

### Config endpoint
```bash
curl --request POST \
  --url http://localhost:8080/config \
  --header 'Content-Type: application/json' \
  --data '{ "startup": "1", "readiness": "2", "liveness": "2"}'
```

## Running

Set the expected delay for each probe on file `prober.yaml`.
In a terminal execute `kubectl apply -f ./prober.yaml`
