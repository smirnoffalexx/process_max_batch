# process_max_batch
There is a client and mock server for processing batch items

For 
First of all you have to start a mock server:
```shell
cd mock_server
go build
./server
```

In a separate terminal run client:
```shell
cd client
go build
./client
```

For testing client run in a client folder:
```shell
go test
```

If you don't have preinstalled golangci-lint linter, you can run in client folder (this command installs linter and runs it):
```shell
make lint
```
