package main

import (
	"fmt"
	"os"

	"golang.org/x/net/context"

	"github.com/ibrokethecloud/workload/pkg/workload"
)

func main() {

	root, err := workload.NewWorkloadCommand(context.TODO())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
