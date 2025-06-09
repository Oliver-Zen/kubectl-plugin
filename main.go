package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "text/tabwriter"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
)

// tableFmt: CONTEXT, CLUSTER, NAME, STATUS, ROLES, AGE, VERSION (tab‑separated)
const tableFmt = "%s\t%s\t%s\t%s\t%s\t%s\t%s\n"

func main() {
    remoteCtx := flag.String("remote-context", "its1", "remote hosting context (for ManagedCluster)")
    kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig (defaults to $HOME/.kube/config)")
    flag.Parse()

    tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

    // ---------- local (current) cluster ----------
    ctxName, clusterName, localClient := buildClient(*kubeconfig, "")
    printHeader(tw)
    listNodes(tw, ctxName, clusterName, localClient)

    // ---------- managed (remote) clusters ----------
    if *remoteCtx != "" {
        dyn := buildDynamicClient(*kubeconfig, *remoteCtx)
        listManagedClusters(tw, dyn, *kubeconfig, *remoteCtx)
    }

    tw.Flush()
}

// buildClient returns (contextName, clusterName, clientset).
func buildClient(kcfg, ctxOverride string) (string, string, *kubernetes.Clientset) {
    loading := clientcmd.NewDefaultClientConfigLoadingRules()
    if kcfg != "" {
        loading.ExplicitPath = kcfg
    }
    overrides := &clientcmd.ConfigOverrides{}
    if ctxOverride != "" {
        overrides.CurrentContext = ctxOverride
    }

    cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, overrides)
    rawCfg, err := cfg.RawConfig()
    exitIf(err)

    restCfg, err := cfg.ClientConfig()
    exitIf(err)

    cs, err := kubernetes.NewForConfig(restCfg)
    exitIf(err)

    ctxName := rawCfg.CurrentContext
    clusterName := "<unknown>"
    if ctx, ok := rawCfg.Contexts[ctxName]; ok {
        clusterName = ctx.Cluster
    }
    return ctxName, clusterName, cs
}

func buildDynamicClient(kcfg, ctxOverride string) dynamic.Interface {
    loading := clientcmd.NewDefaultClientConfigLoadingRules()
    if kcfg != "" {
        loading.ExplicitPath = kcfg
    }
    overrides := &clientcmd.ConfigOverrides{CurrentContext: ctxOverride}
    restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, overrides).ClientConfig()
    exitIf(err)

    dyn, err := dynamic.NewForConfig(restCfg)
    exitIf(err)
    return dyn
}

// listNodes prints node rows using provided CONTEXT and CLUSTER labels.
func listNodes(tw *tabwriter.Writer, contextName, clusterName string, cs *kubernetes.Clientset) {
    nodes, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        fmt.Fprintf(os.Stderr, "warning: cannot list nodes for context %s: %v\n", contextName, err)
        return
    }

    for _, n := range nodes.Items {
        status := "Unknown"
        for _, c := range n.Status.Conditions {
            if c.Type == "Ready" {
                if c.Status == "True" {
                    status = "Ready"
                } else {
                    status = "NotReady"
                }
                break
            }
        }

        role := "<none>"
        for k := range n.Labels {
            const prefix = "node-role.kubernetes.io/"
            if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
                role = k[len(prefix):]
                break
            }
        }

        age := humanAge(n.CreationTimestamp.Time)
        version := n.Status.NodeInfo.KubeletVersion

        printRow(tw, contextName, clusterName, n.Name, status, role, age, version)
    }
}

// listManagedClusters enumerates ManagedCluster objects and lists their nodes.
// parentCtx is the hosting cluster context (e.g., "its1") shown in the CONTEXT column.
func listManagedClusters(tw *tabwriter.Writer, dyn dynamic.Interface, kubeconfig, parentCtx string) {
    gvr := schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1", Resource: "managedclusters"}

    mcs, err := dyn.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        fmt.Fprintf(os.Stderr, "warning: could not list managedclusters: %v\n", err)
        return
    }

    for _, mc := range mcs.Items {
        mcName := mc.GetName() // e.g., "cluster1", "cluster2"

        // We may not have a kubeconfig context with the same name; attempt to build one.
        _, _, cs := buildClient(kubeconfig, mcName)
        listNodes(tw, parentCtx, mcName, cs)
    }
}

func printHeader(tw *tabwriter.Writer) {
    fmt.Fprintf(tw, tableFmt, "CONTEXT", "CLUSTER", "NAME", "STATUS", "ROLES", "AGE", "VERSION")
}

func printRow(tw *tabwriter.Writer, contextName, clusterName, name, status, roles, age, version string) {
    fmt.Fprintf(tw, tableFmt, contextName, clusterName, name, status, roles, age, version)
}

func humanAge(t time.Time) string {
    return time.Since(t).Round(time.Second).String()
}

func exitIf(err error) {
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
