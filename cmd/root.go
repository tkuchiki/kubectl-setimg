package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tkuchiki/kubectl-setimg/pkg/k8s"
	"github.com/tkuchiki/kubectl-setimg/pkg/registry"
	"github.com/tkuchiki/kubectl-setimg/pkg/tui"
)

type SetImageOptions struct {
	configFlags *genericclioptions.ConfigFlags
	k8sClient   *k8s.Client
	registry    *registry.Client

	deployment string
	container  string
	image      string

	// Flags
	listOnly     bool
	watchMode    bool
	version      bool
	watchTimeout time.Duration

	// For rollback
	previousImage string
}

func NewSetImageOptions() *SetImageOptions {
	return &SetImageOptions{
		configFlags:  genericclioptions.NewConfigFlags(true),
		registry:     registry.NewClient(),
		watchTimeout: 5 * time.Minute,
	}
}

func (o *SetImageOptions) Complete(args []string) error {
	// Initialize Kubernetes client
	var err error
	o.k8sClient, err = k8s.NewClient(o.configFlags)
	if err != nil {
		return err
	}

	// Auto-detect interactive mode based on missing information
	// If any required information is missing and not in list mode, use interactive selection
	if !o.listOnly {
		shouldUseInteractive := false

		// Check if deployment is missing
		if len(args) < 1 {
			shouldUseInteractive = true
		} else {
			// Check if container=image is missing or incomplete
			if len(args) < 2 {
				shouldUseInteractive = true
			} else {
				// Check if the second argument is in container=image format
				containerImagePair := args[1]
				parts := strings.Split(containerImagePair, "=")
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					shouldUseInteractive = true
				}
			}
		}

		// For interactive mode, process deployment and container names if provided
		if shouldUseInteractive {
			fmt.Println("🎯 Missing required information, switching to interactive mode...")
			if len(args) >= 1 {
				o.deployment = args[0]
			}
			if len(args) >= 2 {
				// In interactive mode, only container name can be specified (not container=image)
				o.container = args[1]
			}
			// Run interactive mode directly in Complete
			return o.runInteractiveMode()
		}
	}

	// For list mode, only deployment name is required
	if o.listOnly {
		if len(args) < 1 {
			return fmt.Errorf("deployment name is required for --list mode")
		}
		o.deployment = args[0]
		return nil
	}

	// Direct mode: require both deployment and container=image
	if len(args) < 1 {
		return fmt.Errorf("deployment name is required")
	}
	o.deployment = args[0]

	if len(args) < 2 {
		return fmt.Errorf("container=image is required for direct mode")
	}

	containerImagePair := args[1]
	parts := strings.Split(containerImagePair, "=")
	if len(parts) != 2 {
		return fmt.Errorf("container=image format required for direct mode")
	}
	o.container = parts[0]
	o.image = parts[1]

	return nil
}

func (o *SetImageOptions) listContainers() error {
	containers, err := o.k8sClient.GetContainers(o.deployment)
	if err != nil {
		return err
	}

	fmt.Printf("Containers in deployment %s:\n", o.deployment)
	fmt.Println("INDEX\tNAME\t\tCURRENT IMAGE")
	fmt.Println("-----\t----\t\t-------------")
	for i, container := range containers {
		fmt.Printf("%d\t%s\t\t%s\n", i+1, container.Name, container.Image)
	}

	return nil
}

func (o *SetImageOptions) runInteractiveMode() error {
	var err error

	// 1. Select deployment
	if o.deployment == "" {
		fmt.Println("🚀 Loading deployments...")
		o.deployment, err = tui.SelectDeployment(o.k8sClient.GetClientset(), o.k8sClient.GetNamespace())
		if err != nil {
			return fmt.Errorf("failed to select deployment: %v", err)
		}
	}

	// 2. Select container
	var selectedContainer tui.ContainerInfo
	if o.container == "" {
		fmt.Println("📦 Loading containers...")
		containers, err := o.k8sClient.GetContainers(o.deployment)
		if err != nil {
			return err
		}

		// Convert k8s.ContainerInfo to tui.ContainerInfo
		tuiContainers := make([]tui.ContainerInfo, len(containers))
		for i, c := range containers {
			tuiContainers[i] = tui.ContainerInfo{
				Name:  c.Name,
				Image: c.Image,
				Index: c.Index,
			}
		}

		selectedContainer, err = tui.SelectContainer(tuiContainers)
		if err != nil {
			return fmt.Errorf("failed to select container: %v", err)
		}
		o.container = selectedContainer.Name
	} else {
		// Get container info if container name is specified
		fmt.Printf("🚀 Using specified deployment: %s\n", o.deployment)
		fmt.Printf("📦 Using specified container: %s\n", o.container)

		currentImage, err := o.k8sClient.GetCurrentImage(o.deployment, o.container)
		if err != nil {
			return fmt.Errorf("container %s not found in deployment %s: %v", o.container, o.deployment, err)
		}
		selectedContainer = tui.ContainerInfo{
			Name:  o.container,
			Image: currentImage,
		}
	}

	// 3. Select image tag
	fmt.Println("🏷️  Loading image tags...")

	// Get tag list
	tagInfos, err := o.registry.ListTagsWithInfo(selectedContainer.Image)
	if err != nil {
		fmt.Printf("⚠️  Failed to fetch tags: %v\n", err)
		fmt.Println("📝 Falling back to manual input...")

		// Manual input if tag fetching fails
		o.image, err = tui.InputCustomImage(selectedContainer.Image)
		if err != nil {
			return fmt.Errorf("failed to input image: %v", err)
		}
	} else {
		// Convert registry.TagInfo to tui.TagInfo
		tuiTagInfos := make([]tui.TagInfo, len(tagInfos))
		for i, t := range tagInfos {
			tuiTagInfos[i] = tui.TagInfo{
				Tag:       t.Tag,
				CreatedAt: t.CreatedAt,
			}
		}

		// Tag selection TUI
		o.image, err = tui.SelectImageTagWithTimestamp(selectedContainer.Image, tuiTagInfos)
		if err != nil {
			return fmt.Errorf("failed to select image tag: %v", err)
		}
	}

	// Confirmation message
	fmt.Printf("\n✅ Selected:\n")
	fmt.Printf("   Deployment: %s\n", o.deployment)
	fmt.Printf("   Container:  %s\n", o.container)
	fmt.Printf("   New Image:  %s\n", o.image)
	fmt.Println()

	return nil
}

func (o *SetImageOptions) savePreviousImage() error {
	var err error
	o.previousImage, err = o.k8sClient.GetCurrentImage(o.deployment, o.container)
	return err
}

func (o *SetImageOptions) RunWithPatch() error {
	// Save previous image before update
	if err := o.savePreviousImage(); err != nil {
		return err
	}

	// Update the image
	err := o.k8sClient.UpdateContainerImage(o.deployment, o.container, o.image)
	if err != nil {
		return err
	}

	fmt.Printf("deployment.apps/%s container %s image updated to %s\n",
		o.deployment, o.container, o.image)

	// Monitor pod status in watch mode
	if o.watchMode {
		return o.watchPodsAndRollbackIfNeeded()
	}

	return nil
}

func (o *SetImageOptions) Run() error {
	// List only mode
	if o.listOnly {
		return o.listContainers()
	}

	// Interactive mode already handled in Complete() method

	// Update image
	return o.RunWithPatch()
}

func (o *SetImageOptions) watchPodsAndRollbackIfNeeded() error {
	fmt.Printf("\n🔍 Watching deployment %s for %v...\n", o.deployment, o.watchTimeout)

	err := o.k8sClient.WatchDeployment(o.deployment, o.watchTimeout)
	if err != nil {
		fmt.Printf("❌ Error watching deployment: %v\n", err)
		return o.rollbackDeployment()
	}

	fmt.Printf("✅ Deployment %s is ready!\n", o.deployment)
	return nil
}

func (o *SetImageOptions) rollbackDeployment() error {
	if o.previousImage == "" {
		return fmt.Errorf("no previous image saved for rollback")
	}

	// Show confirmation screen in interactive mode
	if o.deployment != "" && o.container != "" && o.image != "" {
		message := fmt.Sprintf("Deployment failed. Rollback container %s to %s?", o.container, o.previousImage)
		if !tui.ConfirmRollback(message) {
			fmt.Println("Rollback cancelled by user.")
			return fmt.Errorf("rollback cancelled")
		}
	}

	fmt.Printf("\n🔄 Rolling back container %s to previous image: %s\n", o.container, o.previousImage)

	err := o.k8sClient.UpdateContainerImage(o.deployment, o.container, o.previousImage)
	if err != nil {
		return fmt.Errorf("failed to rollback deployment: %v", err)
	}

	fmt.Printf("✅ Rollback completed! Container %s image reverted to %s\n", o.container, o.previousImage)
	return nil
}

func NewRootCommand() *cobra.Command {
	opts := NewSetImageOptions()

	cmd := &cobra.Command{
		Use:   "kubectl-setimg DEPLOYMENT [CONTAINER=IMAGE]",
		Short: "Update container image in deployment with interactive selection",
		Long: `Update container image in deployment with interactive selection and multi-registry support.

You can use this command in multiple ways:
1. Interactive selection: kubectl setimg (automatically provides selection when arguments are omitted)
2. Direct mode: kubectl setimg my-app web=nginx:1.21.1
3. List containers: kubectl setimg my-app --list
4. With automatic rollback: kubectl setimg my-app web=nginx:1.21.1 --watch`,
		Example: `  # Direct mode
  kubectl setimg my-app web=nginx:1.21.1
  
  # Interactive selection - automatically triggered when arguments are missing
  kubectl setimg                    # Select deployment, container, and image
  kubectl setimg my-app             # Select container and image
  kubectl setimg my-app web         # Select image only
  
  # List containers only
  kubectl setimg my-app --list
  kubectl setimg my-app -l
  
  # Update with automatic rollback on failure
  kubectl setimg --watch
  kubectl setimg my-app web=nginx:1.21.1 --watch
  kubectl setimg my-app web=nginx:1.21.1 --watch --timeout=10m`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.version {
				fmt.Println(GetVersionInfo())
				return nil
			}
			if err := opts.Complete(args); err != nil {
				return err
			}
			return opts.Run()
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&opts.listOnly, "list", "l", false, "List containers only")
	cmd.Flags().BoolVarP(&opts.watchMode, "watch", "w", false, "Watch deployment and rollback if pods fail to start")
	cmd.Flags().DurationVar(&opts.watchTimeout, "timeout", 5*time.Minute, "Timeout for watching deployment readiness")
	cmd.Flags().BoolVar(&opts.version, "version", false, "Show version information")

	// Add kubectl configuration flags
	opts.configFlags.AddFlags(cmd.Flags())

	return cmd
}

func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
