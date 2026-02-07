package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type DebugPodInfo struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	TargetPod         string    `json:"target_pod,omitempty"`
	Status            string    `json:"status"`
	Age               string    `json:"age"`
	CreationTimestamp time.Time `json:"-"`
	Image             string    `json:"image"`
	Node              string    `json:"node,omitempty"`
}

var (
	listAllNamespaces bool
	outputFormat      string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List active debug pods",
	Long: `List all active debug pods created by kpdbug tool.
Shows information about debug pods including their status, target pod, and age.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList()
	},
}

func init() {
	listCmd.Flags().BoolVarP(&listAllNamespaces, "all-namespaces", "A", false, "list debug pods across all namespaces")
	listCmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "output format (table, json, yaml)")
	rootCmd.AddCommand(listCmd)
}

func runList() error {
	debugPods, err := getDebugPods()
	if err != nil {
		return fmt.Errorf("failed to get debug pods: %v", err)
	}

	if len(debugPods) == 0 {
		fmt.Println("No debug pods found")
		return nil
	}

	switch outputFormat {
	case "json":
		return outputJSON(debugPods)
	case "yaml":
		return outputYAML(debugPods)
	default:
		return outputTable(debugPods)
	}
}

func getDebugPods() ([]DebugPodInfo, error) {
	var args []string
	if listAllNamespaces {
		args = []string{"get", "pods", "--all-namespaces",
			"-l", "debug-tool/type=debug-pod", "-o", "json"}
	} else {
		args = []string{"get", "pods", "-n", namespace,
			"-l", "debug-tool/type=debug-pod", "-o", "json"}
	}

	cmd := ExecCommand("kubectl", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(stderr.String(), "No resources found") {
			return []DebugPodInfo{}, nil
		}
		return nil, fmt.Errorf("error listing pods: %v - %s", err, stderr.String())
	}

	var podList corev1.PodList
	if err := json.Unmarshal(output, &podList); err != nil {
		return nil, fmt.Errorf("error parsing pod list: %v", err)
	}

	var debugPods []DebugPodInfo
	for _, pod := range podList.Items {
		debugPod := DebugPodInfo{
			Name:              pod.Name,
			Namespace:         pod.Namespace,
			Status:            string(pod.Status.Phase),
			Age:               calculateAge(pod.CreationTimestamp.Time),
			CreationTimestamp: pod.CreationTimestamp.Time,
			Node:              pod.Spec.NodeName,
		}

		// Get target pod from labels
		if targetPod, exists := pod.Labels["debug-tool/target"]; exists {
			debugPod.TargetPod = targetPod
		}

		// Get image from first container
		if len(pod.Spec.Containers) > 0 {
			debugPod.Image = pod.Spec.Containers[0].Image
		}

		debugPods = append(debugPods, debugPod)
	}

	return debugPods, nil
}

func calculateAge(creationTime time.Time) string {
	duration := time.Since(creationTime)

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

func outputTable(debugPods []DebugPodInfo) error {
	if listAllNamespaces {
		fmt.Printf("%-30s %-15s %-20s %-12s %-8s %-25s\n",
			"NAME", "NAMESPACE", "TARGET", "STATUS", "AGE", "IMAGE")
		fmt.Printf("%-30s %-15s %-20s %-12s %-8s %-25s\n",
			"----", "---------", "------", "------", "---", "-----")

		for _, pod := range debugPods {
			target := pod.TargetPod
			if target == "" {
				target = "<standalone>"
			}
			fmt.Printf("%-30s %-15s %-20s %-12s %-8s %-25s\n",
				truncateString(pod.Name, 30),
				pod.Namespace,
				truncateString(target, 20),
				pod.Status,
				pod.Age,
				truncateString(pod.Image, 25))
		}
	} else {
		fmt.Printf("%-30s %-20s %-12s %-8s %-25s\n",
			"NAME", "TARGET", "STATUS", "AGE", "IMAGE")
		fmt.Printf("%-30s %-20s %-12s %-8s %-25s\n",
			"----", "------", "------", "---", "-----")

		for _, pod := range debugPods {
			target := pod.TargetPod
			if target == "" {
				target = "<standalone>"
			}
			fmt.Printf("%-30s %-20s %-12s %-8s %-25s\n",
				truncateString(pod.Name, 30),
				truncateString(target, 20),
				pod.Status,
				pod.Age,
				truncateString(pod.Image, 25))
		}
	}
	return nil
}

func outputJSON(debugPods []DebugPodInfo) error {
	jsonData, err := json.MarshalIndent(debugPods, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling to JSON: %v", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

func outputYAML(debugPods []DebugPodInfo) error {
	yamlData, err := yaml.Marshal(debugPods)
	if err != nil {
		return fmt.Errorf("error marshaling to YAML: %v", err)
	}
	fmt.Print(string(yamlData))
	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
