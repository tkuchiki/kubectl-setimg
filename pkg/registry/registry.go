package registry

import (
	"fmt"
	"time"
)

// TagInfo holds tag name and creation time for sorting
type TagInfo struct {
	Tag       string
	CreatedAt time.Time
}

// Provider interface for different container registries
type Provider interface {
	// ListTags fetches available tags for an image
	ListTags(image string) ([]string, error)

	// ListTagsWithInfo fetches available tags with creation time info
	ListTagsWithInfo(image string) ([]TagInfo, error)

	// SupportsImage checks if this provider can handle the given image
	SupportsImage(image string) bool

	// Name returns the provider name
	Name() string
}

// Client manages multiple registry providers
type Client struct {
	providers []Provider
}

// NewClient creates a new registry client with all available providers
func NewClient() *Client {
	return &Client{
		providers: []Provider{
			NewAWSProvider(),       // AWS ECR - check first for specific domain matching
			NewGCPProvider(),       // GCP GCR/Artifact Registry
			NewDockerHubProvider(), // Docker Hub - should be last as it's the most generic
			// Future providers can be added here:
			// NewAzureProvider(),
		},
	}
}

// AddProvider adds a custom provider to the client
func (c *Client) AddProvider(provider Provider) {
	c.providers = append(c.providers, provider)
}

// ListTags fetches available tags for an image using the appropriate provider
func (c *Client) ListTags(image string) ([]string, error) {
	provider := c.findProvider(image)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for image: %s", image)
	}

	return provider.ListTags(image)
}

// ListTagsWithInfo fetches available tags with creation time info using the appropriate provider
func (c *Client) ListTagsWithInfo(image string) ([]TagInfo, error) {
	provider := c.findProvider(image)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for image: %s", image)
	}

	return provider.ListTagsWithInfo(image)
}

// findProvider finds the appropriate provider for an image
func (c *Client) findProvider(image string) Provider {
	for _, provider := range c.providers {
		if provider.SupportsImage(image) {
			return provider
		}
	}
	return nil
}

// GetSupportedProviders returns a list of supported provider names
func (c *Client) GetSupportedProviders() []string {
	var names []string
	for _, provider := range c.providers {
		names = append(names, provider.Name())
	}
	return names
}
