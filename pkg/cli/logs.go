package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	appv1alpha1 "github.com/chakradharkondapalli/topas/api/v1alpha1"
	"github.com/chakradharkondapalli/topas/pkg/k8s"
)

var logsCmd = &cobra.Command{
	Use:   "logs <test-run-name>",
	Short: "Stream logs from a test run",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runName := args[0]

		// 1. Get TestRun
		client, err := k8s.NewClient()
		if err != nil {
			fmt.Printf("Error creating client: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		testRun := &appv1alpha1.TestRun{}
		err = client.Get(ctx, types.NamespacedName{Name: runName, Namespace: namespace}, testRun)
		if err != nil {
			fmt.Printf("Error getting TestRun: %v\n", err)
			os.Exit(1)
		}

		// Wait for RunnerPod to be assigned
		if testRun.Status.RunnerPod == "" {
			fmt.Println("Waiting for runner pod assignment...")
			for i := 0; i < 30; i++ {
				time.Sleep(1 * time.Second)
				client.Get(ctx, types.NamespacedName{Name: runName, Namespace: namespace}, testRun)
				if testRun.Status.RunnerPod != "" {
					break
				}
			}
			if testRun.Status.RunnerPod == "" {
				fmt.Println("Timed out waiting for runner pod.")
				os.Exit(1)
			}
		}

		fmt.Printf("Streaming logs from pod: %s\n", testRun.Status.RunnerPod)

		// 2. Stream Logs
		clientset, err := k8s.NewClientset()
		if err != nil {
			fmt.Printf("Error creating clientset: %v\n", err)
			os.Exit(1)
		}

		req := clientset.CoreV1().Pods(namespace).GetLogs(testRun.Status.RunnerPod, &corev1.PodLogOptions{
			Container: "runner",
			Follow:    true,
		})

		stream, err := req.Stream(ctx)
		if err != nil {
			fmt.Printf("Error opening stream: %v\n", err)
			os.Exit(1)
		}
		defer stream.Close()

		_, err = io.Copy(os.Stdout, stream)
		if err != nil {
			// fmt.Printf("Stream ended: %v\n", err)
		}
	},
}

func init() {
	testCmd.AddCommand(logsCmd)
}
