package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

// Client wraps Kubernetes operations
type Client struct {
	clientset kubernetes.Interface
	namespace string
}

// ContainerInfo represents container information
type ContainerInfo struct {
	Name  string
	Image string
	Index int
}

// NewClient creates a new Kubernetes client
func NewClient(configFlags *genericclioptions.ConfigFlags) (*Client, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	namespace, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, err
	}

	return &Client{
		clientset: clientset,
		namespace: namespace,
	}, nil
}

// GetClientset returns the underlying Kubernetes clientset
func (c *Client) GetClientset() kubernetes.Interface {
	return c.clientset
}

// GetNamespace returns the current namespace
func (c *Client) GetNamespace() string {
	return c.namespace
}

// GetContainers returns containers in a deployment
func (c *Client) GetContainers(deploymentName string) ([]ContainerInfo, error) {
	ctx := context.Background()

	deployment, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s: %v", deploymentName, err)
	}

	var containers []ContainerInfo
	for i, container := range deployment.Spec.Template.Spec.Containers {
		containers = append(containers, ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
			Index: i,
		})
	}

	return containers, nil
}

// GetCurrentImage returns the current image for a container in a deployment
func (c *Client) GetCurrentImage(deploymentName, containerName string) (string, error) {
	ctx := context.Background()

	deployment, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get deployment %s: %v", deploymentName, err)
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return container.Image, nil
		}
	}

	return "", fmt.Errorf("container %s not found in deployment %s", containerName, deploymentName)
}

// UpdateContainerImage updates a container image in a deployment using strategic merge patch
func (c *Client) UpdateContainerImage(deploymentName, containerName, newImage string) error {
	ctx := context.Background()

	// Create strategic merge patch
	patch := fmt.Sprintf(`{
        "spec": {
            "template": {
                "spec": {
                    "containers": [
                        {
                            "name": "%s",
                            "image": "%s"
                        }
                    ]
                }
            }
        }
    }`, containerName, newImage)

	_, err := c.clientset.AppsV1().Deployments(c.namespace).Patch(
		ctx,
		deploymentName,
		types.StrategicMergePatchType,
		[]byte(patch),
		metav1.PatchOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to patch deployment: %v", err)
	}

	return nil
}

// CheckDeploymentReadiness checks if a deployment is ready
func (c *Client) CheckDeploymentReadiness(deploymentName string) (bool, error) {
	ctx := context.Background()

	deployment, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// Check if all replicas are available
	if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas &&
		deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas {
		return true, nil
	}

	// Check for failed pods
	labelSelector := fmt.Sprintf("app=%s", deploymentName)
	pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return false, err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == "Failed" {
			return false, fmt.Errorf("pod %s failed", pod.Name)
		}

		// Check for CrashLoopBackOff, ImagePullError, etc.
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.RestartCount > 3 {
				return false, fmt.Errorf("container %s in pod %s is restarting too frequently", containerStatus.Name, pod.Name)
			}

			if containerStatus.State.Waiting != nil {
				reason := containerStatus.State.Waiting.Reason
				if reason == "ImagePullBackOff" || reason == "CrashLoopBackOff" || reason == "ErrImagePull" {
					return false, fmt.Errorf("container %s in pod %s has problem: %s", containerStatus.Name, pod.Name, reason)
				}
			}
		}
	}

	return false, nil
}

// WatchDeployment monitors deployment readiness with timeout
func (c *Client) WatchDeployment(deploymentName string, timeout time.Duration) error {
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timeout: deployment didn't become ready within %v", timeout)

		case <-ticker.C:
			ready, err := c.CheckDeploymentReadiness(deploymentName)
			if err != nil {
				return err
			}

			if ready {
				return nil
			}
		}
	}
}
