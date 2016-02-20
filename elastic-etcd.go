package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/sttts/elastic-etcd/cliext"
	"github.com/sttts/elastic-etcd/join"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/golang/glog"
)

type Result struct {
	join.Result
	DataDir string
}

func joinEnv(r *Result) map[string]string {
	return map[string]string{
		"ETCD_INITIAL_CLUSTER":            strings.Join(r.InitialCluster, ","),
		"ETCD_INITIAL_CLUSTER_STATE":      r.InitialClusterState,
		"ETCD_INITIAL_ADVERTISE_PEER_URL": r.AdvertisePeerUrls,
		"ETCD_DISCOVERY":                  r.Discovery,
		"ETCD_NAME":                       r.Name,
		"ETCD_DATA_DIR":                   r.DataDir,
	}
}

func printEnv(r *Result) {
	vars := joinEnv(r)
	for k, v := range vars {
		fmt.Printf("%s=\"%s\"\n", k, v)
	}
}

func printDropin(r *Result) {
	println("[service]")
	vars := joinEnv(r)
	for k, v := range vars {
		fmt.Printf("Environment=\"%s=%s\n", k, v)
	}
}

func printFlags(r *Result) {
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
	if r.AdvertisePeerUrls != "" {
		args = append(args, fmt.Sprintf("-initial-advertise-peer-urls=%s", r.AdvertisePeerUrls))
	}

	args = append(args, fmt.Sprintf("-name=%s", r.Name))
	args = append(args, fmt.Sprintf("-data-dir=%s", r.DataDir))

	params := strings.Join(args, " ")

	glog.V(4).Infof("Derived etcd parameter: %s", params)
	fmt.Fprintln(os.Stdout, params)
}

func main() {
	var (
		discoveryUrl             string
		joinStrategy             string
		format                   string
		name                     string
		clientPort               int
		initialAdvertisePeerUrls string
		dataDir                  string
	)

	var formats = []string{"env", "dropin", "flags"}
	var strategies = []join.Strategy{join.PreparedStrategy, join.ReplaceStrategy, join.PruneStrategy, join.AddStrategy}

	checkFlags := func() {
		if name == "" {
			fmt.Fprint(os.Stderr, "name must be set\n")
			os.Exit(1)
		}
		if initialAdvertisePeerUrls == "" {
			fmt.Fprint(os.Stderr, "initial-advertise-peer-urls must consist at least of one url\n")
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
	app.Name = "elastic-etcd"
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
				cli.StringFlag{
					Name:        "join-strategy",
					Usage:       "the strategy to join: dumb, replace, add",
					EnvVar:      "ETCD_JOIN_STRATEGY",
					Value:       string(join.ReplaceStrategy),
					Destination: &joinStrategy,
				},
				cli.StringFlag{
					Name:        "data-dir",
					Usage:       "the etcd data directory",
					EnvVar:      "ETCD_DATA_DIR",
					Value:       "",
					Destination: &dataDir,
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
					EnvVar:      "ETCD_NAME",
					Value:       "",
					Destination: &name,
				},
				cli.IntFlag{
					Name:        "client-port",
					Usage:       "the etcd client port of all peers",
					EnvVar:      "ETCD_CLIENT_PORT",
					Value:       2379,
					Destination: &clientPort,
				},
				cli.StringFlag{
					Name:        "initial-advertise-peer-urls",
					Usage:       "the advertised peer urls of this instance",
					EnvVar:      "ETCD_INITIAL_ADVERTISE_PEER_URLS",
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

				ok = false
				for _, s := range strategies {
					if s == join.Strategy(joinStrategy) {
						ok = true
						break
					}
				}
				if !ok {
					fmt.Fprintf(os.Stderr, "invalid join strategy %q\n", joinStrategy)
					os.Exit(1)
				}

				if dataDir == "" {
					dataDir = name + ".etcd"
				}
				fresh := !fileutil.Exist(dataDir)

				jr, err := join.Join(
					discoveryUrl,
					name,
					initialAdvertisePeerUrls,
					fresh,
					clientPort,
					join.Strategy(joinStrategy),
				)
				if err != nil {
					fmt.Fprintf(os.Stderr, "cluster join failed: %v\n", err)
					os.Exit(1)
				}
				r := &Result{*jr, dataDir}
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
