package replay_test

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/replay"
)

func newReplayConfig(t *testing.T, srv *replay.Server) aws.Config {
	t.Helper()
	cfg, err := replay.NewConfig(srv, "us-west-2")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// tripwire listens on a loopback port and counts connections. Pointing an
// ambient endpoint at it shows whether anything dials something other than the
// replay server.
func tripwire(t *testing.T) (addr string, accepted *atomic.Int64) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted = new(atomic.Int64)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			c.Close()
		}
	}()
	t.Cleanup(func() { l.Close() })
	return l.Addr().String(), accepted
}

func TestReplayV2ClientReachesOnlyReplayServer(t *testing.T) {
	guard := &failTransport{}
	orig := http.DefaultTransport
	http.DefaultTransport = guard
	t.Cleanup(func() { http.DefaultTransport = orig })

	trapAddr, trapped := tripwire(t)
	t.Setenv("AWS_ENDPOINT_URL", "http://"+trapAddr)
	t.Setenv("AWS_ENDPOINT_URL_STS", "http://"+trapAddr)
	t.Setenv("AWS_PROFILE", "does-not-exist")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAREALLOOKINGKEY00")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "real")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	srv := startSample(t)
	cfg := newReplayConfig(t, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := awsv2.NewRegionalClient(cfg, "us-west-2", sts.NewFromConfig)
	out, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatalf("GetCallerIdentity via replay: %v", err)
	}
	if aws.ToString(out.Arn) == "" {
		t.Error("replayed GetCallerIdentity returned no ARN")
	}
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessKeyID != replay.FakeAccessKeyID {
		t.Errorf("replay config used credentials %q, want the fake replay key", creds.AccessKeyID)
	}
	if srv.Requests() == 0 {
		t.Error("replay server saw no requests")
	}
	if n := trapped.Load(); n != 0 {
		t.Errorf("%d connection(s) reached the ambient endpoint", n)
	}
	if len(guard.seen) > 0 {
		t.Errorf("requests used http.DefaultTransport: %v", guard.seen)
	}
}

// Even a client that ignores awsv2 and lets AWS_ENDPOINT_URL_STS redirect it
// cannot leave the replay server: the config's HTTP client refuses to dial
// anything else, so the connection is refused before it reaches the network.
func TestReplayV2ConfigRefusesOtherHosts(t *testing.T) {
	trapAddr, trapped := tripwire(t)
	t.Setenv("AWS_ENDPOINT_URL_STS", "http://"+trapAddr)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAREALLOOKINGKEY00")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "real")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	srv := startSample(t)
	cfg := newReplayConfig(t, srv)

	// Worst case: a config whose sources honour the ambient endpoint.
	hostile, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigSources = hostile.ConfigSources

	cfg.RetryMaxAttempts = 1
	client := sts.NewFromConfig(cfg) // deliberately not awsv2.NewRegionalClient
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err == nil {
		t.Fatal("request to the ambient endpoint unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "refusing to connect") {
		t.Errorf("error does not show the restricted dialer refused the connection: %v", err)
	}
	if n := trapped.Load(); n != 0 {
		t.Errorf("%d connection(s) reached the tripwire", n)
	}
}
