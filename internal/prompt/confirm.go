package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConfirmCluster asks the user to confirm connecting the detected context.
func ConfirmCluster(contextName, clusterName, serverURL string) (bool, error) {
	fmt.Println()
	fmt.Println("  Costra will connect the following Kubernetes cluster:")
	fmt.Printf("    Context:  %s\n", contextName)
	fmt.Printf("    Name:     %s\n", clusterName)
	if serverURL != "" {
		fmt.Printf("    API:      %s\n", serverURL)
	}
	fmt.Println()
	fmt.Print("  Continue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}

// ConfirmDisconnect asks the user to confirm removing Costra from a cluster.
func ConfirmDisconnect(contextName, clusterName, serverURL string) (bool, error) {
	fmt.Println()
	fmt.Println("  Costra will disconnect and remove the collector agent from:")
	fmt.Printf("    Context:  %s\n", contextName)
	fmt.Printf("    Name:     %s\n", clusterName)
	if serverURL != "" {
		fmt.Printf("    API:      %s\n", serverURL)
	}
	fmt.Println()
	fmt.Println("  This removes the DaemonSet, credentials, and RBAC in namespace costra-agent.")
	fmt.Println()
	fmt.Print("  Continue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}
