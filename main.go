package main

import (
	"fmt"
	"os"

	"golang.org/x/net/context"

	"github.com/ibrokethecloud/workload/pkg/workload"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog"
)

func main() {
	defer klog.Flush()

	root, err := workload.NewWorkloadCommand(context.Background(), genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
