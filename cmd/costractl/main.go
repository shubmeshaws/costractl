package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/shubmeshaws/costractl/internal/api"
	"github.com/shubmeshaws/costractl/internal/deploy"
	"github.com/shubmeshaws/costractl/internal/prompt"
	"github.com/shubmeshaws/costractl/internal/region"
	"github.com/shubmeshaws/costractl/internal/tiers"
)

var version = "dev" // overridden at build time via -ldflags

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "cluster":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		switch os.Args[2] {
		case "connect":
			if err := runConnect(os.Args[3:]); err != nil {
				fmt.Fprintf(os.Stderr, "\n✗ %v\n", err)
				os.Exit(1)
			}
		case "disconnect":
			if err := runDisconnect(os.Args[3:]); err != nil {
				fmt.Fprintf(os.Stderr, "\n✗ %v\n", err)
				os.Exit(1)
			}
		default:
			printUsage()
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Printf("costractl version %s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`costractl — connect Kubernetes clusters to Costra

Usage:
  costractl cluster connect \
    --api-token=<costra_v1_...> \
    --organization-id=<uuid> \
    [--api-region=us] \
    [--api-url=<https://host>] \
    [--context=<kube-context>] \
    [--cluster-name=<name>] \
    [--cost-monitoring=true] \
    [--cluster-optimization=false] \
    [--workload-autoscaler=false] \
    [--yes]

  costractl cluster disconnect \
    --api-token=<costra_v1_...> \
    --organization-id=<uuid> \
    [--api-region=us] \
    [--api-url=<https://host>] \
    [--context=<kube-context>] \
    [--cluster-name=<name>] \
    [--delete-namespace] \
    [--yes]

Install:
  curl -fsSL https://get.costraai.com/macos | bash
  curl -fsSL https://get.costraai.com/linux | bash

Run where kubectl context points at the cluster you want to connect.`)
}

func runConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	apiToken := fs.String("api-token", "", "Costra API token (costra_v1_...)")
	apiURL := fs.String("api-url", "", "API base URL (overrides --api-region)")
	apiRegion := fs.String("api-region", "us", "API region: us, eu")
	orgID := fs.String("organization-id", "", "Organization UUID")
	kubeContext := fs.String("context", "", "Kubeconfig context (default: current)")
	clusterName := fs.String("cluster-name", "", "Display name (default: context name)")
	costMonitoring := fs.Bool("cost-monitoring", true, "Tier 1: read-only cost monitoring (always recommended)")
	clusterOptimization := fs.Bool("cluster-optimization", false, "Tier 2: cluster optimization (requires AWS IAM role)")
	workloadAutoscaler := fs.Bool("workload-autoscaler", false, "Tier 3: workload autoscaler (requires write K8s RBAC)")
	skipConfirm := fs.Bool("yes", false, "Skip cluster confirmation prompt")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *apiToken == "" || *orgID == "" {
		return fmt.Errorf("--api-token and --organization-id are required")
	}

	baseURL := region.Resolve(*apiURL, *apiRegion)
	client := api.New(baseURL, *apiToken, *orgID)

	fmt.Println("→ Validating API token...")
	validation, err := client.ValidateToken(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("  ✓ Token valid for organization: %s\n", validation.OrganizationName)

	if err := tiers.HandlePremiumFlags(*clusterOptimization, *workloadAutoscaler); err != nil {
		return err
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if *kubeContext != "" {
		configOverrides.CurrentContext = *kubeContext
	}

	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).RawConfig()
	if err != nil {
		return fmt.Errorf("read kubeconfig: %w", err)
	}

	ctxName := *kubeContext
	if ctxName == "" {
		ctxName = rawConfig.CurrentContext
	}

	name := *clusterName
	if name == "" {
		name = ctxName
	}

	serverURL := ""
	if ctx, ok := rawConfig.Contexts[ctxName]; ok && ctx != nil {
		if cluster, ok := rawConfig.Clusters[ctx.Cluster]; ok && cluster != nil {
			serverURL = cluster.Server
		}
	}

	if !*skipConfirm {
		ok, err := prompt.ConfirmCluster(ctxName, name, serverURL)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("  Aborted.")
			return nil
		}
	}

	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	if err != nil {
		return fmt.Errorf("load kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	fmt.Printf("\n→ Connecting cluster \"%s\" (context: %s)\n", name, ctxName)

	bundle, err := client.Connect(context.Background(), api.ConnectOptions{
		ClusterName:         name,
		KubeContext:         ctxName,
		CostMonitoring:      *costMonitoring,
		ClusterOptimization: *clusterOptimization,
		WorkloadAutoscaler:  *workloadAutoscaler,
	})
	if err != nil {
		return err
	}

	fmt.Println("→ Installing Tier 1 read-only collector agent (DaemonSet on every node, namespace: costra-agent)...")
	if err := deploy.Install(context.Background(), clientset, deploy.Config{
		Namespace:   bundle.Namespace,
		ClusterID:   bundle.ClusterID,
		ClusterName: bundle.ClusterName,
		AgentToken:  bundle.AgentToken,
		IngestURL:   bundle.IngestURL,
		AgentImage:  bundle.AgentImage,
	}); err != nil {
		return fmt.Errorf("deploy agent: %w", err)
	}

	fmt.Println("→ Waiting for agent DaemonSet pods to become ready...")
	ready := waitForDaemonSet(context.Background(), clientset, bundle.Namespace, "costra-collector", 3*time.Minute)

	printSummary(bundle, validation.PermissionsDocURL, ready)
	return nil
}

func runDisconnect(args []string) error {
	fs := flag.NewFlagSet("disconnect", flag.ExitOnError)
	apiToken := fs.String("api-token", "", "Costra API token (costra_v1_...)")
	apiURL := fs.String("api-url", "", "API base URL (overrides --api-region)")
	apiRegion := fs.String("api-region", "us", "API region: us, eu")
	orgID := fs.String("organization-id", "", "Organization UUID")
	kubeContext := fs.String("context", "", "Kubeconfig context (default: current)")
	clusterName := fs.String("cluster-name", "", "Cluster name in Costra (default: context name)")
	deleteNamespace := fs.Bool("delete-namespace", false, "Delete the costra-agent namespace entirely")
	skipConfirm := fs.Bool("yes", false, "Skip confirmation prompt")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *apiToken == "" || *orgID == "" {
		return fmt.Errorf("--api-token and --organization-id are required")
	}

	baseURL := region.Resolve(*apiURL, *apiRegion)
	client := api.New(baseURL, *apiToken, *orgID)

	fmt.Println("→ Validating API token...")
	validation, err := client.ValidateToken(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("  ✓ Token valid for organization: %s\n", validation.OrganizationName)

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if *kubeContext != "" {
		configOverrides.CurrentContext = *kubeContext
	}

	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).RawConfig()
	if err != nil {
		return fmt.Errorf("read kubeconfig: %w", err)
	}

	ctxName := *kubeContext
	if ctxName == "" {
		ctxName = rawConfig.CurrentContext
	}

	name := *clusterName
	if name == "" {
		name = ctxName
	}

	serverURL := ""
	if ctx, ok := rawConfig.Contexts[ctxName]; ok && ctx != nil {
		if cluster, ok := rawConfig.Clusters[ctx.Cluster]; ok && cluster != nil {
			serverURL = cluster.Server
		}
	}

	if !*skipConfirm {
		ok, err := prompt.ConfirmDisconnect(ctxName, name, serverURL)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("  Aborted.")
			return nil
		}
	}

	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	if err != nil {
		return fmt.Errorf("load kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	fmt.Printf("\n→ Disconnecting cluster \"%s\" (context: %s)\n", name, ctxName)

	var apiErr error
	result, apiErr := client.Disconnect(context.Background(), name)
	if apiErr != nil {
		fmt.Printf("  ⚠ Costra API: %v\n", apiErr)
	} else {
		fmt.Printf("  ✓ Cluster registration revoked in Costra (id: %s)\n", result.ClusterID)
	}

	namespace := "costra-agent"
	if result != nil && result.Namespace != "" {
		namespace = result.Namespace
	}

	fmt.Println("→ Removing collector agent from Kubernetes...")
	if err := deploy.Uninstall(context.Background(), clientset, deploy.UninstallOptions{
		Namespace:       namespace,
		DeleteNamespace: *deleteNamespace,
	}); err != nil {
		return fmt.Errorf("remove agent: %w", err)
	}
	fmt.Println("  ✓ DaemonSet, credentials, and RBAC removed")

	if *deleteNamespace {
		fmt.Printf("  ✓ Namespace %s deleted\n", namespace)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Disconnect summary")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Cluster name: %s\n", name)
	if apiErr == nil {
		fmt.Println("  Costra:       Registration revoked")
	} else {
		fmt.Println("  Costra:       API revoke failed — check cluster name or run again")
	}
	fmt.Println("  Kubernetes:   Collector agent removed")
	fmt.Println("═══════════════════════════════════════════════════════")

	if apiErr != nil {
		return apiErr
	}
	return nil
}

func printSummary(bundle *api.ConnectBundle, permissionsURL string, ready bool) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Connection summary")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Cluster ID:   %s\n", bundle.ClusterID)
	fmt.Printf("  Cluster name: %s\n", bundle.ClusterName)
	fmt.Println()
	fmt.Println("  Installed:")
	fmt.Println("    ✓ Tier 1 — Cost monitoring (read-only K8s RBAC)")
	fmt.Println("      Namespace: costra-agent")
	fmt.Println("      Model: DaemonSet on every node")
	fmt.Println("      Permissions: get, list, watch on nodes, pods, workloads")
	if len(bundle.TiersPending) > 0 {
		fmt.Println()
		fmt.Println("  Requested (pending — not installed yet):")
		for _, t := range bundle.TiersPending {
			fmt.Printf("    ○ %s\n", t)
		}
	}
	fmt.Println()
	if ready {
		fmt.Println("  Status: Agents are running on all nodes. Metrics appear in ~5 minutes.")
	} else {
		fmt.Println("  Status: DaemonSet deployed — check: kubectl get daemonset -n costra-agent")
	}
	if permissionsURL != "" {
		fmt.Println()
		fmt.Printf("  Cloud permissions details: %s\n", permissionsURL)
	}
	fmt.Println("═══════════════════════════════════════════════════════")
}

func waitForDaemonSet(ctx context.Context, c kubernetes.Interface, ns, name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ds, err := c.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil && ds.Status.DesiredNumberScheduled > 0 && ds.Status.NumberReady >= ds.Status.DesiredNumberScheduled {
			return true
		}
		time.Sleep(5 * time.Second)
	}
	return false
}
