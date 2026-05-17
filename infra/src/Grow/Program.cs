using Amazon.CDK;
using Grow;

App app = new App();

Tags.Of(app).Add("Project", "grow");

new PipelineStack(app, "GrowPipelineStack", new StackProps
{
    Env = new Amazon.CDK.Environment
    {
        Account = "853479287330",
        Region  = "us-east-1"
    }
});

app.Synth();
