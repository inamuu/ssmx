package aws

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/logincreds"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/signin"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

var errLoginSessionFallbackNotApplicable = errors.New("login_session fallback not applicable")

type sharedProfileSettings struct {
	Region             string
	RoleARN            string
	SourceProfileName  string
	RoleSessionName    string
	ExternalID         string
	MFASerial          string
	LoginSession       string
	RoleDurationSecond time.Duration
}

type loginFallbackPlan struct {
	Region             string
	LoginSession       string
	RoleARN            string
	RoleSessionName    string
	ExternalID         string
	MFASerial          string
	RoleDurationSecond time.Duration
}

func loadConfigWithLoginSessionFallback(ctx context.Context, profile, region string) (aws.Config, error) {
	plan, err := resolveLoginFallbackPlan(profile)
	if err != nil {
		return aws.Config{}, err
	}

	configRegion := firstNonEmpty(region, plan.Region)
	baseCfg, err := loadBaseConfig(ctx, configRegion)
	if err != nil {
		return aws.Config{}, err
	}

	provider, err := newLoginCredentialsProvider(baseCfg, plan.LoginSession)
	if err != nil {
		return aws.Config{}, err
	}

	cfg := baseCfg.Copy()
	cfg.Credentials = aws.NewCredentialsCache(provider)

	if plan.RoleARN == "" {
		return cfg, nil
	}
	if plan.MFASerial != "" {
		return aws.Config{}, fmt.Errorf("profile %q requires MFA for assumed role %s", profile, plan.RoleARN)
	}

	stsClient := sts.NewFromConfig(cfg)
	assumeRoleProvider := stscreds.NewAssumeRoleProvider(stsClient, plan.RoleARN, func(o *stscreds.AssumeRoleOptions) {
		if plan.RoleSessionName != "" {
			o.RoleSessionName = plan.RoleSessionName
		}
		if plan.ExternalID != "" {
			o.ExternalID = aws.String(plan.ExternalID)
		}
		if plan.RoleDurationSecond > 0 {
			o.Duration = plan.RoleDurationSecond
		}
	})

	cfg.Credentials = aws.NewCredentialsCache(assumeRoleProvider)
	return cfg, nil
}

func resolveLoginFallbackPlan(profile string) (loginFallbackPlan, error) {
	profileSettings, err := loadSharedProfileSettings(profile)
	if err != nil {
		return loginFallbackPlan{}, err
	}

	if profileSettings.SourceProfileName != "" && profileSettings.RoleARN != "" {
		sourceSettings, err := loadSharedProfileSettings(profileSettings.SourceProfileName)
		if err != nil {
			return loginFallbackPlan{}, err
		}
		if sourceSettings.LoginSession == "" {
			return loginFallbackPlan{}, errLoginSessionFallbackNotApplicable
		}

		return loginFallbackPlan{
			Region:             firstNonEmpty(profileSettings.Region, sourceSettings.Region),
			LoginSession:       sourceSettings.LoginSession,
			RoleARN:            profileSettings.RoleARN,
			RoleSessionName:    profileSettings.RoleSessionName,
			ExternalID:         profileSettings.ExternalID,
			MFASerial:          profileSettings.MFASerial,
			RoleDurationSecond: profileSettings.RoleDurationSecond,
		}, nil
	}

	if profileSettings.LoginSession == "" {
		return loginFallbackPlan{}, errLoginSessionFallbackNotApplicable
	}

	return loginFallbackPlan{
		Region:             profileSettings.Region,
		LoginSession:       profileSettings.LoginSession,
		RoleARN:            profileSettings.RoleARN,
		RoleSessionName:    profileSettings.RoleSessionName,
		ExternalID:         profileSettings.ExternalID,
		MFASerial:          profileSettings.MFASerial,
		RoleDurationSecond: profileSettings.RoleDurationSecond,
	}, nil
}

func loadBaseConfig(ctx context.Context, region string) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithSharedConfigFiles([]string{}),
		awsconfig.WithSharedCredentialsFiles([]string{}),
	}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

func newLoginCredentialsProvider(cfg aws.Config, session string) (aws.CredentialsProvider, error) {
	cacheDir := os.Getenv("AWS_LOGIN_CACHE_DIRECTORY")
	tokenPath, err := logincreds.StandardCachedTokenFilepath(session, cacheDir)
	if err != nil {
		return nil, err
	}

	return logincreds.New(signin.NewFromConfig(cfg), tokenPath), nil
}

func loadSharedProfileSettings(profile string) (sharedProfileSettings, error) {
	configPath := os.Getenv("AWS_CONFIG_FILE")
	if configPath == "" {
		configPath = awsconfig.DefaultSharedConfigFilename()
	}
	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return sharedProfileSettings{}, errLoginSessionFallbackNotApplicable
		}
		return sharedProfileSettings{}, err
	}
	defer file.Close()

	sectionNames := profileSectionNames(profile)
	settings := sharedProfileSettings{}
	inTargetSection := false
	foundSection := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionName := strings.TrimSpace(line[1 : len(line)-1])
			inTargetSection = false
			for _, candidate := range sectionNames {
				if sectionName == candidate {
					inTargetSection = true
					foundSection = true
					break
				}
			}
			continue
		}

		if !inTargetSection {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)

		switch key {
		case "region":
			settings.Region = value
		case "role_arn":
			settings.RoleARN = value
		case "source_profile":
			settings.SourceProfileName = value
		case "role_session_name":
			settings.RoleSessionName = value
		case "external_id":
			settings.ExternalID = value
		case "mfa_serial":
			settings.MFASerial = value
		case "login_session":
			settings.LoginSession = value
		case "duration_seconds":
			seconds, err := strconv.Atoi(value)
			if err != nil {
				return sharedProfileSettings{}, fmt.Errorf("invalid duration_seconds for profile %q: %w", profile, err)
			}
			settings.RoleDurationSecond = time.Duration(seconds) * time.Second
		}
	}
	if err := scanner.Err(); err != nil {
		return sharedProfileSettings{}, err
	}
	if !foundSection {
		return sharedProfileSettings{}, errLoginSessionFallbackNotApplicable
	}

	return settings, nil
}

func profileSectionNames(profile string) []string {
	if profile == "default" {
		return []string{"default"}
	}

	return []string{
		"profile " + profile,
		profile,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
