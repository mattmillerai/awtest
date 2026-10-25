package awsv2

import (
	"fmt"
	"net/url"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// ApplyEndpoint sets cfg.BaseEndpoint to endpointURL after validating it.
//
// An empty endpointURL clears cfg.BaseEndpoint, so that an ambient
// AWS_ENDPOINT_URL or shared-config endpoint_url is ignored. A non-empty value
// must be an http or https URL with a host, otherwise an error is returned and
// cfg is left unchanged.
func ApplyEndpoint(cfg *aws.Config, endpointURL string) error {
	if endpointURL == "" {
		cfg.BaseEndpoint = nil
		return nil
	}
	if err := validateEndpoint(endpointURL); err != nil {
		return err
	}
	cfg.BaseEndpoint = aws.String(endpointURL)
	return nil
}

func validateEndpoint(endpointURL string) error {
	u, err := url.Parse(endpointURL)
	if err != nil {
		return fmt.Errorf("invalid endpoint %q: %w", endpointURL, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("invalid endpoint %q: must be an http(s) URL with a host", endpointURL)
	}
	return nil
}

// ClientParams are the per-client settings applied by NewClient.
type ClientParams struct {
	// Region overrides the region in the aws.Config when non-empty.
	Region string
	// Endpoint is the explicit endpoint URL, or "" for the service default.
	Endpoint string
}

// NewClient builds a v2 service client. It is the only sanctioned way to
// construct one.
//
// ctor is a service package's NewFromConfig. NewClient passes it a functional
// option that always assigns Options.BaseEndpoint (to p.Endpoint, or nil when
// empty). This is required because each v2 client resolves the service
// specific AWS_ENDPOINT_URL_<SERVICE> variable and shared-config [services]
// endpoint_url inside NewFromConfig, before functional options run; assigning
// the field afterwards defeats those ambient settings. cfg.BaseEndpoint is
// overridden consistently as well.
//
// The option also sets Options.Region when p.Region is non-empty, and, when an
// endpoint is given and Options has a bool UsePathStyle field, sets it true
// (S3 virtual-hosted addressing cannot work against a local endpoint; this
// mirrors v1 S3ForcePathStyle).
//
// NewClient panics if Options has no settable BaseEndpoint *string field:
// failing closed is preferable to building a client whose endpoint cannot be
// controlled.
func NewClient[C any, O any](cfg aws.Config, p ClientParams, ctor func(aws.Config, ...func(*O)) *C) *C {
	cfg.BaseEndpoint = nil
	if p.Endpoint != "" {
		cfg.BaseEndpoint = aws.String(p.Endpoint)
	}
	return ctor(cfg, func(o *O) { applyParams(o, p) })
}

// NewRegionalClient builds a v2 service client in region from a config that
// LoadConfig already validated. It is NewClient with the config's own explicit
// endpoint (cfg.BaseEndpoint, or none) carried over, so a service that
// receives the shared aws.Config keeps the user's --endpoint while ambient
// service endpoints stay ignored.
func NewRegionalClient[C any, O any](cfg aws.Config, region string, ctor func(aws.Config, ...func(*O)) *C) *C {
	return NewClient(cfg, ClientParams{Region: region, Endpoint: aws.ToString(cfg.BaseEndpoint)}, ctor)
}

// applyParams applies p to the Options struct pointed to by opts using
// reflection. It panics if opts has no settable BaseEndpoint *string field.
func applyParams(opts any, p ClientParams) {
	v := reflect.ValueOf(opts)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("awsv2: client options must be a non-nil struct pointer, got %T", opts))
	}
	s := v.Elem()

	be := s.FieldByName("BaseEndpoint")
	if !be.IsValid() || !be.CanSet() || be.Type() != reflect.TypeOf((*string)(nil)) {
		panic(fmt.Sprintf("awsv2: %T has no settable BaseEndpoint *string field; refusing to build a client whose endpoint cannot be controlled", opts))
	}
	if p.Endpoint != "" {
		be.Set(reflect.ValueOf(aws.String(p.Endpoint)))
	} else {
		be.Set(reflect.Zero(be.Type()))
	}

	if p.Region != "" {
		if r := s.FieldByName("Region"); r.IsValid() && r.CanSet() && r.Kind() == reflect.String {
			r.SetString(p.Region)
		}
	}

	if p.Endpoint != "" {
		if ps := s.FieldByName("UsePathStyle"); ps.IsValid() && ps.CanSet() && ps.Kind() == reflect.Bool {
			ps.SetBool(true)
		}
	}
}
