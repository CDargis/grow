using Amazon.CDK;
using Amazon.CDK.AWS.CodeBuild;
using Amazon.CDK.Pipelines;
using Constructs;

namespace Grow;

public class PipelineStack : Stack
{
    public PipelineStack(Construct scope, string id, IStackProps? props = null)
        : base(scope, id, props)
    {
        const string connectionArn = "arn:aws:codeconnections:us-east-1:853479287330:connection/ee8234da-0a0f-4728-9a3f-236e5b2b8671";
        const string repoOwner    = "CDargis";
        const string repoName     = "grow";
        const string branch       = "main";

        // ── Source ────────────────────────────────────────────────────────
        IFileSetProducer source = CodePipelineSource.Connection(
            $"{repoOwner}/{repoName}",
            branch,
            new ConnectionSourceOptions
            {
                ConnectionArn = connectionArn
            }
        );

        // ── Pipeline ──────────────────────────────────────────────────────
        CodePipeline pipeline = new CodePipeline(this, "Pipeline", new CodePipelineProps
        {
            PipelineName     = "GrowPipeline",
            SelfMutation     = true,
            CrossAccountKeys = false,

            Synth = new ShellStep("Synth", new ShellStepProps
            {
                Input  = source,
                Commands = new[]
                {
                    "npm install -g aws-cdk",
                    "dotnet restore infra/src/Grow/Grow.csproj",
                    "cd frontend && npm ci && npm run build && cd ..",
                    "cd infra && cdk synth"
                },
                PrimaryOutputDirectory = "infra/cdk.out"
            }),

            CodeBuildDefaults = new CodeBuildOptions
            {
                BuildEnvironment = new BuildEnvironment
                {
                    BuildImage  = LinuxBuildImage.AMAZON_LINUX_2023_5,
                    ComputeType = ComputeType.SMALL,
                    Privileged  = true  // Docker bundling for Go Lambda
                }
            }
        });

        // ── Deploy stage ──────────────────────────────────────────────────
        pipeline.AddStage(new GrowStage(this, "Deploy", new StageProps
        {
            Env = new Amazon.CDK.Environment
            {
                Account = "853479287330",
                Region  = "us-east-1"
            }
        }));
    }
}

public class GrowStage : Stage
{
    public GrowStage(Construct scope, string id, StageProps? props = null)
        : base(scope, id, props)
    {
        new GrowStack(this, "GrowStack");
    }
}
