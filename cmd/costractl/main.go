package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "dev" // overridden at build time via -ldflags

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--version", "-v":
		fmt.Printf("costractl version %s\n", version)
	case "cluster":
		handleClusterCommand(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func handleClusterCommand(args []string) {
	if len(args) < 1 || args[0] != "connect" {
		fmt.Println("Usage: costractl cluster connect --api-token=<token> --organization-id=<id>")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	apiToken := fs.String("api-token", "", "API token from your Costra dashboard")
	orgID := fs.String("organization-id", "", "Your organization UUID")
	apiRegion := fs.String("api-region", "us", "API region")
	clusterOptimization := fs.Bool("cluster-optimization", false, "Enable cluster optimization (requires AWS IAM role)")
	workloadAutoscaler := fs.Bool("workload-autoscaler", false, "Enable workload autoscaler (requires write RBAC)")
	fs.Parse(args[1:])

	if *apiToken == "" || *orgID == "" {
		fmt.Println("Error: --api-token and --organization-id are required")
		os.Exit(1)
	}

	fmt.Printf("Connecting cluster to Costra (region: %s, org: %s)...\n", *apiRegion, *orgID)
	fmt.Println("Reading current kubectl context...")
	// TODO: use client-go's clientcmd to read the active context here,
	// confirm with the user, then apply the read-only Helm chart.

	if *clusterOptimization {
		fmt.Println("Cluster optimization requested — this requires a cross-account AWS IAM role.")
		fmt.Println("See: https://get.costraai.com/docs/cloud-permissions")
	}
	if *workloadAutoscaler {
		fmt.Println("Workload autoscaler requested — installing write-permission RBAC (separate ClusterRole).")
	}

	fmt.Println("Cost-monitoring agent install: not yet implemented in this stub.")
}

func printUsage() {
	fmt.Println("costractl — connect your infrastructure to Costra")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  costractl --version")
	fmt.Println("  costractl cluster connect --api-token=<token> --organization-id=<id>")
}
