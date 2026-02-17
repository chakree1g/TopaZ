package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	appv1alpha1 "github.com/chakradharkondapalli/topas/api/v1alpha1"
	"github.com/chakradharkondapalli/topas/pkg/k8s"
)

var statusCmd = &cobra.Command{
	Use:   "status <test-run-name>",
	Short: "Show the status of a test run",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runName := args[0]

		client, err := k8s.NewClient()
		if err != nil {
			fmt.Printf("Error creating client: %v\n", err)
			os.Exit(1)
		}

		testRun := &appv1alpha1.TestRun{}
		err = client.Get(context.Background(), types.NamespacedName{Name: runName, Namespace: namespace}, testRun)
		if err != nil {
			fmt.Printf("Error getting TestRun: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Name:       %s\n", testRun.Name)
		fmt.Printf("Namespace:  %s\n", testRun.Namespace)
		fmt.Printf("App:        %s\n", testRun.Spec.AppName)
		fmt.Printf("State:      %s\n", testRun.Status.State)
		fmt.Printf("Result:     %s\n", testRun.Status.Result)
		fmt.Printf("RunnerPod:  %s\n", testRun.Status.RunnerPod)
		if testRun.Status.StartTime != nil {
			fmt.Printf("Started:    %s\n", testRun.Status.StartTime.Format("2006-01-02 15:04:05"))
		}
		if testRun.Status.CompletionTime != nil {
			fmt.Printf("Completed:  %s\n", testRun.Status.CompletionTime.Format("2006-01-02 15:04:05"))
			if testRun.Status.StartTime != nil {
				duration := testRun.Status.CompletionTime.Sub(testRun.Status.StartTime.Time)
				fmt.Printf("Duration:   %s\n", duration.Round(time.Millisecond))
			}
		}
	},
}

func init() {
	testCmd.AddCommand(statusCmd)
}
