package provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	v1 "github.com/aws/aws-sdk-go-v2/service/apigateway"
	v2 "github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
)

type apiGatewayAction string

const (
	INCLUDE apiGatewayAction = "include"
	EXCLUDE apiGatewayAction = "exclude"
)

var (
	ApiGatewayActions     = []string{string(INCLUDE), string(EXCLUDE)}
	AccessLogFormatValues = []string{"$context.httpMethod", "$context.domainName", "$context.status", "$context.path"}
)

type AwsApiGatewayProvider interface {
	getAwsGetRestApisPaginator() AwsGetRestApisPaginator
	getApiGatewayClient() AwsApiGatewayClient
	getApiGatewayV2Client() AwsApiGatewayV2Client
}

type apiGatewayProvider struct {
	config             aws.Config
	apiGatewayClient   AwsApiGatewayClient
	apiGatewayV2Client AwsApiGatewayV2Client
}

type AwsApiGatewayClient interface {
	GetRestApis(ctx context.Context, params *v1.GetRestApisInput, optFns ...func(*v1.Options)) (*v1.GetRestApisOutput, error)
	GetStages(ctx context.Context, params *v1.GetStagesInput, optFns ...func(*v1.Options)) (*v1.GetStagesOutput, error)
}

type AwsApiGatewayV2Client interface {
	GetApis(ctx context.Context, params *v2.GetApisInput, optFns ...func(*v2.Options)) (*v2.GetApisOutput, error)
	GetStages(ctx context.Context, params *v2.GetStagesInput, optFns ...func(*v2.Options)) (*v2.GetStagesOutput, error)
}

type AwsGetRestApisPaginator interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*v1.Options)) (*v1.GetRestApisOutput, error)
}

var _ AwsApiGatewayProvider = (*apiGatewayProvider)(nil)

func (p *apiGatewayProvider) getAwsGetRestApisPaginator() AwsGetRestApisPaginator {
	return v1.NewGetRestApisPaginator(p.apiGatewayClient, &v1.GetRestApisInput{})
}

func (p *apiGatewayProvider) getApiGatewayClient() AwsApiGatewayClient {
	return p.apiGatewayClient
}

func (p *apiGatewayProvider) getApiGatewayV2Client() AwsApiGatewayV2Client {
	return p.apiGatewayV2Client
}

func newFromConfig(cfg aws.Config) *apiGatewayProvider {
	return &apiGatewayProvider{
		config:             cfg,
		apiGatewayClient:   v1.NewFromConfig(cfg),
		apiGatewayV2Client: v2.NewFromConfig(cfg),
	}
}
