package registry

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// DockerHubProvider handles Docker Hub registry
type DockerHubProvider struct{}

// NewDockerHubProvider creates a new Docker Hub registry provider
func NewDockerHubProvider() *DockerHubProvider {
	return &DockerHubProvider{}
}

// Name returns the provider name
func (p *DockerHubProvider) Name() string {
	return "Docker Hub"
}

// SupportsImage checks if this provider can handle the given image
func (p *DockerHubProvider) SupportsImage(image string) bool {
	// Parse the image to get the registry
	ref, err := name.ParseReference(image)
	if err != nil {
		return false
	}

	registryHost := ref.Context().Registry.Name()
	// Docker Hub uses "index.docker.io" or "docker.io" or no registry (default)
	return registryHost == "index.docker.io" ||
		registryHost == "docker.io" ||
		registryHost == name.DefaultRegistry ||
		!strings.Contains(image, "/") || // Simple image name like "nginx"
		(strings.Count(image, "/") == 1 && !strings.Contains(image, ".")) // Format like "library/nginx"
}

// ListTags fetches available tags for an image
func (p *DockerHubProvider) ListTags(image string) ([]string, error) {
	tagInfos, err := p.ListTagsWithInfo(image)
	if err != nil {
		return nil, err
	}

	tags := make([]string, len(tagInfos))
	for i, tagInfo := range tagInfos {
		tags[i] = tagInfo.Tag
	}

	return tags, nil
}

// ListTagsWithInfo fetches available tags with creation time info
func (p *DockerHubProvider) ListTagsWithInfo(image string) ([]TagInfo, error) {
	repo, err := name.NewRepository(image)
	if err != nil {
		// If parsing as repository fails, try to extract repository from full image
		ref, err := name.ParseReference(image)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %v", image, err)
		}
		repo = ref.Context()
	}

	keychain := authn.DefaultKeychain

	tags, err := remote.List(repo, remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, fmt.Errorf("failed to list tags for %s: %v", repo.String(), err)
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags found for image %s", repo.String())
	}

	tagInfos, err := p.getTagsWithCreationTime(repo, tags, keychain)
	if err != nil {
		// If we can't get creation times, fall back to alphabetical sort and create TagInfo with zero time
		sort.Strings(tags)
		tagInfos = make([]TagInfo, len(tags))
		for i, tag := range tags {
			tagInfos[i] = TagInfo{
				Tag:       tag,
				CreatedAt: time.Time{}, // Zero time indicates no timestamp available
			}
		}
	} else {
		// Sort by creation time (newest first)
		sort.Slice(tagInfos, func(i, j int) bool {
			return tagInfos[i].CreatedAt.After(tagInfos[j].CreatedAt)
		})
	}

	// Limit to 20 tags for performance
	if len(tagInfos) > 20 {
		tagInfos = tagInfos[:20]
	}

	return tagInfos, nil
}

// getTagsWithCreationTime fetches creation time for each tag
func (p *DockerHubProvider) getTagsWithCreationTime(repo name.Repository, tags []string, keychain authn.Keychain) ([]TagInfo, error) {
	var tagInfos []TagInfo

	maxConcurrent := 10
	if len(tags) < maxConcurrent {
		maxConcurrent = len(tags)
	}

	tagsToProcess := tags
	if len(tags) > 50 {
		tagsToProcess = tags[:50] // Limit to first 50 tags for performance
	}

	results := make(chan TagInfo, len(tagsToProcess))
	errors := make(chan error, len(tagsToProcess))

	sem := make(chan struct{}, maxConcurrent)

	for _, tag := range tagsToProcess {
		go func(tag string) {
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			tagRef, err := name.ParseReference(fmt.Sprintf("%s:%s", repo.String(), tag))
			if err != nil {
				errors <- fmt.Errorf("failed to parse tag %s: %v", tag, err)
				return
			}

			// Get image manifest
			img, err := remote.Image(tagRef, remote.WithAuthFromKeychain(keychain))
			if err != nil {
				errors <- fmt.Errorf("failed to get image for tag %s: %v", tag, err)
				return
			}

			// Get config to extract creation time
			config, err := img.ConfigFile()
			if err != nil {
				errors <- fmt.Errorf("failed to get config for tag %s: %v", tag, err)
				return
			}

			createdAt := config.Created.Time
			if createdAt.IsZero() {
				// If creation time is not available, use a default old time
				createdAt = time.Unix(0, 0)
			}

			results <- TagInfo{
				Tag:       tag,
				CreatedAt: createdAt,
			}
		}(tag)
	}

	for i := 0; i < len(tagsToProcess); i++ {
		select {
		case tagInfo := <-results:
			tagInfos = append(tagInfos, tagInfo)
		case err := <-errors:
			// Log error but continue with other tags
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	if len(tagInfos) == 0 {
		return nil, fmt.Errorf("failed to get creation time for any tags")
	}

	return tagInfos, nil
}
