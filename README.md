
# DT-DAG

This is an implementation of the DT-DAG to show the feasibility of the protocol.

## Installation

To run this, Go needs to be installed on the user's machine.

This program makes use of the [Kubo](https://github.com/ipfs/kubo/tree/master) and [Miracl](https://github.com/miracl/core/tree/master) libraries to run.
It may be necessary to clone the respective repositories to local storage.

This program has on option to use a local IPFS node as a supernode. This can be done by running an IPFS node locally, either via a process on terminal, or through the [IPFS Desktop application](https://docs.ipfs.tech/install/ipfs-desktop/).

MPI or TMUX can be used for parallel execution of processes, and MPI is executed by default in `run.sh`.
Therefore, if you want to perform parallel verification, please install MPI and make sure that `mpirun` is ready to run.


## Running the program

To start the program, copy the following into a terminal:
```
go run main.go
```

Alternatively, for the testing of multiple processes, a user can make use of the included shell program to run multiple nodes at the same time.
The amount of nodes is listed in the script, which can be modified. 
To run the script, paste the following into a terminal: 
```
./run.sh
```

The program can be stopped through the keybind CRTL + C.

## Config and params

### IPFS
If you are running IPFS Desktop locally, you can write its address to the `localipfs` file so that it will be automatically referenced when the program is run.

```
/ip4/127.0.0.1/udp/4001/quic-v1/p2p/12D3KooWGEtQggFuw1nUm9KYrk7Nf5B3i1jGGofLnAoC6fmMfvXR
```

### hostfile for MPI

It is possible to specify the host file to run with mpirun. The following example runs in the same environment, but it is also possible to run on multiple machines by specifying the IP address.
```
localhost slots=100
```
### Arguments of main.go

- mesh = true or false. If true, use a mesh network. In other words, instead of connecting to an IPFS server, connect directly between processes. If you don't go through an IPFS server, CPU usage will increase to share information about direct connections.

## Reading the logs

Currently, the program will output logs into the log folder. A user can confirm that the program is properly functioning by checking the following in the logs:

- The program continues to generate Aggregate logs with the following format:
{"OwnerID": "","Message":"","CurrentVersion":1,"PreviousCID":"","AggregateBytes":""}{"Keys":[""],"Filehistory":""}
- In the main log for each individual node, the following are met
verify Aggregation sig: true
signature verified : 1 unverified : 0

By default, five parallel executions are performed, so check the following to confirm that each one is working correctly.

- `targets received at 574605297: 5`
  - this means it received 5 signature targets
- `signature verified : 5 unverified : 0`
  - This means that the five signatures were successfully verified for aggregation.
- `Sig Aggregation finished`
  - This means that the signature aggregation was successful.
- `verify Aggregation sig: true`
  - This means that the aggregated signature passed verification.