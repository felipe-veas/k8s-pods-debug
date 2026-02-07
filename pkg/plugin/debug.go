package plugin

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

// Add at the top of the file, after imports
var ExecCommand = exec.Command

// Add near the top with other vars
var (
	sleepDuration = time.Second
	maxAttempts   = 30 // 30 seconds max wait time
)

// ExecError type for execution errors
type ExecError struct {
	msg string
}

func (e *ExecError) Error() string {
	return e.msg
}

func (config *DebugConfig) findExistingDebugPod() (string, error) {
	labelSelector := "debug-tool/type=debug-pod"
	if config.PodName != "" {
		labelSelector += fmt.Sprintf(",debug-tool/target=%s", config.PodName)
	}

	cmd := ExecCommand("kubectl", "get", "pod", "-n", config.Namespace, "-l", labelSelector,
		"--no-headers",
		"-o", "custom-columns=:metadata.name")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()

	// If there's an error, check if it's because no pods were found
	if err != nil {
		if strings.Contains(stderr.String(), "No resources found") {
			return "", nil
		}
		return "", fmt.Errorf("error checking for existing pods: %v - %s", err, stderr.String())
	}

	// Get the first non-empty pod name
	for _, line := range strings.Split(string(output), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			return name, nil
		}
	}

	return "", nil
}

func (config *DebugConfig) askForNewPod(existingPod string) bool {
	if config.Force {
		return true
	}

	fmt.Printf("Debug pod '%s' already exists in namespace '%s'. Do you want to:\n", existingPod, config.Namespace)
	fmt.Printf("[1] Use existing pod\n")
	fmt.Printf("[2] Create new pod\n")
	fmt.Printf("Choose (1/2) [1]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(response)
	return response == "2"
}

func (config *DebugConfig) generateUniqueName() string {
	timestamp := time.Now().Format("150405") // HHMMSS
	randomStr := fmt.Sprintf("%04d", rand.Intn(10000))

	// If no target pod, use simpler name format
	if config.PodName == "" {
		return fmt.Sprintf("debug-%s-%s", timestamp, randomStr)
	}

	// If target pod specified, include it in the name
	return fmt.Sprintf("debug-%s-%s-%s", config.PodName, timestamp, randomStr)
}

func (config *DebugConfig) attachToPod(debugPodName string) error {
	args := []string{"exec", "-it", debugPodName, "-n", config.Namespace, "--", "sh"}
	cmd := ExecCommand("kubectl", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (config *DebugConfig) deletePod(debugPodName string) error {
	cmd := ExecCommand("kubectl", "delete", "pod", debugPodName, "-n", config.Namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (config *DebugConfig) getTargetPodLabels() (map[string]string, error) {
	cmd := ExecCommand("kubectl", "get", "pod", config.PodName, "-n", config.Namespace, "-o", "jsonpath={.metadata.labels}")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting target pod labels: %v - %s", err, stderr.String())
	}

	// If no output, return a map with basic labels
	if len(output) == 0 {
		return map[string]string{
			"debug-tool/type":   "debug-pod",
			"debug-tool/target": config.PodName,
		}, nil
	}

	// Parse JSON output to a map
	labels := make(map[string]string)
	if err := json.Unmarshal(output, &labels); err != nil {
		log.Printf("Warning: Error parsing labels JSON: %v, using basic labels", err)
		return map[string]string{
			"debug-tool/type":   "debug-pod",
			"debug-tool/target": config.PodName,
		}, nil
	}

	return labels, nil
}

func (config *DebugConfig) waitForPod(debugPodName string) error {
	for i := 0; i < maxAttempts; i++ {
		cmd := ExecCommand("kubectl", "get", "pod", debugPodName, "-n", config.Namespace,
			"-o", "jsonpath={.status.phase}")
		output, err := cmd.Output()
		if err == nil && string(output) == "Running" {
			return nil
		}
		time.Sleep(sleepDuration)
	}
	return fmt.Errorf("pod did not become ready within %d seconds", maxAttempts)
}

func (config *DebugConfig) getDeploymentSelectors() (map[string]string, error) {
	// First get the deployment name by looking for the pod's owner reference
	cmd := ExecCommand("kubectl", "get", "pod", config.PodName, "-n", config.Namespace,
		"-o", "jsonpath={.metadata.ownerReferences[?(@.kind=='ReplicaSet')].name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting pod owner reference: %v", err)
	}
	replicaSetName := strings.TrimSpace(string(output))
	if replicaSetName == "" {
		return nil, nil // Pod does not belong to a ReplicaSet
	}

	// Get deployment name from ReplicaSet
	cmd = ExecCommand("kubectl", "get", "rs", replicaSetName, "-n", config.Namespace,
		"-o", "jsonpath={.metadata.ownerReferences[?(@.kind=='Deployment')].name}")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting replicaset owner reference: %v", err)
	}
	deploymentName := strings.TrimSpace(string(output))
	if deploymentName == "" {
		return nil, nil // ReplicaSet does not belong to a Deployment
	}

	// Get deployment matchLabels
	cmd = ExecCommand("kubectl", "get", "deployment", deploymentName, "-n", config.Namespace,
		"-o", "jsonpath={.spec.selector.matchLabels}")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting deployment selector: %v", err)
	}

	// Parse matchLabels
	selectors := make(map[string]string)
	if err := json.Unmarshal(output, &selectors); err != nil {
		return nil, fmt.Errorf("error parsing deployment selector: %v", err)
	}

	return selectors, nil
}

func (config *DebugConfig) getTargetPodSecurityContext() (*corev1.PodSecurityContext, error) {
	cmd := ExecCommand("kubectl", "get", "pod", config.PodName, "-n", config.Namespace, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting pod info: %v", err)
	}

	var pod corev1.Pod
	if err := json.Unmarshal(output, &pod); err != nil {
		return nil, fmt.Errorf("error parsing pod JSON: %v", err)
	}

	return pod.Spec.SecurityContext, nil
}

func getSecurityContextForProfile(profileName string) (*corev1.SecurityContext, *corev1.PodSecurityContext) {
	containerContext := &corev1.SecurityContext{
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	podContext := &corev1.PodSecurityContext{
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	switch profileName {
	case "restricted":
		containerContext.AllowPrivilegeEscalation = ptr.To(false)
		containerContext.Capabilities = &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
		containerContext.RunAsNonRoot = ptr.To(true)
		containerContext.RunAsUser = ptr.To(int64(1000))
		containerContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault

		podContext.RunAsNonRoot = ptr.To(true)
		podContext.RunAsUser = ptr.To(int64(1000))
		podContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault

	case "baseline":
		containerContext.AllowPrivilegeEscalation = ptr.To(false)
		containerContext.Capabilities = &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
		containerContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault

		podContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault

	case "privileged":
		containerContext.AllowPrivilegeEscalation = ptr.To(true)
		containerContext.Privileged = ptr.To(true)
		containerContext.Capabilities = &corev1.Capabilities{
			Add: []corev1.Capability{"ALL"},
		}
		containerContext.SeccompProfile.Type = corev1.SeccompProfileTypeUnconfined

		podContext.SeccompProfile.Type = corev1.SeccompProfileTypeUnconfined
	}

	return containerContext, podContext
}

func (config *DebugConfig) createDebugPod() (string, error) {
	debugPodName := config.generateUniqueName()
	log.Printf("Generating debug pod name: %s", debugPodName)

	// Initialize basic labels
	labels := map[string]string{
		"debug-tool/type": "debug-pod",
	}

	// Configure pod spec
	automountServiceAccountToken := false
	podSpec := corev1.PodSpec{
		AutomountServiceAccountToken:  &automountServiceAccountToken,
		TerminationGracePeriodSeconds: ptr.To(int64(0)),
	}

	// If targeting an existing pod
	if config.PodName != "" {
		// Try to get target pod's security context
		secContext, err := config.getTargetPodSecurityContext()
		if err != nil {
			log.Printf("Warning: Could not get target pod security context: %v", err)
		} else if secContext != nil && secContext.RunAsUser != nil {
			// Only set security context if target pod has RunAsUser defined
			podSpec.SecurityContext = secContext
			log.Printf("Using security context from target pod (UID: %d)", *secContext.RunAsUser)
		} else {
			log.Printf("No security context defined in target pod, using profile settings")
		}

		// Get target pod labels
		targetLabels, err := config.getTargetPodLabels()
		if err == nil {
			labels = targetLabels
		}
		labels["debug-tool/target"] = config.PodName

		// Remove deployment selectors if present
		deploymentSelectors, err := config.getDeploymentSelectors()
		if err == nil && deploymentSelectors != nil {
			for key := range deploymentSelectors {
				delete(labels, key)
			}
		}

		// Enable process namespace sharing
		shareProcessNamespace := true
		podSpec.ShareProcessNamespace = &shareProcessNamespace
	}

	// If no security context is set from target pod, use profile settings
	if podSpec.SecurityContext == nil {
		_, podContext := getSecurityContextForProfile(config.Profile)
		podSpec.SecurityContext = podContext
		log.Printf("Using security context from profile: %s", config.Profile)
	}

	// Ensure debug tool labels are present
	labels["debug-tool/type"] = "debug-pod"

	debugPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      debugPodName,
			Namespace: config.Namespace,
			Labels:    labels,
		},
		Spec: podSpec,
	}

	// Add the debug container with appropriate security context
	containerContext, _ := getSecurityContextForProfile(config.Profile)
	if podSpec.SecurityContext != nil && podSpec.SecurityContext.RunAsUser != nil {
		// If pod has a specific RunAsUser, override the container's RunAsUser
		containerContext.RunAsUser = podSpec.SecurityContext.RunAsUser
		containerContext.RunAsNonRoot = podSpec.SecurityContext.RunAsNonRoot
	}

	// Add the debug container
	var command []string
	if config.Interactive && config.TTY {
		command = []string{"bash"}
	} else {
		command = []string{"sleep", "infinity"}
	}

	debugPod.Spec.Containers = []corev1.Container{
		{
			Name:            "debugger",
			Image:           config.Image,
			Command:         command,
			Stdin:           true,
			TTY:             true,
			SecurityContext: containerContext,
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse(config.MemoryLimit),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(config.CPURequest),
					corev1.ResourceMemory: resource.MustParse(config.MemoryRequest),
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"/bin/true"},
					},
				},
				InitialDelaySeconds: 5,
				PeriodSeconds:       10,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"/bin/true"},
					},
				},
				InitialDelaySeconds: 5,
				PeriodSeconds:       10,
			},
		},
	}

	podYAML, err := yaml.Marshal(debugPod)
	if err != nil {
		return "", fmt.Errorf("error generating YAML: %v", err)
	}

	log.Printf("Applying debug pod YAML...")
	applyCmd := ExecCommand("kubectl", "apply", "-f", "-")
	applyCmd.Stdin = bytes.NewReader(podYAML)
	var stderr bytes.Buffer
	applyCmd.Stderr = &stderr
	if err := applyCmd.Run(); err != nil {
		return "", fmt.Errorf("error creating debug pod: %v - %s", err, stderr.String())
	}

	log.Printf("Debug pod created successfully")
	return debugPodName, nil
}

func (config *DebugConfig) getTargetContainerName() (string, error) {
	cmd := ExecCommand("kubectl", "get", "pod", config.PodName, "-n", config.Namespace,
		"-o", "jsonpath={.spec.containers[0].name}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting container name: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (config *DebugConfig) setupSignalHandler(debugPodName string) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Printf("\nReceived interrupt signal, cleaning up...")
		if err := config.deletePod(debugPodName); err != nil {
			log.Printf("Warning: Failed to delete pod %s: %v", debugPodName, err)
		}
		os.Exit(1)
	}()
}

func runDebug() error {
	config := NewDebugConfigFromFlags()
	return config.Execute()
}
