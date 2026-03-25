package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

func LoadConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(profile),
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err == nil {
		return cfg, nil
	}

	fallbackCfg, fallbackErr := loadConfigWithLoginSessionFallback(ctx, profile, region)
	if fallbackErr == nil {
		return fallbackCfg, nil
	}
	if errors.Is(fallbackErr, errLoginSessionFallbackNotApplicable) {
		return aws.Config{}, err
	}

	return aws.Config{}, fmt.Errorf("%w; login_session fallback failed: %v", err, fallbackErr)
}
