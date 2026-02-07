package plugin

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	cleanAllNamespaces bool
	cleanForce         bool
	cleanOlderThan     string
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up debug pods",
	Long: `Clean up debug pods created by kpdbug tool.
This command will remove debug pods based on the specified criteria.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClean()
	},
}

func init() {
	cleanCmd.Flags().BoolVarP(&cleanAllNamespaces, "all-namespaces", "A", false, "clean debug pods across all namespaces")
	cleanCmd.Flags().BoolVarP(&cleanForce, "force", "f", false, "force cleanup without confirmation")
	cleanCmd.Flags().StringVar(&cleanOlderThan, "older-than", "", "clean pods older than specified duration (e.g., 1h, 30m)")
	rootCmd.AddCommand(cleanCmd)
}

func runClean() error {
	debugPods, err := getDebugPods()
	if err != nil {
		return fmt.Errorf("failed to get debug pods: %v", err)
	}

	if len(debugPods) == 0 {
		fmt.Println("No debug pods found to clean")
		return nil
	}

	podsToDelete, err := filterPodsForCleanup(debugPods)
	if err != nil {
		return fmt.Errorf("failed to filter pods: %v", err)
	}

	if len(podsToDelete) == 0 {
		fmt.Println("No debug pods match the cleanup criteria")
		return nil
	}

	if !cleanForce {
		fmt.Printf("The following debug pods will be deleted:\n")
		for _, pod := range podsToDelete {
			target := pod.TargetPod
			if target == "" {
				target = "<standalone>"
			}
			fmt.Printf("  %s/%s (target: %s, age: %s)\n",
				pod.Namespace, pod.Name, target, pod.Age)
		}

		if !askForConfirmation("Do you want to continue? (y/N): ") {
			fmt.Println("Cleanup cancelled")
			return nil
		}
	}

	deletedCount := 0
	for _, pod := range podsToDelete {
		err := deletePodByName(pod.Name, pod.Namespace)
		if err != nil {
			fmt.Printf("Warning: Failed to delete pod %s/%s: %v\n",
				pod.Namespace, pod.Name, err)
		} else {
			fmt.Printf("Deleted debug pod %s/%s\n", pod.Namespace, pod.Name)
			deletedCount++
		}
	}

	fmt.Printf("Successfully deleted %d debug pods\n", deletedCount)
	return nil
}

func filterPodsForCleanup(pods []DebugPodInfo) ([]DebugPodInfo, error) {
	if cleanOlderThan == "" {
		return pods, nil
	}

	duration, err := time.ParseDuration(cleanOlderThan)
	if err != nil {
		return nil, fmt.Errorf("invalid duration format for --older-than: %v", err)
	}

	var filtered []DebugPodInfo
	cutoff := time.Now().Add(-duration)

	for _, pod := range pods {
		if pod.CreationTimestamp.Before(cutoff) {
			filtered = append(filtered, pod)
		}
	}

	return filtered, nil
}

func askForConfirmation(prompt string) bool {
	fmt.Print(prompt)
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func deletePodByName(podName, namespace string) error {
	cmd := ExecCommand("kubectl", "delete", "pod", podName, "-n", namespace)
	return cmd.Run()
}
