package provider

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type LambdaClient interface {
	Invoke(ctx context.Context, params *lambda.InvokeInput, optFns ...func(*lambda.Options)) (*lambda.InvokeOutput, error)
}

func LambdaBasedResource() *schema.Resource {
	return &schema.Resource{
		Create: resourceCreateUpdate,
		Read:   resourceRead,
		Update: resourceCreateUpdate,
		Delete: resourceDelete,

		Schema: map[string]*schema.Schema{
			"function_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"qualifier": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "$LATEST",
			},
			"triggers": {
				Type:     schema.TypeMap,
				Optional: true,
			},
			"input": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateFunc:     validation.StringIsJSON,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool { return d.Get("conceal_input").(bool) },
			},
			"conceal_input": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"conceal_result": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"finalizer": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"function_name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"qualifier": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "$LATEST",
						},
						"input": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsJSON,
						},
					},
				},
			},
			"result": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	d.Partial(true)
	concealInput := d.Get("conceal_input").(bool)
	concealResult := d.Get("conceal_result").(bool)

	res, err := callLambda(d.Id(), extractLambdaInformation(d), meta)
	if err != nil {
		return err
	}

	if d.Id() == "" {
		d.SetId(uuid.New().String())
	}
	if concealInput {
		d.Set("input", "")
	}

	if concealResult {
		d.Set("result", "")
	} else {
		d.Set("result", string(res))
	}

	d.Partial(false)
	return nil
}

func resourceRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceDelete(d *schema.ResourceData, meta interface{}) error {
	concealResult := d.Get("conceal_result").(bool)
	if destroyRaw, ok := d.GetOk("finalizer"); ok {
		destroy := destroyRaw.([]interface{})
		if len(destroy) > 0 {
			res, err := callLambda(d.Id(), destroy[0].(map[string]interface{}), meta)
			if err != nil {
				return err
			}
			if !concealResult {
				log.Printf("%s received destroy response: %s\n", d.Id(), string(res))
			}
		}
	}
	d.SetId("")
	return nil
}

func extractLambdaInformation(d *schema.ResourceData) map[string]interface{} {
	ret := map[string]interface{}{}
	for _, param := range []string{"function_name", "qualifier", "input"} {
		attr := d.GetRawConfig().GetAttr(param)
		if !attr.IsNull() {
			// Using raw config because input is wiped from regular config if concealed (see DiffSuppressFunc of input field)
			ret[param] = d.GetRawConfig().GetAttr(param).AsString()
		} else {
			ret[param] = d.Get(param)
		}
	}
	return ret
}

func callLambda(id string, data map[string]interface{}, meta interface{}) ([]byte, error) {
	conn := meta.(LambdaClient)

	functionName := data["function_name"].(string)
	qualifier := data["qualifier"].(string)
	input := []byte(data["input"].(string))

	res, err := conn.Invoke(context.TODO(), &lambda.InvokeInput{
		FunctionName:   aws.String(functionName),
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		Payload:        input,
		Qualifier:      aws.String(qualifier),
	})

	if err != nil {
		return nil, fmt.Errorf("Lambda Invocation (%s) failed: %w", id, err)
	}

	if res.FunctionError != nil {
		return nil, fmt.Errorf("Lambda function (%s) returned error: (%s)", functionName, string(res.Payload))
	}

	return res.Payload, nil
}
