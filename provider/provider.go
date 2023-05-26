package provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return createProvider(providerConfigure)
}

func createProvider(configureContextFunc schema.ConfigureContextFunc) *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"profile": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"assume_role": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"role_arn": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"lambdabased_resource": LambdaBasedResource(),
		},

		ConfigureContextFunc: configureContextFunc,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile(d.Get("profile").(string)),
		config.WithRegion(d.Get("region").(string)),
	)

	if err != nil {
		return nil, diag.FromErr(err)
	}

	if assumeRoleRaw, ok := d.GetOk("assume_role"); ok {
		assumeRole := assumeRoleRaw.([]interface{})[0]
		role := assumeRole.(map[string]interface{})["role_arn"].(string)
		stsSvc := sts.NewFromConfig(cfg)
		creds := stscreds.NewAssumeRoleProvider(stsSvc, role)
		cfg.Credentials = aws.NewCredentialsCache(creds)
	}

	return lambda.NewFromConfig(cfg), nil
}
