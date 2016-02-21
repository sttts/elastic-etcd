# elastic-etcd

An experimental wrapper around etcd2 to add elastic discovery and join.

## Ratio

While etcd was born in the cloud era, it does not really play well in a dynamic environment where node come and go and where IP addresses are ephemeral. Moreover, etcd is meant – with its RAFT algorithm at the core – as a consistent key-value store. It rather refuses to form or join a cluster than putting concistency at risk.

As a consequence it is very conservative in implementing advanced cluster member management mechanisms – or even heuristics to make the operation of an etcd cluster more confortable.

The elastic-etcd binary experiments with those advanced member management heuristics. It is meant as a frontend to the etcd binary, applying these heuristics and then creating a matching command line for etcd itself.

## Usage

The elastic-etcd binary – in its current incarnation – is called with a subset of the etcd parameters. It does its jobs using the etcd [discovery service](https://coreos.com/os/docs/latest/cluster-discovery.html) and then prints out the matching etcd configuration.

## Output Format

Depending on the context where elastic-etcd is used, it can print out either etcd flags, a systemd dropin or shell environment variables:

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

## Command Line Help

```
NAME:
   elastic-etcd - auto join a cluster, either during bootstrapping or later

USAGE:
   elastic-etcd [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   -o "env"                   the output format out of: env, dropin, flags
   --join-strategy "replace"  the strategy to join: dumb, replace, add
                              [$ETCD_JOIN_STRATEGY]
   --client-port "2379"       the etcd client port of all peers [$ETCD_CLIENT_PORT]
   --cluster-size "-1"        the maximum etcd cluster size, default: size value of
                              discovery url, 0 for infinit [$ETCD_CLUSTER_SIZE]

   --discovery-url            a etcd discovery url [$ELASTIC_ETCD_DISCOVERY]
   --data-dir                 the etcd data directory [$ETCD_DATA_DIR]
   --name                     the cluster-unique node name [$ETCD_NAME]
   --initial-advertise-peer-urls "http://localhost:2380"  the advertised peer urls
                              of this instance [$ETCD_INITIAL_ADVERTISE_PEER_URLS]

   --alsologtostderr=false    log to standard error as well as files
   --log_backtrace_at=:0      when logging hits line file:N, emit a stack trace
   --log_dir=                 If non-empty, write log files in this directory
   --logtostderr=false        log to standard error instead of files
   --stderrthreshold=2        logs at or above this threshold go to stderr
   --v=0                      log level for V logs
   --vmodule=                 comma-separated list of pattern=N settings for
                              file-filtered logging
   --help, -h                 show help
```
