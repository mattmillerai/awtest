module github.com/mattmillerai/awtest

go 1.26.0

toolchain go1.27.3

require (
	github.com/aws/aws-sdk-go-v2 v1.47.4
	github.com/aws/aws-sdk-go-v2/config v1.33.10
	github.com/aws/aws-sdk-go-v2/credentials v1.20.10
	github.com/aws/aws-sdk-go-v2/service/accessanalyzer v1.56.0
	github.com/aws/aws-sdk-go-v2/service/account v1.35.7
	github.com/aws/aws-sdk-go-v2/service/acm v1.46.1
	github.com/aws/aws-sdk-go-v2/service/amplify v1.44.1
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.47.0
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.42.0
	github.com/aws/aws-sdk-go-v2/service/appsync v1.61.0
	github.com/aws/aws-sdk-go-v2/service/athena v1.62.1
	github.com/aws/aws-sdk-go-v2/service/backup v1.65.0
	github.com/aws/aws-sdk-go-v2/service/batch v1.76.0
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.82.2
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.75.1
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.60.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.52.8
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.87.0
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.72.7
	github.com/aws/aws-sdk-go-v2/service/codecommit v1.36.5
	github.com/aws/aws-sdk-go-v2/service/codedeploy v1.40.1
	github.com/aws/aws-sdk-go-v2/service/codepipeline v1.49.7
	github.com/aws/aws-sdk-go-v2/service/cognitoidentity v1.38.1
	github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider v1.70.0
	github.com/aws/aws-sdk-go-v2/service/configservice v1.70.1
	github.com/aws/aws-sdk-go-v2/service/directconnect v1.51.0
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.68.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.341.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.56.0
	github.com/aws/aws-sdk-go-v2/service/ecs v1.98.0
	github.com/aws/aws-sdk-go-v2/service/efs v1.49.4
	github.com/aws/aws-sdk-go-v2/service/eks v1.99.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.64.1
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.43.5
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.63.5
	github.com/aws/aws-sdk-go-v2/service/emr v1.66.1
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.48.7
	github.com/aws/aws-sdk-go-v2/service/glacier v1.37.1
	github.com/aws/aws-sdk-go-v2/service/glue v1.160.0
	github.com/aws/aws-sdk-go-v2/service/guardduty v1.93.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.64.5
	github.com/aws/aws-sdk-go-v2/service/iot v1.80.1
	github.com/aws/aws-sdk-go-v2/service/ivs v1.55.7
	github.com/aws/aws-sdk-go-v2/service/ivschat v1.29.4
	github.com/aws/aws-sdk-go-v2/service/ivsrealtime v1.37.7
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.54.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.52.0
	github.com/aws/aws-sdk-go-v2/service/lambda v1.108.0
	github.com/aws/aws-sdk-go-v2/service/macie2 v1.59.5
	github.com/aws/aws-sdk-go-v2/service/mediaconvert v1.104.0
	github.com/aws/aws-sdk-go-v2/service/neptune v1.53.5
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.80.0
	github.com/aws/aws-sdk-go-v2/service/organizations v1.56.1
	github.com/aws/aws-sdk-go-v2/service/rds v1.131.1
	github.com/aws/aws-sdk-go-v2/service/redshift v1.71.5
	github.com/aws/aws-sdk-go-v2/service/rekognition v1.60.3
	github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi v1.35.5
	github.com/aws/aws-sdk-go-v2/service/route53 v1.70.5
	github.com/aws/aws-sdk-go-v2/service/s3 v1.114.3
	github.com/aws/aws-sdk-go-v2/service/sagemaker v1.270.1
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.49.0
	github.com/aws/aws-sdk-go-v2/service/securityhub v1.81.0
	github.com/aws/aws-sdk-go-v2/service/ses v1.42.5
	github.com/aws/aws-sdk-go-v2/service/sfn v1.50.0
	github.com/aws/aws-sdk-go-v2/service/sns v1.47.6
	github.com/aws/aws-sdk-go-v2/service/sqs v1.48.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.79.3
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.5
	github.com/aws/aws-sdk-go-v2/service/transcribe v1.60.1
	github.com/aws/aws-sdk-go-v2/service/waf v1.38.4
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.77.7
	github.com/aws/smithy-go v1.28.6
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/olekukonko/tablewriter v0.0.5
	golang.org/x/term v0.47.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.22 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.5 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	golang.org/x/sys v0.49.0 // indirect
)
