package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"kubectl-multi/pkg/cluster"
	"kubectl-multi/pkg/util"
)

// Custom help function for describe command
func describeHelpFunc(cmd *cobra.Command, args []string) {
	// Get original kubectl help
	kubectlHelp, err := util.GetKubectlHelp("describe")
	if err != nil {
		// Fallback to default help if kubectl help is not available
		cmd.Help()
		return
	}

	// Multi-cluster plugin information
	multiClusterInfo := `Show details of a specific resource or group of resources across all managed clusters.
This command displays detailed information about resources similar to kubectl describe,
but across all KubeStellar managed clusters.`

	// Multi-cluster examples
	multiClusterExamples := `# Describe a specific pod across all clusters
kubectl multi describe pod nginx

# Describe all pods with a specific label across all clusters
kubectl multi describe pods -l app=nginx

# Describe a service across all clusters
kubectl multi describe service/my-service

# Describe nodes across all clusters
kubectl multi describe nodes`

	// Multi-cluster usage
	multiClusterUsage := `kubectl multi describe [TYPE[.VERSION][.GROUP] [NAME_PREFIX | -l label] | TYPE[.VERSION][.GROUP]/NAME] [flags]`

	// Format combined help
	combinedHelp := util.FormatMultiClusterHelp(kubectlHelp, multiClusterInfo, multiClusterExamples, multiClusterUsage)
	fmt.Fprintln(cmd.OutOrStdout(), combinedHelp)
}

func newDescribeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe [TYPE[.VERSION][.GROUP] [NAME_PREFIX | -l label] | TYPE[.VERSION][.GROUP]/NAME]",
		Short: "Show details of a specific resource or group of resources across managed clusters",
		Long: `Show details of a specific resource or group of resources across all managed clusters.
This command displays detailed information about resources similar to kubectl describe,
but across all KubeStellar managed clusters.`,
		Example: `# Describe a specific pod across all clusters
kubectl multi describe pod nginx

# Describe all pods with a specific label across all clusters
kubectl multi describe pods -l app=nginx

# Describe a service across all clusters
kubectl multi describe service/my-service

# Describe nodes across all clusters
kubectl multi describe nodes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("resource type must be specified")
			}

			kubeconfig, remoteCtx, _, namespace, allNamespaces := GetGlobalFlags()
			return handleDescribeCommand(args, kubeconfig, remoteCtx, namespace, allNamespaces)
		},
	}

	// Set custom help function
	cmd.SetHelpFunc(describeHelpFunc)

	return cmd
}

func handleDescribeCommand(args []string, kubeconfig, remoteCtx, namespace string, allNamespaces bool) error {
	clusters, err := cluster.DiscoverClusters(kubeconfig, remoteCtx)
	if err != nil {
		return fmt.Errorf("failed to discover clusters: %v", err)
	}

	fmt.Printf("Describing %s across %d clusters...\n\n", args[0], len(clusters))

	for _, clusterInfo := range clusters {
		if clusterInfo.Client == nil {
			continue
		}

		fmt.Printf("=== Cluster: %s (Context: %s) ===\n", clusterInfo.Name, clusterInfo.Context)

		// TODO: Implement actual describe functionality
		// This would use kubectl's describe packages or implement similar functionality
		fmt.Printf("Describe functionality for cluster %s not yet implemented\n", clusterInfo.Name)

		fmt.Printf("\n")
	}

	return nil
}
