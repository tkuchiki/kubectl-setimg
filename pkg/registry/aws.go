package registry

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// AWSProvider handles Amazon ECR registry
type AWSProvider struct{}

// NewAWSProvider creates a new AWS ECR registry provider
func NewAWSProvider() *AWSProvider {
	return &AWSProvider{}
}

// Name returns the provider name
func (p *AWSProvider) Name() string {
	return "AWS ECR"
}

// SupportsImage checks if this provider can handle the given image
func (p *AWSProvider) SupportsImage(image string) bool {
	// AWS ECR uses format: <account-id>.dkr.ecr.<region>.amazonaws.com
	return strings.Contains(image, ".dkr.ecr.") && strings.Contains(image, ".amazonaws.com")
}

// ListTags fetches available tags for an image
func (p *AWSProvider) ListTags(image string) ([]string, error) {
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
func (p *AWSProvider) ListTagsWithInfo(image string) ([]TagInfo, error) {
	// Parse the ECR image URL to extract region and repository
	region, repository, err := p.parseECRImage(image)
	if err != nil {
		return nil, err
	}

	// Create AWS config with the extracted region
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Create ECR service client
	svc := ecr.NewFromConfig(cfg)

	// Call DescribeImages to get tags and timestamps
	input := &ecr.DescribeImagesInput{
		RepositoryName: aws.String(repository),
		MaxResults:     aws.Int32(100), // Limit to 100 images for performance
	}

	// Handle pagination if needed
	var allImageDetails []types.ImageDetail
	paginator := ecr.NewDescribeImagesPaginator(svc, input)

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe images for repository %s: %v", repository, err)
		}
		allImageDetails = append(allImageDetails, result.ImageDetails...)

		// Limit total results for performance
		if len(allImageDetails) >= 100 {
			break
		}
	}

	var tagInfos []TagInfo
	for _, imageDetail := range allImageDetails {
		// Skip images without tags
		if len(imageDetail.ImageTags) == 0 {
			continue
		}

		// Process each tag for this image
		for _, tag := range imageDetail.ImageTags {
			if tag == "" {
				continue
			}

			createdAt := time.Now() // Default fallback
			if imageDetail.ImagePushedAt != nil {
				createdAt = *imageDetail.ImagePushedAt
			}

			tagInfos = append(tagInfos, TagInfo{
				Tag:       tag,
				CreatedAt: createdAt,
			})
		}
	}

	if len(tagInfos) == 0 {
		return nil, fmt.Errorf("no tagged images found in ECR repository %s", repository)
	}

	// Sort by creation time (newest first)
	sort.Slice(tagInfos, func(i, j int) bool {
		return tagInfos[i].CreatedAt.After(tagInfos[j].CreatedAt)
	})

	// Limit to 20 tags for performance
	if len(tagInfos) > 20 {
		tagInfos = tagInfos[:20]
	}

	return tagInfos, nil
}

// parseECRImage parses an ECR image URL and extracts the region and repository name
func (p *AWSProvider) parseECRImage(image string) (region, repository string, err error) {
	// ECR image format: <account-id>.dkr.ecr.<region>.amazonaws.com/<repository>:<tag>
	// Example: 123456789012.dkr.ecr.us-west-2.amazonaws.com/my-repo:latest

	// Regular expression to match ECR URLs
	ecrRegex := regexp.MustCompile(`^(\d+)\.dkr\.ecr\.([^.]+)\.amazonaws\.com\/([^:]+)(?::(.+))?$`)

	matches := ecrRegex.FindStringSubmatch(image)
	if len(matches) < 4 {
		return "", "", fmt.Errorf("invalid ECR image format: %s. Expected format: <account-id>.dkr.ecr.<region>.amazonaws.com/<repository>[:tag]", image)
	}

	// Extract components
	// matches[1] = account-id (not needed for our purposes)
	// matches[2] = region
	// matches[3] = repository
	// matches[4] = tag (optional)

	region = matches[2]
	repository = matches[3]

	if region == "" || repository == "" {
		return "", "", fmt.Errorf("failed to extract region or repository from ECR image: %s", image)
	}

	return region, repository, nil
}

// validateECRAccess validates that we can access the ECR registry
func (p *AWSProvider) validateECRAccess(region, repository string) error {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	svc := ecr.NewFromConfig(cfg)

	// Try to describe the repository to validate access
	_, err = svc.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repository},
	})

	if err != nil {
		return fmt.Errorf("failed to access ECR repository %s in region %s: %v. Please check your AWS credentials and permissions", repository, region, err)
	}

	return nil
}
