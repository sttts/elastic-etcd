# elastic-etcd

An experimental wrapper around etcd2 to add elastic discovery and join.

## Ratio

While etcd was born in the cloud era, it does not really play well in a dynamic environment where node come and go and where IP addresses are ephemeral. Moreover, etcd is meant – with its RAFT algorithm at the core – as a consistent key-value store. It rather refuses to form or join a cluster than putting concistency at risk.

As a consequence it is very conservative in implementing advanced cluster member management mechanisms – or even heuristics to make the operation of an etcd cluster more confortable.

The elastic-etcd binary experiments with those advanced member management heuristics. It is meant as a frontend to the etcd binary, applying these heuristics and then creating a matching command line for etcd itself.

## Usage

The elastic-etcd binary – in its current incarnation – is called with a subset of the etcd parameters. It does its jobs using the etcd [discovery service](https://coreos.com/os/docs/latest/cluster-discovery.html) and then prints out the matching etcd configuration. Depending on the context where elastic-etcd is used, it can print out either etcd flags, a systemd dropin or shell environment variables:

- `elastic-etcd -o flags ...` prints `-name=server1 -initial-cluster-state=new...`.
- `elastic-etcd -o dropin ...` prints
```
[service]
Environment="ETCD_NAME=server1"
Environment="ETCD_INITIAL_CLUSTER_STATE=new"
...
```

- `elastic-etcd -o env ...` prints
```bash
ETCD_NAME=server1
ETCD_INITIAL_CLUSTER_STATE=new
```
