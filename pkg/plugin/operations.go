package plugin

import (
	"log"
	"os"
	"os/exec"

	"sigs.k8s.io/yaml"
)

// DebugOperation represents a debug operation type
type DebugOperation int

const (
	OperationStandalone DebugOperation = iota
	OperationCopyPod
	OperationAddContainer
)

// DebugConfig holds the configuration for debug operations
type DebugConfig struct {
	Operation     DebugOperation
	Namespace     string
	PodName       string
	Image         string
	Interactive   bool
	TTY           bool
	RemoveAfter   bool
	Force         bool
	CopyPod       bool
	Profile       string
	CPURequest    string
	MemoryLimit   string
	MemoryRequest string
}

// NewDebugConfigFromFlags creates a DebugConfig from global flags
func NewDebugConfigFromFlags() *DebugConfig {
	config := &DebugConfig{
		Namespace:     namespace,
		PodName:       podName,
		Image:         image,
		Interactive:   interactive,
		TTY:           tty,
		RemoveAfter:   removeAfter,
		Force:         force,
		CopyPod:       copyPod,
		Profile:       profile,
		CPURequest:    cpuRequest,
		MemoryLimit:   memoryLimit,
		MemoryRequest: memoryRequest,
	}

	// Determine operation type
	if config.PodName == "" {
		config.Operation = OperationStandalone
	} else if config.CopyPod {
		config.Operation = OperationCopyPod
	} else {
		config.Operation = OperationAddContainer
	}

	return config
}

// Execute runs the debug operation based on the configuration
func (config *DebugConfig) Execute() error {
	switch config.Operation {
	case OperationStandalone:
		return config.executeStandalone()
	case OperationCopyPod:
		return config.executeCopyPod()
	case OperationAddContainer:
		return config.executeAddContainer()
	default:
		return NewValidationError("operation", "unknown", "invalid debug operation")
	}
}

// executeStandalone creates a new standalone debug pod
func (config *DebugConfig) executeStandalone() error {
	debugPodName, err := config.createDebugPod()
	if err != nil {
		return WrapKubectlError(err, "create debug pod")
	}

	// Set up signal handler for cleanup
	if config.RemoveAfter {
		config.setupSignalHandler(debugPodName)
	}

	// Wait for pod to be ready only if we're going to attach to it
	if config.Interactive && config.TTY {
		log.Printf("Waiting for pod to be ready...")
		if err := config.waitForPod(debugPodName); err != nil {
			return NewTimeoutError("pod ready", "30s").WithOriginalError(err)
		}
	}

	// If --rm flag is set, clean up the pod after the session ends
	if config.RemoveAfter {
		defer func() {
			log.Printf("Cleaning up debug pod %s...", debugPodName)
			deleteArgs := []string{
				"delete",
				"pod",
				debugPodName,
				"-n",
				config.Namespace,
			}
			deleteCmd := ExecCommand("kubectl", deleteArgs...)
			if err := deleteCmd.Run(); err != nil {
				log.Printf("Warning: Failed to delete debug pod: %v", err)
			} else {
				log.Printf("Debug pod deleted successfully")
			}
		}()
	}

	// Attach to the pod if interactive mode is enabled
	if config.Interactive && config.TTY {
		attachArgs := []string{
			"attach",
			"-it",
			debugPodName,
			"-n",
			config.Namespace,
		}
		attachCmd := ExecCommand("kubectl", attachArgs...)
		attachCmd.Stdin = os.Stdin
		attachCmd.Stdout = os.Stdout
		attachCmd.Stderr = os.Stderr
		if err := attachCmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return WrapKubectlError(err, "attach to pod")
		}
	} else {
		log.Printf("You can access the pod with: kubectl exec -it %s -n %s -- sh\n", debugPodName, config.Namespace)
	}

	return nil
}

// executeCopyPod creates a copy of the target pod with debug container
func (config *DebugConfig) executeCopyPod() error {
	// Verify target pod exists
	if err := config.verifyTargetPod(); err != nil {
		return err
	}

	// Check for existing debug pod
	if !config.Force {
		existingPod, err := config.findExistingDebugPod()
		if err != nil {
			return WrapKubectlError(err, "check existing debug pods")
		}

		if existingPod != "" {
			if !config.askForNewPod(existingPod) {
				return config.useExistingPod(existingPod)
			}
		}
	}

	// Create the copy
	return config.createPodCopy()
}

// executeAddContainer adds an ephemeral container to existing pod
func (config *DebugConfig) executeAddContainer() error {
	// Verify target pod exists
	if err := config.verifyTargetPod(); err != nil {
		return err
	}

	// Get the target container name
	containerName, err := config.getTargetContainerName()
	if err != nil {
		return WrapKubectlError(err, "get target container name")
	}

	args := []string{
		"debug", config.PodName,
		"-n", config.Namespace,
		"--image", config.Image,
		"--target=" + containerName,
	}

	// Always set profile if specified, otherwise use "general" as default
	if config.Profile != "" {
		args = append(args, "--profile="+config.Profile)
	} else {
		args = append(args, "--profile=general")
	}

	if config.Interactive {
		args = append(args, "-i")
	}
	if config.TTY {
		args = append(args, "-t")
	}
	if config.Interactive && config.TTY {
		args = append(args, "--")
	}

	log.Printf("Adding debug container to pod %s (targeting container %s)...\n", config.PodName, containerName)
	cmd := ExecCommand("kubectl", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Helper methods

func (config *DebugConfig) verifyTargetPod() error {
	cmd := ExecCommand("kubectl", "get", "pod", config.PodName, "-n", config.Namespace)
	if cmd.Run() != nil {
		return NewPodNotFoundError(config.PodName, config.Namespace)
	}
	return nil
}

func (config *DebugConfig) useExistingPod(existingPod string) error {
	log.Printf("Using existing debug pod: %s\n", existingPod)
	if config.Interactive && config.TTY {
		log.Printf("Attaching to pod...\n")
		if err := config.attachToPod(existingPod); err != nil {
			return WrapKubectlError(err, "attach to existing pod")
		}
		if config.RemoveAfter {
			log.Printf("Removing debug pod...\n")
			if err := config.deletePod(existingPod); err != nil {
				return WrapKubectlError(err, "delete pod")
			}
		}
	} else {
		log.Printf("You can access the pod with: kubectl exec -it %s -n %s -- sh\n", existingPod, config.Namespace)
	}
	return nil
}

func (config *DebugConfig) createPodCopy() error {
	debugPodName := config.generateUniqueName()

	// Set up signal handler for cleanup
	if config.RemoveAfter {
		config.setupSignalHandler(debugPodName)
	}

	// Create custom debug container configuration for resources
	customDebug := map[string]interface{}{
		"resources": map[string]interface{}{
			"limits": map[string]string{
				"memory": config.MemoryLimit,
			},
			"requests": map[string]string{
				"cpu":    config.CPURequest,
				"memory": config.MemoryRequest,
			},
		},
	}

	// Create temporary file for custom debug configuration
	customYAML, err := yaml.Marshal(customDebug)
	if err != nil {
		return NewDetailedError(ErrorTypeValidation, "failed to create custom debug configuration").WithOriginalError(err)
	}

	tmpfile, err := os.CreateTemp("", "debug-custom-*.yaml")
	if err != nil {
		return NewDetailedError(ErrorTypeValidation, "failed to create temporary file").WithOriginalError(err)
	}
	defer func() {
		_ = os.Remove(tmpfile.Name())
	}()

	if _, err := tmpfile.Write(customYAML); err != nil {
		return NewDetailedError(ErrorTypeValidation, "failed to write custom debug configuration").WithOriginalError(err)
	}
	if err := tmpfile.Close(); err != nil {
		return NewDetailedError(ErrorTypeValidation, "failed to close temporary file").WithOriginalError(err)
	}

	// Check if target pod has a security context
	secContext, err := config.getTargetPodSecurityContext()
	if err != nil {
		log.Printf("Warning: Could not get target pod security context: %v", err)
	}

	args := []string{
		"debug", config.PodName,
		"-n", config.Namespace,
		"--image", config.Image,
		"--share-processes",
		"--copy-to=" + debugPodName,
		"--custom=" + tmpfile.Name(),
	}

	// Only set profile if target pod has security context or profile was explicitly set
	if (secContext != nil && secContext.RunAsUser != nil) || config.Profile != "" {
		profileToUse := config.Profile
		if profileToUse == "" {
			profileToUse = "general"
		}
		args = append(args, "--profile="+profileToUse)
	}

	if config.Interactive {
		args = append(args, "-i")
	}
	if config.TTY {
		args = append(args, "-t")
	}
	if config.Interactive && config.TTY {
		args = append(args, "--")
	}

	log.Printf("Creating debug pod %s as a copy of %s...\n", debugPodName, config.PodName)
	cmd := ExecCommand("kubectl", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return WrapKubectlError(err, "create debug pod copy")
	}

	if !config.Interactive || !config.TTY {
		log.Printf("You can access the pod with: kubectl exec -it %s -n %s -- sh\n", debugPodName, config.Namespace)
	}

	if config.RemoveAfter && config.Interactive && config.TTY {
		log.Printf("Removing debug pod...\n")
		if err := config.deletePod(debugPodName); err != nil {
			return WrapKubectlError(err, "delete debug pod")
		}
	}

	return nil
}
