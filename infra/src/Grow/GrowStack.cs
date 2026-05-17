using Amazon.CDK;
using Amazon.CDK.AWS.CertificateManager;
using Amazon.CDK.AWS.CloudFront;
using Amazon.CDK.AWS.CloudFront.Origins;
using Amazon.CDK.AWS.DynamoDB;
using DynamoAttribute = Amazon.CDK.AWS.DynamoDB.Attribute;
using Amazon.CDK.AWS.IAM;
using Amazon.CDK.AWS.Lambda;
using LambdaFunction      = Amazon.CDK.AWS.Lambda.Function;
using LambdaFunctionProps = Amazon.CDK.AWS.Lambda.FunctionProps;
using LambdaCode          = Amazon.CDK.AWS.Lambda.Code;
using LambdaRuntime       = Amazon.CDK.AWS.Lambda.Runtime;
using LambdaArchitecture  = Amazon.CDK.AWS.Lambda.Architecture;
using Amazon.CDK.AWS.Apigatewayv2.Alpha;
using Amazon.CDK.AWS.Apigatewayv2.Integrations.Alpha;
using Amazon.CDK.AWS.Route53;
using Amazon.CDK.AWS.Route53.Targets;
using Amazon.CDK.AWS.S3;
using Amazon.CDK.AWS.S3.Deployment;
using Constructs;

namespace Grow;

public class GrowStack : Stack
{
    public GrowStack(Construct scope, string id, IStackProps? props = null)
        : base(scope, id, props)
    {
        const string domainName = "grow.chrisdargis.com";

        // ── DynamoDB ──────────────────────────────────────────────────────

        Table plantsTable = new Table(this, "PlantsTable", new TableProps
        {
            TableName     = "grow-plants",
            PartitionKey  = new DynamoAttribute { Name = "plantId", Type = AttributeType.STRING },
            BillingMode   = BillingMode.PAY_PER_REQUEST,
            RemovalPolicy = RemovalPolicy.RETAIN
        });
        plantsTable.AddGlobalSecondaryIndex(new GlobalSecondaryIndexProps
        {
            IndexName      = "user-index",
            PartitionKey   = new DynamoAttribute { Name = "userId",  Type = AttributeType.STRING },
            SortKey        = new DynamoAttribute { Name = "plantId", Type = AttributeType.STRING },
            ProjectionType = ProjectionType.ALL
        });

        Table environmentsTable = new Table(this, "EnvironmentsTable", new TableProps
        {
            TableName     = "grow-environments",
            PartitionKey  = new DynamoAttribute { Name = "environmentId", Type = AttributeType.STRING },
            BillingMode   = BillingMode.PAY_PER_REQUEST,
            RemovalPolicy = RemovalPolicy.RETAIN
        });
        environmentsTable.AddGlobalSecondaryIndex(new GlobalSecondaryIndexProps
        {
            IndexName      = "user-index",
            PartitionKey   = new DynamoAttribute { Name = "userId",        Type = AttributeType.STRING },
            SortKey        = new DynamoAttribute { Name = "environmentId", Type = AttributeType.STRING },
            ProjectionType = ProjectionType.ALL
        });

        Table logsTable = new Table(this, "LogsTable", new TableProps
        {
            TableName     = "grow-logs",
            PartitionKey  = new DynamoAttribute { Name = "plantId", Type = AttributeType.STRING },
            SortKey       = new DynamoAttribute { Name = "logId",   Type = AttributeType.STRING },
            BillingMode   = BillingMode.PAY_PER_REQUEST,
            RemovalPolicy = RemovalPolicy.RETAIN
        });
        logsTable.AddGlobalSecondaryIndex(new GlobalSecondaryIndexProps
        {
            IndexName      = "user-date-index",
            PartitionKey   = new DynamoAttribute { Name = "userId", Type = AttributeType.STRING },
            SortKey        = new DynamoAttribute { Name = "date",   Type = AttributeType.STRING },
            ProjectionType = ProjectionType.ALL
        });

        // ── S3 Buckets ────────────────────────────────────────────────────

        Bucket mediaBucket = new Bucket(this, "MediaBucket", new BucketProps
        {
            BucketName        = "grow-media",
            BlockPublicAccess = BlockPublicAccess.BLOCK_ALL,
            RemovalPolicy     = RemovalPolicy.RETAIN,
            Cors = new[]
            {
                new CorsRule
                {
                    AllowedMethods = new[] { HttpMethods.PUT, HttpMethods.GET },
                    AllowedOrigins = new[] { $"https://{domainName}" },
                    AllowedHeaders = new[] { "*" },
                    MaxAge         = 3000
                }
            }
        });

        Bucket siteBucket = new Bucket(this, "SiteBucket", new BucketProps
        {
            BucketName        = domainName,
            BlockPublicAccess = BlockPublicAccess.BLOCK_ALL,
            RemovalPolicy     = RemovalPolicy.RETAIN
        });

        // ── Lambda ────────────────────────────────────────────────────────

        LambdaFunction apiFunction = new LambdaFunction(this, "ApiFunction", new LambdaFunctionProps
        {
            FunctionName  = "grow-api",
            Runtime       = LambdaRuntime.PROVIDED_AL2023,
            Architecture  = LambdaArchitecture.ARM_64,
            Handler       = "bootstrap",
            Timeout       = Duration.Seconds(30),
            MemorySize    = 256,
            Code = LambdaCode.FromAsset("../backend", new Amazon.CDK.AWS.S3.Assets.AssetOptions
            {
                Bundling = new BundlingOptions
                {
                    Image   = DockerImage.FromRegistry("golang:1.22-alpine"),
                    Command = new[]
                    {
                        "sh", "-c",
                        "CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o /asset-output/bootstrap ./cmd/api"
                    }
                }
            }),
            Environment = new Dictionary<string, string>
            {
                ["PLANTS_TABLE"]       = plantsTable.TableName,
                ["ENVIRONMENTS_TABLE"] = environmentsTable.TableName,
                ["LOGS_TABLE"]         = logsTable.TableName,
                ["LOGS_DATE_GSI"]      = "user-date-index",
                ["MEDIA_BUCKET"]       = mediaBucket.BucketName,
                ["USER_ID"]            = "default"
            }
        });

        plantsTable.GrantReadWriteData(apiFunction);
        environmentsTable.GrantReadWriteData(apiFunction);
        logsTable.GrantReadWriteData(apiFunction);
        mediaBucket.GrantReadWrite(apiFunction);

        // ── API Gateway ───────────────────────────────────────────────────

        HttpApi httpApi = new HttpApi(this, "HttpApi", new HttpApiProps
        {
            ApiName    = "grow-api",
            CorsPreflight = new CorsPreflightOptions
            {
                AllowOrigins = new[] { $"https://{domainName}" },
                AllowMethods = new[] { CorsHttpMethod.ANY },
                AllowHeaders = new[] { "Content-Type", "Authorization" }
            }
        });

        httpApi.AddRoutes(new AddRoutesOptions
        {
            Path        = "/{proxy+}",
            Methods     = new[] { Amazon.CDK.AWS.Apigatewayv2.Alpha.HttpMethod.ANY },
            Integration = new HttpLambdaIntegration("ApiIntegration", apiFunction)
        });

        // ── ACM Certificate ───────────────────────────────────────────────

        IHostedZone hostedZone = HostedZone.FromLookup(this, "HostedZone", new HostedZoneProviderProps
        {
            DomainName = "chrisdargis.com"
        });

        Certificate certificate = new Certificate(this, "Certificate", new CertificateProps
        {
            DomainName = domainName,
            Validation = CertificateValidation.FromDns(hostedZone)
        });

        // ── CloudFront OAC ────────────────────────────────────────────────

        CfnOriginAccessControl oac = new CfnOriginAccessControl(this, "OAC", new CfnOriginAccessControlProps
        {
            OriginAccessControlConfig = new CfnOriginAccessControl.OriginAccessControlConfigProperty
            {
                Name                          = $"{domainName}-oac",
                OriginAccessControlOriginType = "s3",
                SigningBehavior               = "always",
                SigningProtocol               = "sigv4"
            }
        });

        // ── CloudFront Distribution ───────────────────────────────────────

        string apiOriginDomain = $"{httpApi.ApiId}.execute-api.{this.Region}.amazonaws.com";

        Distribution distribution = new Distribution(this, "Distribution", new DistributionProps
        {
            DefaultBehavior = new BehaviorOptions
            {
                Origin               = new S3Origin(siteBucket),
                ViewerProtocolPolicy = ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
                CachePolicy          = CachePolicy.CACHING_OPTIMIZED,
                AllowedMethods       = AllowedMethods.ALLOW_GET_HEAD
            },
            AdditionalBehaviors = new Dictionary<string, IBehaviorOptions>
            {
                ["/api/*"] = new BehaviorOptions
                {
                    Origin = new HttpOrigin(apiOriginDomain, new HttpOriginProps
                    {
                        OriginPath = ""
                    }),
                    ViewerProtocolPolicy  = ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
                    CachePolicy           = CachePolicy.CACHING_DISABLED,
                    AllowedMethods        = AllowedMethods.ALLOW_ALL,
                    OriginRequestPolicy   = OriginRequestPolicy.ALL_VIEWER_EXCEPT_HOST_HEADER
                }
            },
            ErrorResponses = new[]
            {
                new ErrorResponse { HttpStatus = 403, ResponsePagePath = "/index.html", ResponseHttpStatus = 200 },
                new ErrorResponse { HttpStatus = 404, ResponsePagePath = "/index.html", ResponseHttpStatus = 200 }
            },
            DefaultRootObject = "index.html",
            DomainNames       = new[] { domainName },
            Certificate       = certificate,
            PriceClass        = PriceClass.PRICE_CLASS_100
        });

        // Attach OAC to the S3 origin via L1 escape hatch
        CfnDistribution cfnDistribution = (CfnDistribution)distribution.Node.DefaultChild!;
        cfnDistribution.AddPropertyOverride("DistributionConfig.Origins.0.OriginAccessControlId", oac.AttrId);
        cfnDistribution.AddPropertyOverride("DistributionConfig.Origins.0.S3OriginConfig.OriginAccessIdentity", "");

        siteBucket.AddToResourcePolicy(new PolicyStatement(new PolicyStatementProps
        {
            Actions    = new[] { "s3:GetObject" },
            Resources  = new[] { siteBucket.ArnForObjects("*") },
            Principals = new IPrincipal[] { new ServicePrincipal("cloudfront.amazonaws.com") },
            Conditions = new Dictionary<string, object>
            {
                ["StringEquals"] = new Dictionary<string, string>
                {
                    ["AWS:SourceArn"] = $"arn:aws:cloudfront::{this.Account}:distribution/{distribution.DistributionId}"
                }
            }
        }));

        // ── Route 53 ──────────────────────────────────────────────────────

        new ARecord(this, "ARecord", new ARecordProps
        {
            Zone       = hostedZone,
            RecordName = domainName,
            Target     = RecordTarget.FromAlias(new CloudFrontTarget(distribution))
        });

        // ── Frontend Deployment ───────────────────────────────────────────

        new BucketDeployment(this, "SiteDeployment", new BucketDeploymentProps
        {
            Sources             = new[] { Source.Asset("../frontend/dist") },
            DestinationBucket   = siteBucket,
            Distribution        = distribution,
            DistributionPaths   = new[] { "/*" }
        });

        // ── Outputs ───────────────────────────────────────────────────────

        new CfnOutput(this, "SiteUrl",                new CfnOutputProps { Value = $"https://{domainName}" });
        new CfnOutput(this, "DistributionId",         new CfnOutputProps { Value = distribution.DistributionId });
        new CfnOutput(this, "ApiEndpoint",            new CfnOutputProps { Value = httpApi.ApiEndpoint });
        new CfnOutput(this, "PlantsTableName",        new CfnOutputProps { Value = plantsTable.TableName });
        new CfnOutput(this, "EnvironmentsTableName",  new CfnOutputProps { Value = environmentsTable.TableName });
        new CfnOutput(this, "LogsTableName",          new CfnOutputProps { Value = logsTable.TableName });
        new CfnOutput(this, "MediaBucketName",        new CfnOutputProps { Value = mediaBucket.BucketName });
    }
}
