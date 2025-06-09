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

const tableFmt = "%-15s %-35s %-10s %-15s %-8s %-10s\n"

func main() {
    remoteCtx := flag.String("remote-context", "its1", "remote hosting context (for ManagedCluster)")
    kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig")
    flag.Parse()

    // ----- local cluster -----
    currCtx, localClient := buildClient(*kubeconfig, "")
    printHeader()
    listNodes(currCtx, localClient)

    // ----- managed clusters (remote) -----
    if *remoteCtx != "" {
        dyn := buildDynamicClient(*kubeconfig, *remoteCtx)
        listManagedClusters(dyn, *kubeconfig)
    }
}

// buildClient returns the context name and a typed clientset
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

// buildDynamicClient creates a dynamic client bound to the given context
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

// listNodes prints one line per node belonging to the given cluster
func listNodes(clusterName string, cs *kubernetes.Clientset) {
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
            if len(k) >= 24 && k[:24] == "node-role.kubernetes.io/" {
                role = k[24:]
                break
            }
        }

        age := humanAge(n.CreationTimestamp.Time)
        version := n.Status.NodeInfo.KubeletVersion

        printRow(clusterName, n.Name, status, role, age, version)
    }
}

// listManagedClusters discovers ManagedCluster resources and prints their nodes
func listManagedClusters(dyn dynamic.Interface, kubeconfig string) {
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
        listNodes(name, cs)
    }
}

// printHeader prints the table header without vertical bars or horizontals
func printHeader() {
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintf(w, tableFmt, "CLUSTER", "NAME", "STATUS", "ROLES", "AGE", "VERSION")
    w.Flush()
}

func printRow(cluster, name, status, roles, age, version string) {
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintf(w, tableFmt, cluster, name, status, roles, age, version)
    w.Flush()
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
