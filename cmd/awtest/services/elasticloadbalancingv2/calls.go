package elasticloadbalancingv2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

const serviceName = "ELBv2"

func errorResults(debug bool, method string, err error) []types.ScanResult {
	utils.HandleAWSError(debug, method, err)
	return []types.ScanResult{
		{
			ServiceName: serviceName,
			MethodName:  method,
			Error:       err,
			Timestamp:   time.Now(),
		},
	}
}

func describeLoadBalancers(ctx context.Context, svc *elasticloadbalancingv2.Client) ([]elbtypes.LoadBalancer, error) {
	var lbs []elbtypes.LoadBalancer
	p := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(svc, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		lbs = append(lbs, page.LoadBalancers...)
	}
	return lbs, nil
}

var ELBv2Calls = []types.AWSService{
	{
		Name: "elbv2:DescribeLoadBalancers",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var all []elbtypes.LoadBalancer
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, elasticloadbalancingv2.NewFromConfig)
				lbs, err := describeLoadBalancers(ctx, svc)
				if err != nil {
					return nil, err
				}
				all = append(all, lbs...)
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "elbv2:DescribeLoadBalancers"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			lbs, ok := output.([]elbtypes.LoadBalancer)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, lb := range lbs {
				name := aws.ToString(lb.LoadBalancerName)
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "load-balancer",
					ResourceName: name,
					Details: map[string]interface{}{
						"Arn":     aws.ToString(lb.LoadBalancerArn),
						"DNSName": aws.ToString(lb.DNSName),
						"Type":    string(lb.Type),
						"Scheme":  string(lb.Scheme),
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found ELBv2 Load Balancer: %s (Type: %s)", utils.ColorizeItem(name), string(lb.Type)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "elbv2:DescribeTargetGroups",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var all []elbtypes.TargetGroup
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, elasticloadbalancingv2.NewFromConfig)
				p := elasticloadbalancingv2.NewDescribeTargetGroupsPaginator(svc, &elasticloadbalancingv2.DescribeTargetGroupsInput{})
				for p.HasMorePages() {
					page, err := p.NextPage(ctx)
					if err != nil {
						return nil, err
					}
					all = append(all, page.TargetGroups...)
				}
			}
			return all, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "elbv2:DescribeTargetGroups"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			groups, ok := output.([]elbtypes.TargetGroup)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, tg := range groups {
				name := aws.ToString(tg.TargetGroupName)
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "target-group",
					ResourceName: name,
					Details: map[string]interface{}{
						"Arn":      aws.ToString(tg.TargetGroupArn),
						"Protocol": string(tg.Protocol),
						"Port":     aws.ToInt32(tg.Port),
						"VpcId":    aws.ToString(tg.VpcId),
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found ELBv2 Target Group: %s", utils.ColorizeItem(name)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "elbv2:DescribeListeners",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			listeners := []elbtypes.Listener{}
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, elasticloadbalancingv2.NewFromConfig)
				lbs, err := describeLoadBalancers(ctx, svc)
				if err != nil {
					return nil, err
				}
				for _, lb := range lbs {
					p := elasticloadbalancingv2.NewDescribeListenersPaginator(svc, &elasticloadbalancingv2.DescribeListenersInput{
						LoadBalancerArn: lb.LoadBalancerArn,
					})
					for p.HasMorePages() {
						page, err := p.NextPage(ctx)
						if err != nil {
							return nil, err
						}
						listeners = append(listeners, page.Listeners...)
					}
				}
			}
			return listeners, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			const method = "elbv2:DescribeListeners"
			var results []types.ScanResult
			if err != nil {
				return errorResults(debug, method, err)
			}
			listeners, ok := output.([]elbtypes.Listener)
			if !ok {
				utils.HandleAWSError(debug, method, fmt.Errorf("unexpected output type %T", output))
				return results
			}
			for _, l := range listeners {
				arn := aws.ToString(l.ListenerArn)
				results = append(results, types.ScanResult{
					ServiceName:  serviceName,
					MethodName:   method,
					ResourceType: "listener",
					ResourceName: arn,
					Details: map[string]interface{}{
						"LoadBalancerArn": aws.ToString(l.LoadBalancerArn),
						"Protocol":        string(l.Protocol),
						"Port":            aws.ToInt32(l.Port),
					},
					Timestamp: time.Now(),
				})
				utils.PrintResult(debug, "", method, fmt.Sprintf("Found ELBv2 Listener: %s (Port: %d)", utils.ColorizeItem(arn), aws.ToInt32(l.Port)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
