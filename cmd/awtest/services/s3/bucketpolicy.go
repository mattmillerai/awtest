package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// S3 bucket listing is global: this entry does not consult the region scope
// and runs exactly once per scan. Per bucket it makes GetBucketLocation plus
// GetBucketPolicy, GetBucketPolicyStatus, GetBucketAcl and GetPublicAccessBlock.
func init() {
	S3Calls = append(S3Calls, getBucketPolicyCall)
}

var getBucketPolicyCall = types.AWSService{
	Name: "s3:GetBucketPolicy",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		svc := awsv2.NewRegionalClient(cfg, "", s3.NewFromConfig)
		output, err := svc.ListBuckets(ctx, &s3.ListBucketsInput{})
		return map[string]interface{}{
			"output": output,
			"cfg":    cfg,
			"ctx":    ctx,
		}, err
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			err = awsv2.Wrap(err)
			utils.HandleAWSError(debug, "s3:GetBucketPolicy", err)
			return []types.ScanResult{{ServiceName: "S3", MethodName: "s3:GetBucketPolicy", Error: err, Timestamp: time.Now()}}
		}
		m, ok := output.(map[string]interface{})
		if !ok {
			return nil
		}
		lb, _ := m["output"].(*s3.ListBucketsOutput)
		cfg, hasCfg := m["cfg"].(aws.Config)
		ctx, _ := m["ctx"].(context.Context)
		if ctx == nil {
			ctx = context.Background()
		}
		if lb == nil || !hasCfg {
			return nil
		}

		var results []types.ScanResult
		buckets := lb.Buckets
		if len(buckets) > maxBuckets {
			buckets = buckets[:maxBuckets]
			results = append(results, types.ScanResult{
				ServiceName:  "S3",
				MethodName:   "s3:GetBucketPolicy",
				ResourceType: "note",
				ResourceName: "buckets-truncated",
				Details:      map[string]interface{}{"truncated": true, "buckets_total": len(lb.Buckets), "buckets_examined": maxBuckets},
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "s3:GetBucketPolicy", fmt.Sprintf("S3 buckets: %d found, limited to the first %d", len(lb.Buckets), maxBuckets), nil)
		}

		defaultSvc := awsv2.NewRegionalClient(cfg, "", s3.NewFromConfig)
		for _, b := range buckets {
			if b.Name == nil {
				continue
			}
			results = append(results, scanBucket(ctx, debug, cfg, defaultSvc, *b.Name)...)
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}

// scanBucket resolves the bucket region and runs the four read-only checks.
func scanBucket(ctx context.Context, debug bool, cfg aws.Config, defaultSvc *s3.Client, bucket string) []types.ScanResult {
	region := "unknown"
	svc := defaultSvc
	if loc, err := defaultSvc.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)}); err == nil {
		constraint := string(loc.LocationConstraint)
		region = NormalizeBucketRegion(&constraint)
		svc = awsv2.NewRegionalClient(cfg, region, s3.NewFromConfig)
	}

	var results []types.ScanResult
	// record builds one result and prints one line for it.
	record := func(method, label, status string, details map[string]interface{}, err error) {
		if details == nil {
			details = map[string]interface{}{}
		}
		details["region"] = region
		details["status"] = status
		results = append(results, types.ScanResult{
			ServiceName:  "S3",
			MethodName:   method,
			ResourceType: "bucket",
			ResourceName: bucket,
			Details:      details,
			Error:        err,
			Timestamp:    time.Now(),
		})
		utils.PrintResult(debug, "", method, fmt.Sprintf("S3 bucket: %s | %s: %s", utils.ColorizeItem(bucket), label, status), nil)
	}
	// fail handles an error that is neither denied nor a "none" status.
	fail := func(method string, err error) {
		err = awsv2.Wrap(err)
		utils.HandleAWSError(debug, method, err)
		results = append(results, types.ScanResult{
			ServiceName:  "S3",
			MethodName:   method,
			ResourceType: "bucket",
			ResourceName: bucket,
			Details:      map[string]interface{}{"region": region, "status": "error"},
			Error:        err,
			Timestamp:    time.Now(),
		})
	}

	// a. policy
	if out, err := svc.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)}); err != nil {
		if st, ok := statusFromError(err, codeNoPolicy, "none"); ok {
			record("s3:GetBucketPolicy", "policy", st, nil, nil)
		} else {
			fail("s3:GetBucketPolicy", err)
		}
	} else {
		doc := aws.ToString(out.Policy)
		details := map[string]interface{}{"policy": doc}
		count, findings, perr := PolicyFacts(doc)
		details["statement_count"] = count
		details["findings"] = findings
		if perr != nil {
			details["policy_parse_error"] = perr.Error()
		}
		record("s3:GetBucketPolicy", "policy", "present", details, nil)
	}

	// b. policy status
	if out, err := svc.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{Bucket: aws.String(bucket)}); err != nil {
		if st, ok := statusFromError(err, codeNoPolicy, "no_policy"); ok {
			record("s3:GetBucketPolicyStatus", "policy status", st, nil, nil)
		} else {
			fail("s3:GetBucketPolicyStatus", err)
		}
	} else {
		st := "not_public"
		if out.PolicyStatus != nil && aws.ToBool(out.PolicyStatus.IsPublic) {
			st = "public"
		}
		record("s3:GetBucketPolicyStatus", "policy status", st, nil, nil)
	}

	// c. ACL
	if out, err := svc.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: aws.String(bucket)}); err != nil {
		if st, ok := statusFromError(err, "", ""); ok {
			record("s3:GetBucketAcl", "acl", st, nil, nil)
		} else {
			fail("s3:GetBucketAcl", err)
		}
	} else {
		grants, findings := ACLFacts(out.Grants)
		details := map[string]interface{}{"grants": grants, "findings": findings}
		if out.Owner != nil {
			details["owner_id"] = aws.ToString(out.Owner.ID)
			details["owner_display_name"] = aws.ToString(out.Owner.DisplayName)
		}
		record("s3:GetBucketAcl", "acl", "ok", details, nil)
	}

	// d. public access block
	if out, err := svc.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)}); err != nil {
		if st, ok := statusFromError(err, codeNoPublicBlck, "none"); ok {
			record("s3:GetPublicAccessBlock", "public access block", st, nil, nil)
		} else {
			fail("s3:GetPublicAccessBlock", err)
		}
	} else {
		pab := out.PublicAccessBlockConfiguration
		if pab == nil {
			pab = &s3types.PublicAccessBlockConfiguration{}
		}
		record("s3:GetPublicAccessBlock", "public access block", "configured", map[string]interface{}{
			"block_public_acls":       aws.ToBool(pab.BlockPublicAcls),
			"ignore_public_acls":      aws.ToBool(pab.IgnorePublicAcls),
			"block_public_policy":     aws.ToBool(pab.BlockPublicPolicy),
			"restrict_public_buckets": aws.ToBool(pab.RestrictPublicBuckets),
		}, nil)
	}
	return results
}
