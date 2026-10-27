package services

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/accessanalyzer"
	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/amplify"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/appsync"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/aws/aws-sdk-go-v2/service/directconnect"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/emr"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/glacier"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/guardduty"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iot"
	"github.com/aws/aws-sdk-go-v2/service/ivs"
	"github.com/aws/aws-sdk-go-v2/service/ivschat"
	"github.com/aws/aws-sdk-go-v2/service/ivsrealtime"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/macie2"
	"github.com/aws/aws-sdk-go-v2/service/mediaconvert"
	"github.com/aws/aws-sdk-go-v2/service/neptune"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/securityhub"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/transcribe"
	"github.com/aws/aws-sdk-go-v2/service/waf"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
)

// How to add a migrated (CallV2) service to the endpoint guards:
//
//  1. Look up `const ServiceID = "..."` in
//     vendor/github.com/aws/aws-sdk-go-v2/service/<pkg>/api_client.go (it is
//     also exported as <pkg>.ServiceID).
//  2. Add the prefix (the part of AWSService.Name before ':') to v2ServiceIDs
//     with that package's ServiceID constant, one per client the service's
//     CallV2/Process builds (e.g. sts also makes IAM follow-up calls).
//  3. Add the prefix to v2EnvSuffix with the matching suffixes of
//     AWS_ENDPOINT_URL_<SUFFIX>, in the same order. The SDK derives the suffix
//     from the ServiceID with strings.ToUpper and spaces -> '_'
//     (e.g. "ACM" -> ACM, "Cognito Identity" -> COGNITO_IDENTITY,
//     "SFN" -> SFN). Never guess it from the service name: the name
//     "certificatemanager" would give CERTIFICATEMANAGER instead of ACM, and the
//     ambient-endpoint guard would then pass vacuously.
//
// guardTargets fails the test when a service has no v2EnvSuffix entry;
// TestV2EnvSuffixMatchesServiceID proves the two tables agree.

// v2EnvSuffix maps a service-name prefix to the <SVC> parts of
// AWS_ENDPOINT_URL_<SVC> that the service's clients resolve. Every service in
// AllServices needs an entry. The "sts" prefix also covers the test-only synthetic v2 service.
var v2EnvSuffix = map[string][]string{
	"sts": {"STS", "IAM"}, // Process makes IAM follow-up calls with the same config
	"kms": {"KMS"},
	"ecr": {"ECR"},
	"s3":  {"S3"},

	"batch":          {"BATCH"},
	"cloudformation": {"CLOUDFORMATION"},
	"codepipeline":   {"CODEPIPELINE"},
	"glacier":        {"GLACIER"},
	"eventbridge":    {"EVENTBRIDGE"},

	"appsync":         {"APPSYNC"},
	"secretsmanager":  {"SECRETS_MANAGER"},
	"ses":             {"SES"},
	"rds":             {"RDS"},
	"cognitoidentity": {"COGNITO_IDENTITY"},

	"amplify":     {"AMPLIFY"},
	"acm":         {"ACM"},
	"states":      {"SFN"},
	"ssm":         {"SSM"},
	"cognito-idp": {"COGNITO_IDENTITY_PROVIDER"},

	// ecs also covers the fargate service (ecs:ListFargateTasks).
	"ecs":         {"ECS"},
	"efs":         {"EFS"},
	"eks":         {"EKS"},
	"elasticache": {"ELASTICACHE"},

	"redshift": {"REDSHIFT"},
	"config":   {"CONFIG_SERVICE"},
	"lambda":   {"LAMBDA"},
	// cloudtrail covers both cloudtrail:DescribeTrails and cloudtrail:ListTrails;
	// elasticbeanstalk covers DescribeApplications and DescribeEvents.
	"cloudtrail":       {"CLOUDTRAIL"},
	"elasticbeanstalk": {"ELASTIC_BEANSTALK"},

	"glue":        {"GLUE"},
	"dynamodb":    {"DYNAMODB"},
	"iot":         {"IOT"},
	"rekognition": {"REKOGNITION"},
	"transcribe":  {"TRANSCRIBE"},

	"athena":     {"ATHENA"},
	"backup":     {"BACKUP"},
	"codebuild":  {"CODEBUILD"},
	"codecommit": {"CODECOMMIT"},
	"codedeploy": {"CODEDEPLOY"},

	"directconnect": {"DIRECT_CONNECT"},
	"emr":           {"EMR"},
	"kinesis":       {"KINESIS"},
	"guardduty":     {"GUARDDUTY"},
	"macie2":        {"MACIE2"},

	"mediaconvert":  {"MEDIACONVERT"},
	"neptune":       {"NEPTUNE"},
	"opensearch":    {"OPENSEARCH"},
	"organizations": {"ORGANIZATIONS"},
	"sagemaker":     {"SAGEMAKER"},

	"securityhub": {"SECURITYHUB"},
	"apigateway":  {"API_GATEWAY"},
	"sns":         {"SNS"},
	"iam":         {"IAM"},

	// The cloudwatchlogs prefix covers cloudwatchlogs:DescribeLogGroupsAndStreams
	// (CloudWatch Logs client) and cloudwatchlogs:ListMetrics (CloudWatch client).
	"route53":        {"ROUTE_53"},
	"cloudfront":     {"CLOUDFRONT"},
	"cloudwatch":     {"CLOUDWATCH"},
	"cloudwatchlogs": {"CLOUDWATCH_LOGS", "CLOUDWATCH"},

	// ec2 covers both ec2:DescribeInstances (ec2 service) and ec2:DescribeVpcs
	// (vpc service), which share the ec2 module.
	"ec2": {"EC2"},

	// sqs:ListQueues also builds an EventBridge client (DescribeRule follow-up).
	"sqs":         {"SQS", "EVENTBRIDGE"},
	"waf":         {"WAF"},
	"ivs":         {"IVS"},
	"ivsChat":     {"IVSCHAT"},
	"ivsRealtime": {"IVS_REALTIME"},

	// "tag" is the Resource Groups Tagging API call prefix (tag:GetResources).
	"elbv2":          {"ELASTIC_LOAD_BALANCING_V2"},
	"wafv2":          {"WAFV2"},
	"tag":            {"RESOURCE_GROUPS_TAGGING_API"},
	"accessanalyzer": {"ACCESSANALYZER"},
	"account":        {"ACCOUNT"},
	"apigatewayv2":   {"APIGATEWAYV2"},
}

// v2ServiceIDs maps the same prefixes to the ServiceID constants of the v2
// service packages, in the same order as v2EnvSuffix.
var v2ServiceIDs = map[string][]string{
	"sts": {sts.ServiceID, iam.ServiceID},
	"kms": {kms.ServiceID},
	"ecr": {ecr.ServiceID},
	"s3":  {s3.ServiceID},

	"batch":          {batch.ServiceID},
	"cloudformation": {cloudformation.ServiceID},
	"codepipeline":   {codepipeline.ServiceID},
	"glacier":        {glacier.ServiceID},
	"eventbridge":    {eventbridge.ServiceID},

	"appsync":         {appsync.ServiceID},
	"secretsmanager":  {secretsmanager.ServiceID},
	"ses":             {ses.ServiceID},
	"rds":             {rds.ServiceID},
	"cognitoidentity": {cognitoidentity.ServiceID},

	"amplify":     {amplify.ServiceID},
	"acm":         {acm.ServiceID},
	"states":      {sfn.ServiceID},
	"ssm":         {ssm.ServiceID},
	"cognito-idp": {cognitoidentityprovider.ServiceID},

	"ecs":         {ecs.ServiceID},
	"efs":         {efs.ServiceID},
	"eks":         {eks.ServiceID},
	"elasticache": {elasticache.ServiceID},

	"redshift":         {redshift.ServiceID},
	"config":           {configservice.ServiceID},
	"lambda":           {lambda.ServiceID},
	"cloudtrail":       {cloudtrail.ServiceID},
	"elasticbeanstalk": {elasticbeanstalk.ServiceID},

	"glue":        {glue.ServiceID},
	"dynamodb":    {dynamodb.ServiceID},
	"iot":         {iot.ServiceID},
	"rekognition": {rekognition.ServiceID},
	"transcribe":  {transcribe.ServiceID},

	"athena":     {athena.ServiceID},
	"backup":     {backup.ServiceID},
	"codebuild":  {codebuild.ServiceID},
	"codecommit": {codecommit.ServiceID},
	"codedeploy": {codedeploy.ServiceID},

	"directconnect": {directconnect.ServiceID},
	"emr":           {emr.ServiceID},
	"kinesis":       {kinesis.ServiceID},
	"guardduty":     {guardduty.ServiceID},
	"macie2":        {macie2.ServiceID},

	"mediaconvert":  {mediaconvert.ServiceID},
	"neptune":       {neptune.ServiceID},
	"opensearch":    {opensearch.ServiceID},
	"organizations": {organizations.ServiceID},
	"sagemaker":     {sagemaker.ServiceID},

	"securityhub": {securityhub.ServiceID},
	"apigateway":  {apigateway.ServiceID},
	"sns":         {sns.ServiceID},
	"iam":         {iam.ServiceID},

	"route53":        {route53.ServiceID},
	"cloudfront":     {cloudfront.ServiceID},
	"cloudwatch":     {cloudwatch.ServiceID},
	"cloudwatchlogs": {cloudwatchlogs.ServiceID, cloudwatch.ServiceID},

	"ec2": {ec2.ServiceID},

	"sqs":         {sqs.ServiceID, eventbridge.ServiceID},
	"waf":         {waf.ServiceID},
	"ivs":         {ivs.ServiceID},
	"ivsChat":     {ivschat.ServiceID},
	"ivsRealtime": {ivsrealtime.ServiceID},

	"elbv2":          {elasticloadbalancingv2.ServiceID},
	"wafv2":          {wafv2.ServiceID},
	"tag":            {resourcegroupstaggingapi.ServiceID},
	"accessanalyzer": {accessanalyzer.ServiceID},
	"account":        {account.ServiceID},
	"apigatewayv2":   {apigatewayv2.ServiceID},
}

// sdkEnvSuffix is how the SDK turns a ServiceID into the AWS_ENDPOINT_URL_<SVC>
// suffix.
func sdkEnvSuffix(serviceID string) string {
	return strings.ToUpper(strings.ReplaceAll(serviceID, " ", "_"))
}

func TestV2EnvSuffixMatchesServiceID(t *testing.T) {
	for prefix, ids := range v2ServiceIDs {
		got, ok := v2EnvSuffix[prefix]
		if !ok {
			t.Errorf("%s: in v2ServiceIDs but missing from v2EnvSuffix", prefix)
			continue
		}
		if len(got) != len(ids) {
			t.Errorf("%s: v2EnvSuffix has %d entries, v2ServiceIDs has %d", prefix, len(got), len(ids))
			continue
		}
		for i, id := range ids {
			if want := sdkEnvSuffix(id); got[i] != want {
				t.Errorf("%s: v2EnvSuffix[%d] = %q, but ServiceID %q gives %q", prefix, i, got[i], id, want)
			}
		}
	}
	for prefix := range v2EnvSuffix {
		if _, ok := v2ServiceIDs[prefix]; !ok {
			t.Errorf("%s: in v2EnvSuffix but missing from v2ServiceIDs", prefix)
		}
	}
}

// TestGuardTargetsUseExplicitSuffixes checks that every guard target got its
// suffixes from the table.
func TestGuardTargetsUseExplicitSuffixes(t *testing.T) {
	for _, tg := range guardTargets(t) {
		want := v2EnvSuffix[servicePrefix(tg.svc.Name)]
		if strings.Join(tg.envSuffixes, ",") != strings.Join(want, ",") {
			t.Errorf("%s: suffixes %v, want %v", tg.svc.Name, tg.envSuffixes, want)
		}
	}
}
