# Registry Package

The registry package provides a multi-provider interface for interacting with different container registries.

## Supported Providers

### 1. AWS ECR (Amazon Elastic Container Registry)

**Image Format**: `<account-id>.dkr.ecr.<region>.amazonaws.com/<repository>[:tag]`

**Example**: `123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:v1.0.0`

**Authentication**:
The AWS provider uses the AWS SDK v2 default credential chain:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role for EC2 instances
4. IAM role for containers (ECS/EKS)

**Required IAM Permissions**:
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ecr:DescribeImages",
                "ecr:DescribeRepositories"
            ],
            "Resource": "*"
        }
    ]
}
```

**Setup Examples**:

*Using AWS CLI configuration:*
```bash
aws configure
# Enter your access key, secret key, region, and output format
```

*Using environment variables:*
```bash
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export AWS_DEFAULT_REGION=us-west-2
```

*Using IAM role (for EC2/EKS):*
No additional setup needed if running on EC2/EKS with appropriate IAM role attached.

### 2. GCP (Google Container Registry / Artifact Registry)

**Image Formats**:
- GCR: `gcr.io/<project-id>/<repository>[:tag]`
- Artifact Registry: `<region>-docker.pkg.dev/<project-id>/<repository>/<image>[:tag]`

**Authentication**: Uses Application Default Credentials (ADC)

### 3. Docker Hub

**Image Format**: `[docker.io/]<repository>[:tag]` or `<repository>[:tag]`

**Authentication**: Uses default Docker authentication

## Usage Example

```go
package main

import (
    "fmt"
    "github.com/tkuchiki/kubectl-setimg/pkg/registry"
)

func main() {
    client := registry.NewClient()
    
    // AWS ECR
    tags, err := client.ListTagsWithInfo("123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    for _, tag := range tags {
        fmt.Printf("Tag: %s, Created: %s\n", tag.Tag, tag.CreatedAt)
    }
}
```

## Adding New Providers

To add a new registry provider:

1. Create a new file in the `pkg/registry/` directory
2. Implement the `Provider` interface:
   ```go
   type Provider interface {
       ListTags(image string) ([]string, error)
       ListTagsWithInfo(image string) ([]TagInfo, error)
       SupportsImage(image string) bool
       Name() string
   }
   ```
3. Add the provider to `NewClient()` in `registry.go`

## Error Handling

The registry package handles various error conditions:

- **Authentication failures**: Returns descriptive error messages
- **Network issues**: Includes timeout and retry logic where appropriate
- **Invalid image formats**: Validates image URLs before processing
- **Empty repositories**: Gracefully handles repositories with no tags

## Performance Considerations

- Results are limited to 20 tags by default for performance
- AWS ECR uses pagination to handle large repositories
- Concurrent tag fetching for timestamp information (where supported)
- Caching can be implemented at the client level if needed