package replay

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)

// NewConfig is the aws-sdk-go-v2 counterpart of NewSession: fake static
// credentials, srv as the only endpoint, and an HTTP client that can only
// reach srv. Shared config, profiles, credential environment variables and
// ambient AWS_ENDPOINT_URL settings are not consulted.
//
// It lives apart from NewSession so that the v1 session code (which sets an
// HTTPClient) stays in a file the v2 no-bypass guard does not treat as v2 code.
func NewConfig(srv *Server, region string) (aws.Config, error) {
	return awsv2.LoadConfig(context.Background(), awsv2.Options{
		Region:          region,
		AccessKeyID:     FakeAccessKeyID,
		SecretAccessKey: FakeSecretAccessKey,
		Endpoint:        srv.URL,
		Transport:       RestrictedTransport(strings.TrimPrefix(srv.URL, "http://")),
		HTTPTimeout:     60 * time.Second,
		Quiet:           true,
	})
}
