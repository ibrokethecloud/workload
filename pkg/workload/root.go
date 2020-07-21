package workload

import (
	"context"
	"fmt"
	"os"

	"k8s.io/client-go/dynamic"

	"github.com/spf13/cobra"
)

type WorkloadCommandConfig struct {
	Namespace string
	AllKinds  bool
	Client    dynamic.Interface
	Context   context.Context
	Start     bool
	Stop      bool
}

type WorkloadConfig struct {
	Name  string `json:"name"`
	Scale int64  `json:"scale"`
	Kind  string `json:"kind"`
}

func NewWorkloadCommand(ctx context.Context) (*cobra.Command, error) {
	var err error
	wcc := WorkloadCommandConfig{}
	rootCmd := &cobra.Command{
		Use:   "kubectl-workload",
		Short: "kubectl plugin to stop / start workloads",
		Long: `The plugin interacts with the k8s api to generate a list of workloads in the specified
	namespaces. k8s has no concept of stopping / starting workloads. In certain scenarios, this may be required.
	the plugin performs a stop by saving the state of the workload in a configmap before scaling it to 0, and then
	using the same saved stage to start the workload by restoring the scale.`,
		Run: func(cmd *cobra.Command, args []string) {
			wcc.Client, err = CreateClientset()
			if err != nil {
				fmt.Errorf("%v \n", err)
				os.Exit(1)
			}
			wcc.Context = ctx
			if wcc.Stop && wcc.Start {
				fmt.Println("Only one option scale-down/scale-up is possible at a time")
				os.Exit(1)
			}

			if len(args) == 0 && !wcc.AllKinds {
				fmt.Println("Need atleast one object type like deployment/<name-of-deployment>")
				os.Exit(1)
			}
			wcc.processCommand(args)
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&wcc.AllKinds, "all-kinds", "a", false, "operate on all deployments and statefulsets")
	rootCmd.PersistentFlags().StringVarP(&wcc.Namespace, "namespace", "n", "default", "namespace")
	rootCmd.PersistentFlags().BoolVar(&wcc.Stop, "stop", false, "scale down specified workloads")
	rootCmd.PersistentFlags().BoolVar(&wcc.Start, "start", false, "scale down specified workloads")

	return rootCmd, nil
}
