# GCP-HCP Platform API (experimental)

This is the implementation of a experimental Platform API server, which uses code generation to implement a kubernetes API server with custom types and storages.

To play with the setup:

The folder `api/private` contains the source API definitions with a new "+orlop:public" marker.
Fields marked with this marker are moved to `api/public` when running `go run ./cmd/orlop-gen`.

To run the demo apiserver:
```sh
# Run the API server using a in-memory database
$ go run ./cmd/platform-api-server --private-port 8080 --public-port 8081

# you can interact with the API using kubectl:
$ export KUBECONFIG="$PWD/kubeconfig-public.yaml"
$ kubecl create -f test.mhc.yaml
```

There is also a simple scheduler controller within the project:
```sh
# Run the scheduler:
$ export KUBECONFIG="$PWD/kubeconfig.yaml"
$ go run ./cmd/mhc-scheduler --health-probe-bind-address=0 --metrics-bind-address=0
```