package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/chakradharkondapalli/topas/api/v1alpha1"
	"github.com/chakradharkondapalli/topas/pkg/k8s"
)

var (
	scriptPath string
	gitURL     string
	gitPath    string
	appName    string
	namespace  string
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Schedule a new test run",
	Run: func(cmd *cobra.Command, args []string) {
		if scriptPath == "" && gitURL == "" {
			fmt.Println("Error: --script or --git is required")
			os.Exit(1)
		}
		if appName == "" {
			fmt.Println("Error: --app is required")
			os.Exit(1)
		}

		// 1. Initialize Client
		k8sClient, err := k8s.NewClient()
		if err != nil {
			fmt.Printf("Failed to create client: %v\n", err)
			os.Exit(1)
		}

		// 2. Prepare TestRun
		runName := fmt.Sprintf("%s-run-%d", appName, os.Getpid()) // Simple naming for now
		testRun := &appv1alpha1.TestRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      runName,
				Namespace: namespace,
			},
			Spec: appv1alpha1.TestRunSpec{
				AppName: appName,
			},
		}

		if scriptPath != "" {
			content, err := os.ReadFile(scriptPath)
			if err != nil {
				fmt.Printf("Failed to read script: %v\n", err)
				os.Exit(1)
			}
			testRun.Spec.Script = string(content)
		} else {
			testRun.Spec.Git = &appv1alpha1.GitSource{
				URL:  gitURL,
				Path: gitPath,
			}
		}

		// 3. Create Resource
		if err := k8sClient.Create(context.Background(), testRun); err != nil {
			fmt.Printf("Failed to schedule test run: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("TestRun scheduled: %s/%s\n", namespace, runName)
	},
}

func init() {
	testCmd.AddCommand(scheduleCmd)

	scheduleCmd.Flags().StringVar(&scriptPath, "script", "", "Path to local Lua script")
	scheduleCmd.Flags().StringVar(&gitURL, "git", "", "Git repository URL")
	scheduleCmd.Flags().StringVar(&gitPath, "git-path", "", "Path within git repo")
	scheduleCmd.Flags().StringVar(&appName, "app", "", "Target App name")
	scheduleCmd.Flags().StringVar(&namespace, "namespace", "default", "Target Namespace")
}
