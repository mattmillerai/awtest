package elasticloadbalancingv2

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

func TestELBv2_Process(t *testing.T) {
	tests := []struct {
		name             string
		call             int
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
	}{
		{
			name: "load balancers",
			call: 0,
			output: []elbtypes.LoadBalancer{{
				LoadBalancerName: aws.String("example-alb"),
				LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/example-alb/50dc6c495c0c9188"),
				DNSName:          aws.String("example-alb-1.us-west-2.elb.amazonaws.com"),
				Type:             elbtypes.LoadBalancerTypeEnumApplication,
				Scheme:           elbtypes.LoadBalancerSchemeEnumInternetFacing,
			}},
			wantLen:          1,
			wantResourceName: "example-alb",
		},
		{name: "load balancers error", call: 0, err: fmt.Errorf("AccessDenied: denied"), wantLen: 1, wantError: true},
		{name: "load balancers empty", call: 0, output: []elbtypes.LoadBalancer{}, wantLen: 0},
		{name: "load balancers wrong type", call: 0, output: "oops", wantLen: 0},
		{
			name: "target groups",
			call: 1,
			output: []elbtypes.TargetGroup{{
				TargetGroupName: aws.String("example-tg"),
				TargetGroupArn:  aws.String("arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/example-tg/73e2d6bc24d8a067"),
				Protocol:        elbtypes.ProtocolEnumHttp,
				Port:            aws.Int32(80),
				VpcId:           aws.String("vpc-0abc1234"),
			}},
			wantLen:          1,
			wantResourceName: "example-tg",
		},
		{name: "target groups error", call: 1, err: fmt.Errorf("AccessDenied: denied"), wantLen: 1, wantError: true},
		{name: "target groups empty", call: 1, output: []elbtypes.TargetGroup{}, wantLen: 0},
		{name: "target groups wrong type", call: 1, output: 42, wantLen: 0},
		{
			name: "listeners",
			call: 2,
			output: []elbtypes.Listener{{
				ListenerArn:     aws.String("arn:aws:elasticloadbalancing:us-west-2:123456789012:listener/app/example-alb/50dc6c495c0c9188/f2f7dc8efc522ab2"),
				LoadBalancerArn: aws.String("arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/example-alb/50dc6c495c0c9188"),
				Protocol:        elbtypes.ProtocolEnumHttps,
				Port:            aws.Int32(443),
			}},
			wantLen:          1,
			wantResourceName: "arn:aws:elasticloadbalancing:us-west-2:123456789012:listener/app/example-alb/50dc6c495c0c9188/f2f7dc8efc522ab2",
		},
		{name: "listeners error", call: 2, err: fmt.Errorf("AccessDenied: denied"), wantLen: 1, wantError: true},
		{name: "listeners empty", call: 2, output: []elbtypes.Listener{}, wantLen: 0},
		{name: "listeners wrong type", call: 2, output: nil, wantLen: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := ELBv2Calls[tt.call]
			results := svc.Process(tt.output, tt.err, false)
			if len(results) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(results), tt.wantLen)
			}
			for _, r := range results {
				if r.ServiceName != "ELBv2" || r.MethodName != svc.Name || r.Timestamp.IsZero() {
					t.Errorf("bad result %+v", r)
				}
				if tt.wantError && r.Error == nil {
					t.Error("want Error set")
				}
				if !tt.wantError && r.Error != nil {
					t.Errorf("unexpected error %v", r.Error)
				}
				if tt.wantResourceName != "" && r.ResourceName != tt.wantResourceName {
					t.Errorf("ResourceName = %q, want %q", r.ResourceName, tt.wantResourceName)
				}
			}
		})
	}
}

func TestELBv2_Names(t *testing.T) {
	want := []string{"elbv2:DescribeLoadBalancers", "elbv2:DescribeTargetGroups", "elbv2:DescribeListeners"}
	if len(ELBv2Calls) != len(want) {
		t.Fatalf("got %d calls, want %d", len(ELBv2Calls), len(want))
	}
	for i, w := range want {
		if ELBv2Calls[i].Name != w {
			t.Errorf("call %d name = %q, want %q", i, ELBv2Calls[i].Name, w)
		}
		if ELBv2Calls[i].CallV2 == nil {
			t.Errorf("call %d: CallV2 not set", i)
		}
	}
}
