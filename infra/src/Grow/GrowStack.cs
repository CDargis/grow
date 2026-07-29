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
using Amazon.CDK.AWS.Apigatewayv2.Authorizers.Alpha;
using Amazon.CDK.AWS.Apigatewayv2.Integrations.Alpha;
using Amazon.CDK.AWS.Cognito;
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
        logsTable.AddGlobalSecondaryIndex(new GlobalSecondaryIndexProps
        {
            IndexName      = "user-logtype-date-index",
            PartitionKey   = new DynamoAttribute { Name = "userId",      Type = AttributeType.STRING },
            SortKey        = new DynamoAttribute { Name = "logTypeDate", Type = AttributeType.STRING },
            ProjectionType = ProjectionType.ALL
        });

        Table settingsTable = new Table(this, "SettingsTable", new TableProps
        {
            TableName     = "grow-settings",
            PartitionKey  = new DynamoAttribute { Name = "userId", Type = AttributeType.STRING },
            BillingMode   = BillingMode.PAY_PER_REQUEST,
            RemovalPolicy = RemovalPolicy.RETAIN
        });

        Table productsTable = new Table(this, "ProductsTable", new TableProps
        {
            TableName     = "grow-products",
            PartitionKey  = new DynamoAttribute { Name = "productId", Type = AttributeType.STRING },
            BillingMode   = BillingMode.PAY_PER_REQUEST,
            RemovalPolicy = RemovalPolicy.RETAIN
        });
        productsTable.AddGlobalSecondaryIndex(new GlobalSecondaryIndexProps
        {
            IndexName      = "user-index",
            PartitionKey   = new DynamoAttribute { Name = "userId",    Type = AttributeType.STRING },
            SortKey        = new DynamoAttribute { Name = "productId", Type = AttributeType.STRING },
            ProjectionType = ProjectionType.ALL
        });

        // ── Cognito ───────────────────────────────────────────────────────

        UserPool userPool = new UserPool(this, "UserPool", new UserPoolProps
        {
            UserPoolName      = "grow-users",
            SelfSignUpEnabled = false,
            SignInAliases     = new SignInAliases { Email = true },
            StandardAttributes = new StandardAttributes
            {
                Email = new StandardAttribute { Required = true, Mutable = true }
            },
            PasswordPolicy = new PasswordPolicy
            {
                MinLength        = 12,
                RequireLowercase = true,
                RequireUppercase = true,
                RequireDigits    = true,
                RequireSymbols   = false
            },
            AccountRecovery = AccountRecovery.EMAIL_ONLY,
            RemovalPolicy   = RemovalPolicy.RETAIN
        });

        UserPoolClient userPoolClient = userPool.AddClient("WebClient", new UserPoolClientOptions
        {
            UserPoolClientName   = "grow-web",
            GenerateSecret       = false, // public SPA client -- PKCE, no client secret
            RefreshTokenValidity = Duration.Days(365), // personal app -- avoid frequent re-logins
            OAuth = new OAuthSettings
            {
                Flows        = new OAuthFlows { AuthorizationCodeGrant = true },
                Scopes       = new[] { OAuthScope.OPENID, OAuthScope.EMAIL, OAuthScope.PROFILE },
                CallbackUrls = new[] { $"https://{domainName}/callback" },
                LogoutUrls   = new[] { $"https://{domainName}/" }
            },
            SupportedIdentityProviders = new[] { UserPoolClientIdentityProvider.COGNITO }
        });

        // Separate client for Claude's MCP connector, not the browser frontend's WebClient.
        // Cognito's OIDC discovery document only advertises client_secret_basic/post for
        // token_endpoint_auth_methods_supported -- never "none", even for genuinely public
        // clients -- so a standards-compliant OAuth client (Claude) needs an actual secret
        // to authenticate the way the metadata says it should. The browser frontend can't
        // hold a secret safely, so it stays on its own public/PKCE-only client above.
        UserPoolClient mcpClient = userPool.AddClient("McpClient", new UserPoolClientOptions
        {
            UserPoolClientName   = "grow-mcp",
            GenerateSecret       = true,
            RefreshTokenValidity = Duration.Days(365),
            OAuth = new OAuthSettings
            {
                Flows        = new OAuthFlows { AuthorizationCodeGrant = true },
                Scopes       = new[] { OAuthScope.OPENID, OAuthScope.EMAIL, OAuthScope.PROFILE },
                CallbackUrls = new[] { "https://claude.ai/api/mcp/auth_callback" }
            },
            SupportedIdentityProviders = new[] { UserPoolClientIdentityProvider.COGNITO }
        });

        userPool.AddDomain("Domain", new UserPoolDomainOptions
        {
            CognitoDomain = new CognitoDomainOptions { DomainPrefix = "grow-chrisdargis" }
        });

        HttpJwtAuthorizer jwtAuthorizer = new HttpJwtAuthorizer(
            "JwtAuthorizer",
            $"https://cognito-idp.{this.Region}.amazonaws.com/{userPool.UserPoolId}",
            new HttpJwtAuthorizerProps
            {
                JwtAudience = new[] { userPoolClient.UserPoolClientId }
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
                    Image   = DockerImage.FromRegistry("public.ecr.aws/docker/library/golang:1.25-alpine"),
                    Command = new[]
                    {
                        "sh", "-c",
                        "CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOCACHE=/tmp/go-build go build -tags lambda.norpc -o /asset-output/bootstrap ./cmd/api"
                    }
                }
            }),
            Environment = new Dictionary<string, string>
            {
                ["PLANTS_TABLE"]       = plantsTable.TableName,
                ["ENVIRONMENTS_TABLE"] = environmentsTable.TableName,
                ["LOGS_TABLE"]         = logsTable.TableName,
                ["LOGS_DATE_GSI"]           = "user-date-index",
                ["LOGS_LOGTYPE_DATE_GSI"]   = "user-logtype-date-index",
                ["SETTINGS_TABLE"]     = settingsTable.TableName,
                ["PRODUCTS_TABLE"]     = productsTable.TableName,
                ["MEDIA_BUCKET"]       = mediaBucket.BucketName,
                ["USER_ID"]                 = "default",
                ["USER_POOL_ID"]            = userPool.UserPoolId,
                ["USER_POOL_CLIENT_ID"]     = userPoolClient.UserPoolClientId,
                ["MCP_USER_POOL_CLIENT_ID"] = mcpClient.UserPoolClientId,
                ["PUBLIC_BASE_URL"]         = $"https://{domainName}",
                ["COGNITO_HOSTED_UI_BASE"]  = $"https://grow-chrisdargis.auth.{this.Region}.amazoncognito.com"
            }
        });

        plantsTable.GrantReadWriteData(apiFunction);
        environmentsTable.GrantReadWriteData(apiFunction);
        logsTable.GrantReadWriteData(apiFunction);
        settingsTable.GrantReadWriteData(apiFunction);
        productsTable.GrantReadWriteData(apiFunction);
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

        HttpLambdaIntegration apiIntegration = new HttpLambdaIntegration("ApiIntegration", apiFunction);

        // Unauthenticated on purpose: the frontend needs these (non-secret)
        // values to start the Cognito login flow before it has a token.
        // Registered as a specific path so it takes precedence over the
        // {proxy+} catch-all below, which requires the JWT authorizer.
        httpApi.AddRoutes(new AddRoutesOptions
        {
            Path        = "/api/auth-config",
            Methods     = new[] { Amazon.CDK.AWS.Apigatewayv2.Alpha.HttpMethod.GET },
            Integration = apiIntegration
        });

        // MCP: also unauthenticated at the API Gateway level, on purpose --
        // it validates its own bearer token in Go so it can return the
        // WWW-Authenticate response shape MCP clients expect, which API
        // Gateway's built-in JWT authorizer can't be customized to produce.
        // See internal/mcpserver's package comment.
        httpApi.AddRoutes(new AddRoutesOptions
        {
            Path        = "/api/mcp",
            Methods     = new[] { Amazon.CDK.AWS.Apigatewayv2.Alpha.HttpMethod.ANY },
            Integration = apiIntegration
        });

        // Unauthenticated OAuth resource metadata for MCP clients -- must be
        // reachable at the domain root, not just under /api/*, so it also
        // gets its own CloudFront behavior below.
        httpApi.AddRoutes(new AddRoutesOptions
        {
            Path        = "/.well-known/oauth-protected-resource",
            Methods     = new[] { Amazon.CDK.AWS.Apigatewayv2.Alpha.HttpMethod.GET },
            Integration = apiIntegration
        });

        // RFC 8414 authorization-server metadata, hosted by us at the domain
        // root and pointing at Cognito's real endpoints -- Cognito's own
        // path-bearing issuer URL breaks MCP clients' metadata discovery.
        // See internal/mcpserver.AuthorizationServerMetadata.
        httpApi.AddRoutes(new AddRoutesOptions
        {
            Path        = "/.well-known/oauth-authorization-server",
            Methods     = new[] { Amazon.CDK.AWS.Apigatewayv2.Alpha.HttpMethod.GET },
            Integration = apiIntegration
        });

        httpApi.AddRoutes(new AddRoutesOptions
        {
            Path        = "/{proxy+}",
            Methods     = new[] { Amazon.CDK.AWS.Apigatewayv2.Alpha.HttpMethod.ANY },
            Integration = apiIntegration,
            Authorizer  = jwtAuthorizer
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
        HttpOrigin apiOrigin = new HttpOrigin(apiOriginDomain, new HttpOriginProps { OriginPath = "" });

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
                    Origin                = apiOrigin,
                    ViewerProtocolPolicy  = ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
                    CachePolicy           = CachePolicy.CACHING_DISABLED,
                    AllowedMethods        = AllowedMethods.ALLOW_ALL,
                    OriginRequestPolicy   = OriginRequestPolicy.ALL_VIEWER_EXCEPT_HOST_HEADER
                },
                // MCP clients look for OAuth resource metadata at the domain
                // root, not just under /api/* -- see internal/mcpserver.
                ["/.well-known/*"] = new BehaviorOptions
                {
                    Origin                = apiOrigin,
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
        new CfnOutput(this, "SettingsTableName",      new CfnOutputProps { Value = settingsTable.TableName });
        new CfnOutput(this, "ProductsTableName",      new CfnOutputProps { Value = productsTable.TableName });
        new CfnOutput(this, "MediaBucketName",        new CfnOutputProps { Value = mediaBucket.BucketName });
        new CfnOutput(this, "UserPoolId",             new CfnOutputProps { Value = userPool.UserPoolId });
        new CfnOutput(this, "UserPoolClientId",        new CfnOutputProps { Value = userPoolClient.UserPoolClientId });
        new CfnOutput(this, "UserPoolDomain",         new CfnOutputProps { Value = $"https://grow-chrisdargis.auth.{this.Region}.amazoncognito.com" });
        new CfnOutput(this, "McpEndpoint",             new CfnOutputProps { Value = $"https://{domainName}/api/mcp" });
        new CfnOutput(this, "McpClientId",             new CfnOutputProps { Value = mcpClient.UserPoolClientId });
        new CfnOutput(this, "McpClientSecret",         new CfnOutputProps { Value = mcpClient.UserPoolClientSecret.UnsafeUnwrap() });
    }
}
