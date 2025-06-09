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

// tableFmt uses \t (tab) separators so that tabwriter can adjust each column
// to the widest cell automatically. No fixed widths are hard‑coded.
const tableFmt = "%s\t%s\t%s\t%s\t%s\t%s\t\n"

func main() {
    remoteCtx := flag.String("remote-context", "its1", "remote hosting context (for ManagedCluster)")
    kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig (defaults to $HOME/.kube/config)")
    flag.Parse()

    // Create a single tabwriter instance for the whole table.
    tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

    // ---------- local (current) cluster ----------
    currCtx, localClient := buildClient(*kubeconfig, "")
    printHeader(tw)
    listNodes(tw, currCtx, localClient)

    // ---------- managed (remote) clusters ----------
    if *remoteCtx != "" {
        dyn := buildDynamicClient(*kubeconfig, *remoteCtx)
        listManagedClusters(tw, dyn, *kubeconfig)
    }

    // Flush once at the end so that all rows are aligned consistently.
    tw.Flush()
}

// buildClient returns the context name and a typed clientset bound to it.
func buildClient(kcfg, ctxOverride string) (string, *kubernetes.Clientset) {
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

    return rawCfg.CurrentContext, cs
}

// buildDynamicClient creates a dynamic client bound to the given context.
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

// listNodes prints one line per node belonging to the given cluster.
func listNodes(tw *tabwriter.Writer, clusterName string, cs *kubernetes.Clientset) {
    nodes, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
    exitIf(err)

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

        printRow(tw, clusterName, n.Name, status, role, age, version)
    }
}

// listManagedClusters discovers ManagedCluster resources and prints their nodes.
func listManagedClusters(tw *tabwriter.Writer, dyn dynamic.Interface, kubeconfig string) {
    gvr := schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1", Resource: "managedclusters"}

    mcs, err := dyn.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        fmt.Fprintf(os.Stderr, "warning: could not list managedclusters: %v\n", err)
        return
    }

    for _, mc := range mcs.Items {
        name := mc.GetName()
        // create a client bound to this managed cluster context
        _, cs := buildClient(kubeconfig, name)
        listNodes(tw, name, cs)
    }
}

// printHeader outputs the table header.
func printHeader(tw *tabwriter.Writer) {
    fmt.Fprintf(tw, tableFmt, "CLUSTER", "NAME", "STATUS", "ROLES", "AGE", "VERSION")
}

// printRow outputs a single data row.
func printRow(tw *tabwriter.Writer, cluster, name, status, roles, age, version string) {
    fmt.Fprintf(tw, tableFmt, cluster, name, status, roles, age, version)
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
