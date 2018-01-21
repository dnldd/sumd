### sumd
sumd is a reference implementation of a download server with data integrity verification built in. It takes metadata from politeia release records, locates the release file, verifies its authenticity and responds to the client with details of the data integrity check and a unique time bound download link if the release file is authentic.

The inputs, outputs, data formats and reasons for creating sumd are detailed in [sumd.md](sumd.md).


### sumdemo
sumdemo is a detailed example of how politeia software release records are created and used to verify the authenticity of release files before serving a download request.

sumdemo requires a running instance of politeia and sumd configured correctly. To run sumdemo follow the steps below.

build politeiad
```
  cd politeia/politeiad
  dep ensure
  go build
```

run politeiad on testnet, specifying your `rpcuser` and `rpcpass`
```
./politeiad --testnet --rpcuser=user  --rpcpass=pass
```

build sumd & sumdemo
```
  cd sumd
  dep ensure
  go build
  cd /sumdemo
  go build
```

run sumd
```
./sumd --reldir=rel --baseurl=http://127.0.0.1 --port=:55650
```

with both politeia and sumd running, start sumdemo specifying your `rpcuser` and `rpcpass`

for success case:
```
cd /sumdemo
./sumdemo --pi=https://127.0.0.1:59374 --sumd=http://127.0.0.1:55650 --rpcuser=user --rpcpass=pass
```

for failure case:
```
cd /sumdemo
./sumdemo --pi=https://127.0.0.1:59374 --sumd=http://127.0.0.1:55650 --rpcuser=user --rpcpass=pass --fail
```
