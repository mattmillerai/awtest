package awsv2

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go/logging"
)

// Options are the inputs to LoadConfig. The zero value selects the SDK's
// default credential chain.
type Options struct {
	// Region is the AWS region; empty leaves it to the default chain.
	Region string
	// AccessKeyID and SecretAccessKey select static credentials when both are
	// non-empty. They always win over environment and profile credentials.
	AccessKeyID     string
	SecretAccessKey string
	// SessionToken is used only with static credentials whose AccessKeyID has
	// the "ASIA" prefix (temporary credentials), exactly like v1 main.go.
	SessionToken string
	// Profile is a shared-config profile name, used when static credentials
	// are not given.
	Profile string
	// Endpoint is an explicit http(s) endpoint URL. Empty means the real AWS
	// endpoints; ambient AWS_ENDPOINT_URL settings are never honoured.
	Endpoint string
	// Quiet suppresses all SDK logging, even with Debug.
	Quiet bool
	// Debug logs SDK retry attempts to Stderr (never requests, responses or
	// signing details, which could leak secrets).
	Debug bool
	// HTTPClient overrides the SDK's HTTP client (used by tests).
	HTTPClient aws.HTTPClient
	// Stderr receives debug logging; os.Stderr when nil.
	Stderr io.Writer
}

// LoadConfig builds an aws.Config following the same three credential paths as
// the v1 code in main.go:
//
//  1. Static keys (AccessKeyID and SecretAccessKey both set). These always
//     win over environment and profile credentials and the shared config and
//     credentials files are not read at all, and even a bogus AWS_PROFILE
//     cannot cause an error (see the comment in the code for why an empty
//     file list alone is not enough).
//  2. A named Profile, via config.WithSharedConfigProfile.
//  3. The SDK default chain.
//
// No retry options are set: the retry policy is an open phase-1 decision, so
// the SDK defaults apply for now.
//
// The returned cfg.BaseEndpoint is exactly the validated Endpoint, or nil;
// AWS_ENDPOINT_URL and shared-config endpoint_url never survive. Note that
// service-specific variables are resolved later by each client, which is why
// clients must be built with NewClient.
func LoadConfig(ctx context.Context, opts Options) (aws.Config, error) {
	var loadOpts []func(*config.LoadOptions) error

	if opts.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(opts.Region))
	}
	if opts.HTTPClient != nil {
		loadOpts = append(loadOpts, config.WithHTTPClient(opts.HTTPClient))
	}

	switch {
	case opts.AccessKeyID != "" && opts.SecretAccessKey != "":
		token := ""
		if strings.HasPrefix(opts.AccessKeyID, "ASIA") {
			token = opts.SessionToken
		}
		// Empty, non-nil file lists stop the SDK reading any shared file, but
		// the SDK still insists that a profile named via AWS_PROFILE exists
		// and fails with SharedConfigProfileNotExistError. Pin the profile to
		// "default" and give it a private, empty config file that defines it,
		// so neither the user's files nor a bogus AWS_PROFILE can matter.
		dir, err := os.MkdirTemp("", "awtest-awsv2-")
		if err != nil {
			return aws.Config{}, err
		}
		defer os.RemoveAll(dir)
		empty := filepath.Join(dir, "config")
		if err := os.WriteFile(empty, []byte("[default]\n"), 0o600); err != nil {
			return aws.Config{}, err
		}
		loadOpts = append(loadOpts,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, token)),
			config.WithSharedConfigFiles([]string{empty}),
			config.WithSharedCredentialsFiles([]string{}),
			config.WithSharedConfigProfile("default"),
		)
	case opts.Profile != "":
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(opts.Profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return aws.Config{}, err
	}

	if err := ApplyEndpoint(&cfg, opts.Endpoint); err != nil {
		return aws.Config{}, err
	}

	if opts.Debug && !opts.Quiet {
		w := opts.Stderr
		if w == nil {
			w = os.Stderr
		}
		// Retries only: LogRequest/LogResponse/LogSigning would leak secrets.
		cfg.ClientLogMode = aws.LogRetries
		cfg.Logger = logging.NewStandardLogger(w)
	} else {
		cfg.ClientLogMode = 0
	}

	return cfg, nil
}
