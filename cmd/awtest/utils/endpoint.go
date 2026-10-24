package utils

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

// EndpointEnvVar is the environment variable consulted when -endpoint-url is unset.
const EndpointEnvVar = "AWS_ENDPOINT_URL"

// ValidateEndpointURL checks that raw is an absolute http(s) URL with a host
// and returns it with surrounding whitespace trimmed.
func ValidateEndpointURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("invalid -endpoint-url %q: value is empty", raw)
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid -endpoint-url %q: %v", raw, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("invalid -endpoint-url %q: scheme must be http or https", raw)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("invalid -endpoint-url %q: missing host", raw)
	}
	return trimmed, nil
}

// ResolveEndpointURL returns the endpoint to use: the flag value if set, else
// the AWS_ENDPOINT_URL environment variable, else "" (no override). The chosen
// value is validated and errors name their source.
func ResolveEndpointURL(flagValue string, getenv func(string) string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		v, err := ValidateEndpointURL(flagValue)
		if err != nil {
			return "", fmt.Errorf("flag -endpoint-url: %w", err)
		}
		return v, nil
	}
	if getenv != nil {
		if env := strings.TrimSpace(getenv(EndpointEnvVar)); env != "" {
			v, err := ValidateEndpointURL(env)
			if err != nil {
				return "", fmt.Errorf("environment variable %s: %w", EndpointEnvVar, err)
			}
			return v, nil
		}
	}
	return "", nil
}

// ApplyEndpoint points sess at a custom AWS-compatible endpoint. It is a no-op
// when endpointURL is empty.
//
// Every client built from the session (sess.Copy, svc.New(sess, &aws.Config{Region: r}))
// inherits Config.Endpoint and S3ForcePathStyle, so regional clients are covered.
// Services must therefore never construct their own sessions; derive them with
// sess.Copy instead. S3 is switched to path-style because virtual-hosted-style
// addressing would resolve bucket.<host> names that a local endpoint can't serve.
func ApplyEndpoint(sess *session.Session, endpointURL string) {
	if endpointURL == "" {
		return
	}
	sess.Config.Endpoint = aws.String(endpointURL)
	sess.Config.S3ForcePathStyle = aws.Bool(true)
}
