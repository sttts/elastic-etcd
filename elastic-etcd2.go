package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/sttts/elastic-etcd2/cliext"
	"github.com/sttts/elastic-etcd2/join"
)

func joinEnv(r *join.Result) map[string]string {
	return map[string]string{
		"ETCD_INITIAL_CLUSTER":            strings.Join(r.InitialCluster, ","),
		"ETCD_INITIAL_CLUSTER_STATE":      r.InitialClusterState,
		"ETCD_INITIAL_ADVERTISE_PEER_URL": r.AdvertisePeerUrls,
		"ETCD_DISCOVERY":                  r.Discovery,
		"ETCD_NAME":                       r.Name,
	}
}

func printEnv(r *join.Result) {
	vars := joinEnv(r)
	for k, v := range vars {
		fmt.Printf("%s=\"%s\"\n", k, v)
	}
}

func printDropin(r *join.Result) {
	println("[service]")
	vars := joinEnv(r)
	for k, v := range vars {
		fmt.Printf("Environment=\"%s=%s\n", k, v)
	}
}

func printFlags(r *join.Result) {
	args := []string{}
	if r.InitialClusterState != "" {
		args = append(args, fmt.Sprintf("-initial-cluster-state=%s", r.InitialClusterState))
	}
	if r.InitialCluster != nil {
		args = append(args, fmt.Sprintf("-initial-cluster=%s", strings.Join(r.InitialCluster, ",")))
	}
	if r.Discovery != "" {
		args = append(args, fmt.Sprintf("-discovery=%s", r.Discovery))
	}

	args = append(args, fmt.Sprintf("-initial-advertise-peer-urls=%s", r.AdvertisePeerUrls))
	args = append(args, fmt.Sprintf("-name=%s", r.Name))

	fmt.Fprintln(os.Stdout, strings.Join(args, " "))
}

func main() {
	var (
		discoveryUrl             string
		prune                    bool
		autoAdd                  bool
		format                   string
		name                     string
		clientPort               int
		initialAdvertisePeerUrls string
	)

	var formats = []string{"env", "dropin", "flags"}

	checkFlags := func() {
		if name == "" {
			fmt.Fprint(os.Stderr, "name must be set\n")
			os.Exit(1)
		}
		if discoveryUrl == "" {
			fmt.Fprint(os.Stderr, "discovery-url must be set\n")
			os.Exit(1)
		}

		discoveryUrl = strings.TrimRight(discoveryUrl, "/")

		u, err := url.Parse(discoveryUrl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid discovery url %q: %v\n", discoveryUrl, err)
			os.Exit(1)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			fmt.Fprint(os.Stderr, "discovery url must use http or https scheme\n")
			os.Exit(1)
		}
	}

	app := cli.NewApp()
	app.Name = "elastic-etcd2"
	app.Usage = "make etcd2 a good elastic cloud citizen"
	app.HideVersion = true
	app.Version = ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "discovery-url",
			Value:       "",
			Usage:       "a etcd discovery url",
			Destination: &discoveryUrl,
			EnvVar:      "ELASTIC_ETCD_DISCOVERY",
		},
	}
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if !strings.HasPrefix(f.Name, "test.") {
			app.Flags = append(app.Flags, cliext.FlagsFlag{f})
		}
	})
	app.Action = func(c *cli.Context) {
		checkFlags()
	}
	app.Commands = []cli.Command{
		{
			Name:  "join",
			Usage: "auto join a cluster, either during bootstrapping or later",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "prune",
					Usage:       "remove not responding members from cluster",
					EnvVar:      "ELASTIC_ETCD_PRUNE",
					Destination: &prune,
				},
				cli.BoolTFlag{
					Name:        "auto-add",
					Usage:       "add myself to an existing cluster",
					EnvVar:      "ELASTIC_ETCD_AUTO_ADD",
					Destination: &autoAdd,
				},
				cli.StringFlag{
					Name:        "o",
					Usage:       fmt.Sprintf("the output format out of: %s", strings.Join(formats, ", ")),
					Value:       "env",
					Destination: &format,
				},
				cli.StringFlag{
					Name:        "name",
					Usage:       "the cluster-unique node name",
					EnvVar:      "ELASTIC_ETCD_NAME",
					Value:       "",
					Destination: &name,
				},
				cli.IntFlag{
					Name:        "client-port",
					Usage:       "the etcd client port of all peers",
					EnvVar:      "ELASTIC_ETCD_CLIENT_PORT",
					Value:       2379,
					Destination: &clientPort,
				},
				cli.StringFlag{
					Name:        "initial-advertise-peer-urls=",
					Usage:       "the advertised peer urls of this instance",
					EnvVar:      "ELASTIC_ETCD_INITIAL_ADVERTISE_PEER_URLS",
					Value:       "http://localhost:2380",
					Destination: &initialAdvertisePeerUrls,
				},
			},
			Action: func(c *cli.Context) {
				checkFlags()

				ok := false
				for _, f := range formats {
					if f == format {
						ok = true
						break
					}
				}
				if !ok {
					fmt.Fprintf(os.Stderr, "invalid output format %q\n", format)
					os.Exit(1)
				}

				r, err := join.Join(discoveryUrl, name, initialAdvertisePeerUrls, clientPort)
				if err != nil {
					fmt.Fprintf(os.Stderr, "cluster join failed: %v\n", err)
					os.Exit(1)
				}
				switch format {
				case "flags":
					printFlags(r)
				case "env":
					printEnv(r)
				case "dropin":
					printDropin(r)
				}
			},
		},
	}

	app.Run(os.Args)
}
