package main

import (
	"fmt"
	"os"
	"strings"

	elastic "github.com/sttts/elastic-etcd/pkg/elastic-etcd"
)

func joinEnv(r *elastic.EtcdConfig) map[string]string {
	return map[string]string{
		"ETCD_INITIAL_CLUSTER":             strings.Join(r.InitialCluster, ","),
		"ETCD_INITIAL_CLUSTER_STATE":       r.InitialClusterState,
		"ETCD_INITIAL_ADVERTISE_PEER_URLS": r.AdvertisePeerURLs,
		"ETCD_DISCOVERY":                   r.Discovery,
		"ETCD_NAME":                        r.Name,
		"ETCD_DATA_DIR":                    r.DataDir,
	}
}

func printFlags(r *elastic.EtcdConfig) {
	params := strings.Join(r.Flags(), " ")
	fmt.Fprintln(os.Stdout, params)
}

func printEnv(r *elastic.EtcdConfig) {
	vars := joinEnv(r)
	for k, v := range vars {
		fmt.Printf("%s=\"%s\"\n", k, v)
	}
}

func printDropin(r *elastic.EtcdConfig) {
	fmt.Print(`[Unit]
After=elastic-etcd.service
Requires=elastic-etcd.service

[Service]
`)
	vars := joinEnv(r)
	for k, v := range vars {
		fmt.Printf("Environment=\"%s=%s\"\n", k, v)
	}
}

func main() {
	r, format, err := elastic.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if r == nil {
		os.Exit(0)
	}

	switch format {
	case "flags":
		printFlags(r)
	case "env":
		printEnv(r)
	case "dropin":
		printDropin(r)
	}
}
