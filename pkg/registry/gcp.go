package registry

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GCPProvider handles GCR and Artifact Registry
type GCPProvider struct{}

// NewGCPProvider creates a new GCP registry provider
func NewGCPProvider() *GCPProvider {
	return &GCPProvider{}
}

// Name returns the provider name
func (p *GCPProvider) Name() string {
	return "GCP (GCR/Artifact Registry)"
}

// SupportsImage checks if this provider can handle the given image
func (p *GCPProvider) SupportsImage(image string) bool {
	// Parse the image to get the registry
	ref, err := name.ParseReference(image)
	if err != nil {
		return false
	}

	registryHost := ref.Context().Registry.Name()
	return strings.Contains(registryHost, "gcr.io") || strings.Contains(registryHost, "pkg.dev")
}

// ListTags fetches available tags for an image
func (p *GCPProvider) ListTags(image string) ([]string, error) {
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
func (p *GCPProvider) ListTagsWithInfo(image string) ([]TagInfo, error) {
	repo, err := name.NewRepository(image)
	if err != nil {
		// If parsing as repository fails, try to extract repository from full image
		ref, err := name.ParseReference(image)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %v", image, err)
		}
		repo = ref.Context()
	}

	keychain := p.getKeychain()

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

// getKeychain gets authentication keychain for GCP registries
func (p *GCPProvider) getKeychain() authn.Keychain {
	// Try to get auth from Application Default Credentials
	if adcKeychain := p.getADCKeychain(); adcKeychain != nil {
		return adcKeychain
	}

	// Fall back to default keychain
	return authn.DefaultKeychain
}

// getADCKeychain attempts to create a keychain using Application Default Credentials
func (p *GCPProvider) getADCKeychain() authn.Keychain {
	ctx := context.Background()

	tokenSource, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil
	}

	return &adcKeychain{tokenSource: tokenSource}
}

// getTagsWithCreationTime fetches creation time for each tag
func (p *GCPProvider) getTagsWithCreationTime(repo name.Repository, tags []string, keychain authn.Keychain) ([]TagInfo, error) {
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

// adcKeychain implements authn.Keychain using Application Default Credentials
type adcKeychain struct {
	tokenSource oauth2.TokenSource
}

func (a *adcKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	if a.tokenSource == nil {
		return authn.Anonymous, nil
	}

	token, err := a.tokenSource.Token()
	if err != nil {
		return authn.Anonymous, nil
	}

	return &authn.Bearer{Token: token.AccessToken}, nil
}
