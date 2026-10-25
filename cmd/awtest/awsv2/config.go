package awsv2

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
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
	// MaxAttempts is the total number of attempts (first try plus retries)
	// the SDK makes per request. 0 selects DefaultMaxAttempts.
	MaxAttempts int
	// RetryBackoff replaces the SDK's default jittered exponential backoff
	// between attempts. It exists for tests (a zero-delay delayer); nil keeps
	// the SDK default.
	RetryBackoff retry.BackoffDelayer
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
// Retries: the SDK retries up to Options.MaxAttempts (DefaultMaxAttempts, 4
// total attempts, for v1 parity) with its standard backoff, except for the
// throttle codes in AwtestThrottleCodes, which the SDK never retries because
// awtest's own scan backoff handles them. The retryer is installed on the
// loader, so it also applies to the STS client the SDK builds internally for
// assume-role and web-identity credentials.
//
// Credential-chain isolation: while the SDK loader runs,
// AWS_IGNORE_CONFIGURED_ENDPOINT_URLS=true is set in the process environment
// so the internal STS client of the credential chain cannot be redirected by
// AWS_ENDPOINT_URL, AWS_ENDPOINT_URL_STS or shared-config endpoint settings.
// The variable is process-global while held (a package mutex serialises
// LoadConfig calls, but other goroutines reading the environment would see
// it), so call LoadConfig at startup, before worker goroutines. The previous
// value is restored (or the variable unset) on every return path.
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

	if opts.Endpoint != "" {
		if err := validateEndpoint(opts.Endpoint); err != nil {
			return aws.Config{}, err
		}
	}
	loadOpts = append(loadOpts, config.WithRetryer(newRetryer(opts)))
	loadOpts = append(loadOpts, endpointLoadOpts(opts.Endpoint)...)

	cfg, err := loadDefaultConfigIsolated(ctx, loadOpts)
	if err != nil {
		return aws.Config{}, err
	}
	clearLegacyResolvers(&cfg)
	// Surface the policy on the config; clients cap their retryer to it.
	cfg.RetryMaxAttempts = maxAttempts(opts)

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
