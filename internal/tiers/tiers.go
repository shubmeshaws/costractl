package tiers

import "fmt"

// HandlePremiumFlags validates Tier 2/3 opt-ins. Tier 1 only is installed today.
func HandlePremiumFlags(clusterOptimization, workloadAutoscaler bool) error {
	if clusterOptimization {
		fmt.Println()
		fmt.Println("  ⚠ Tier 2 (Cluster optimization) requested but not yet available.")
		fmt.Println("    Requires a cross-account AWS IAM role — coming soon.")
		fmt.Println("    Only Tier 1 (read-only cost monitoring) will be installed.")
		fmt.Println()
	}
	if workloadAutoscaler {
		fmt.Println()
		fmt.Println("  ⚠ Tier 3 (Workload autoscaler) requested but not yet available.")
		fmt.Println("    Requires a separate write K8s RBAC — coming soon.")
		fmt.Println("    Only Tier 1 (read-only cost monitoring) will be installed.")
		fmt.Println()
	}
	return nil
}
