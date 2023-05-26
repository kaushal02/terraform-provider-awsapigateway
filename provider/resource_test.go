package provider

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/kaushal02/terraform-provider-awsapigateway/provider/mocks"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestLambdaBasedResource_basicLifecycle(t *testing.T) {
	m, c := createMockLambdaClient(t)
	defer c.Finish()
	var steps []resource.TestStep

	configParam := newConfigParameters()

	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			rs := getTestResourceState(s)
			assert.Equal(t, "func-createupdate-name-1", rs.Attributes["function_name"])
			assert.Equal(t, "$LATEST", rs.Attributes["qualifier"])
			assert.Equal(t, getInputJson("createupdate-input-param-val"), rs.Attributes["input"])
			return nil
		},
	})

	// No lambda invocation when no change
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			rs := getTestResourceState(s)
			assert.Equal(t, "func-createupdate-name-1", rs.Attributes["function_name"])
			assert.Equal(t, "$LATEST", rs.Attributes["qualifier"])
			assert.Equal(t, getInputJson("createupdate-input-param-val"), rs.Attributes["input"])
			return nil
		},
	})

	// Lambda invocation on Input change (because ConcealInput is false)
	configParam.Input = "a-new-input-value"
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			rs := getTestResourceState(s)
			assert.Equal(t, getInputJson("a-new-input-value"), rs.Attributes["input"])
			return nil
		},
	})

	// Invoke finalizer function when deleted
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, true)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: " ", // Destroy: true,
		Check: func(s *terraform.State) error {
			assert.Nil(t, getTestResourceState(s))
			return nil
		},
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		PreCheck:          preCheck,
		ProviderFactories: createMockProviderFactories(m),
		Steps:             steps,
	})
}

func TestLambdaBasedResource_concealedInput(t *testing.T) {
	m, c := createMockLambdaClient(t)
	defer c.Finish()
	var steps []resource.TestStep

	configParam := newConfigParameters()
	configParam.ConcealInput = true
	configParam.FinalizerBlockOn = false

	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			rs := getTestResourceState(s)
			assert.Equal(t, "", rs.Attributes["input"])     // input is concealed
			assert.NotEqual(t, "", rs.Attributes["result"]) // result is not concealed
			return nil
		},
	})

	// Input change doesn't trigger lambda becaue input is concealed
	configParam.Input = "a-new-input-val"
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			rs := getTestResourceState(s)
			assert.Equal(t, "", rs.Attributes["input"])
			assert.NotEqual(t, "", rs.Attributes["result"]) // result is not concealed
			return nil
		},
	})

	// A change in triggers should invoke lambda
	configParam.TriggerParameter = "trigger-now"
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			rs := getTestResourceState(s)
			assert.Equal(t, "", rs.Attributes["input"])
			assert.NotEqual(t, "", rs.Attributes["result"]) // result is not concealed
			return nil
		},
	})

	// Unconceal
	configParam.ConcealInput = false
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			rs := getTestResourceState(s)
			assert.Equal(t, getInputJson("a-new-input-val"), rs.Attributes["input"])
			assert.NotEqual(t, "", rs.Attributes["result"]) // result is not concealed
			return nil
		},
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		PreCheck:          preCheck,
		ProviderFactories: createMockProviderFactories(m),
		Steps:             steps,
	})
}

func TestLambdaBasedResource_concealedResult(t *testing.T) {
	m, c := createMockLambdaClient(t)
	defer c.Finish()
	var steps []resource.TestStep

	configParam := newConfigParameters()
	configParam.ConcealResult = true
	configParam.FinalizerBlockOn = false

	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			rs := getTestResourceState(s)
			assert.NotEqual(t, "", rs.Attributes["input"]) // input is concealed
			assert.Equal(t, "", rs.Attributes["result"])   // result is not concealed
			return nil
		},
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		PreCheck:          preCheck,
		ProviderFactories: createMockProviderFactories(m),
		Steps:             steps,
	})
}

func TestLambdaBasedResource_lambdaError(t *testing.T) {
	m, c := createMockLambdaClient(t)
	defer c.Finish()
	var steps []resource.TestStep

	configParam := newConfigParameters()
	configParam.FinalizerBlockOn = false

	// Failing a newly created resource (error while calling)
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), fmt.Errorf("this-error-is-expected"))
	steps = append(steps, resource.TestStep{
		Config:      generateTestConfig(configParam),
		ExpectError: regexp.MustCompile("this-error-is-expected"),
	})

	// Expect an non-empty plan since previous apply failed
	steps = append(steps, resource.TestStep{
		Config:             generateTestConfig(configParam),
		PlanOnly:           true,
		ExpectNonEmptyPlan: true,
	})

	// Failing a newly created resource (lambda returning error)
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(true), nil)
	steps = append(steps, resource.TestStep{
		Config:      generateTestConfig(configParam),
		ExpectError: regexp.MustCompile("result-val"),
	})

	// Expect an non-empty plan since previous apply failed
	steps = append(steps, resource.TestStep{
		Config:             generateTestConfig(configParam),
		PlanOnly:           true,
		ExpectNonEmptyPlan: true,
	})

	// Succeed this time
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			assert.NotNil(t, getTestResourceState(s))
			return nil
		},
	})

	// Expect an empty plan since previous apply succeded
	steps = append(steps, resource.TestStep{
		Config:             generateTestConfig(configParam),
		PlanOnly:           true,
		ExpectNonEmptyPlan: false,
	})

	configParam.Input = "a-new-input-val"
	// Failing an existing resource (error while calling)
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), fmt.Errorf("this-error-is-expected"))
	steps = append(steps, resource.TestStep{
		Config:      generateTestConfig(configParam),
		ExpectError: regexp.MustCompile("this-error-is-expected"),
	})

	// Expect an non-empty plan since previous apply failed
	steps = append(steps, resource.TestStep{
		Config:             generateTestConfig(configParam),
		PlanOnly:           true,
		ExpectNonEmptyPlan: true,
	})

	// Failing an existing resource (lambda returning error)
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(true), nil)
	steps = append(steps, resource.TestStep{
		Config:      generateTestConfig(configParam),
		ExpectError: regexp.MustCompile("result-val"),
	})

	// Expect an non-empty plan since previous apply failed
	steps = append(steps, resource.TestStep{
		Config:             generateTestConfig(configParam),
		PlanOnly:           true,
		ExpectNonEmptyPlan: true,
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		PreCheck:          preCheck,
		ProviderFactories: createMockProviderFactories(m),
		Steps:             steps,
	})
}

func TestLambdaBasedResource_finalizerLambdaError(t *testing.T) {
	m, c := createMockLambdaClient(t)
	defer c.Finish()
	var steps []resource.TestStep

	configParam := newConfigParameters()

	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, false)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: generateTestConfig(configParam),
		Check: func(s *terraform.State) error {
			assert.NotNil(t, getTestResourceState(s))
			return nil
		},
	})

	// Expect an empty plan since we successfully created the resource
	steps = append(steps, resource.TestStep{
		Config:             generateTestConfig(configParam),
		PlanOnly:           true,
		ExpectNonEmptyPlan: false,
	})

	// Failing an existing resource (error while calling)
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, true)).Return(createLambdaInvokeOutput(false), fmt.Errorf("this-error-is-expected"))
	steps = append(steps, resource.TestStep{
		Config:      " ",
		ExpectError: regexp.MustCompile("this-error-is-expected"),
	})

	// Expect an non-empty plan since previous apply failed
	steps = append(steps, resource.TestStep{
		Config:             " ",
		PlanOnly:           true,
		ExpectNonEmptyPlan: true,
	})

	// Failing an existing resource (lambda returning error)
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, true)).Return(createLambdaInvokeOutput(true), nil)
	steps = append(steps, resource.TestStep{
		Config:      " ",
		ExpectError: regexp.MustCompile("result-val"),
	})

	// Expect an non-empty plan since previous apply failed
	steps = append(steps, resource.TestStep{
		Config:             " ",
		PlanOnly:           true,
		ExpectNonEmptyPlan: true,
	})

	// Succeed this time
	m.EXPECT().Invoke(gomock.Any(), createLambdaInvokeInput(configParam, true)).Return(createLambdaInvokeOutput(false), nil)
	steps = append(steps, resource.TestStep{
		Config: " ",
		Check: func(s *terraform.State) error {
			assert.Nil(t, getTestResourceState(s))
			return nil
		},
	})

	// Expect an empty plan since we successfully destroyed the resource
	steps = append(steps, resource.TestStep{
		Config:             " ",
		PlanOnly:           true,
		ExpectNonEmptyPlan: false,
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		PreCheck:          preCheck,
		ProviderFactories: createMockProviderFactories(m),
		Steps:             steps,
	})
}

//      .-.     .-.     .-.     .-.     .-.     .-.     .-.
// `._.'   `._.'   `._.'   `._.'   `._.'   `._.'   `._.'   `._.'
//
//                     Utility functions
//
//  .-.     .-.     .-.     .-.     .-.     .-.     .-.     .-.
// '   `._.'   `._.'   `._.'   `._.'   `._.'   `._.'   `._.'   `

func preCheck() {
	env, _ := schema.MultiEnvDefaultFunc([]string{"AWS_REGION", "AWS_DEFAULT_REGION"}, "")()
	if env == "" {
		os.Setenv("AWS_REGION", "dummy-region")
	}
}

func createMockLambdaClient(t *testing.T) (*mocks.MockLambdaClient, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	return mocks.NewMockLambdaClient(ctrl), ctrl
}

func createMockProviderFactories(lambdaClient LambdaClient) map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"lambdabased": func() (*schema.Provider, error) {
			p := createProvider(func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
				return lambdaClient, nil
			})
			raw := map[string]interface{}{"region": "us-east-1"}
			err := p.Configure(context.Background(), terraform.NewResourceConfigRaw(raw))
			if err != nil {
				log.Fatal(err)
			}
			return p, nil
		},
	}
}

func generateTestConfig(params configParameters) string {
	t := template.New("LambdaBasedResourceTest")
	t.Parse(`
		resource "lambdabased_resource" "test" {
			function_name = "{{.FunctionName}}"
			triggers = { trig_key = "{{.TriggerParameter}}" }
			qualifier = "{{.Qualifier}}"
			input = "{\"param\":\"{{.Input}}\"}"
			conceal_input = {{.ConcealInput}}
			conceal_result = {{.ConcealResult}}
			{{if .FinalizerBlockOn}}
			finalizer {
				function_name = "{{.FinalizerFunctionName}}"
				qualifier = "{{.FinalizerQualifier}}"
				input = "{\"param\":\"{{.FinalizerInput}}\"}"
			}
			{{end}}
		}`)
	var tpl bytes.Buffer
	t.Execute(&tpl, params)
	return tpl.String()
}

func getTestResourceState(s *terraform.State) *terraform.InstanceState {
	rs, ok := s.RootModule().Resources["lambdabased_resource.test"]
	if ok {
		return rs.Primary
	}
	return nil
}

type configParameters struct {
	FunctionName     string
	TriggerParameter string
	Input            string
	Qualifier        string
	ConcealInput     bool
	ConcealResult    bool

	FinalizerBlockOn      bool
	FinalizerFunctionName string
	FinalizerInput        string
	FinalizerQualifier    string
}

func newConfigParameters() configParameters {
	return configParameters{
		FunctionName:          "func-createupdate-name-1",
		TriggerParameter:      "trigger-param-val",
		Input:                 "createupdate-input-param-val",
		Qualifier:             "$LATEST",
		ConcealInput:          false,
		ConcealResult:         false,
		FinalizerBlockOn:      true,
		FinalizerFunctionName: "func-destroy-name-1",
		FinalizerInput:        "destroy-input-param-val",
		FinalizerQualifier:    "$LATEST",
	}
}

func getInputJson(param string) string {
	return fmt.Sprintf("{\"param\":\"%s\"}", param)
}

func createLambdaInvokeInput(cp configParameters, forFinalizer bool) *lambda.InvokeInput {
	ret := &lambda.InvokeInput{
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
	}

	if !forFinalizer {
		ret.FunctionName = &cp.FunctionName
		ret.Qualifier = &cp.Qualifier
		ret.Payload = []byte(getInputJson(cp.Input))
	} else {
		if cp.FinalizerBlockOn {
			ret.FunctionName = &cp.FinalizerFunctionName
			ret.Qualifier = &cp.FinalizerQualifier
			ret.Payload = []byte(getInputJson(cp.FinalizerInput))
		}
	}
	return ret
}

func createLambdaInvokeOutput(functionError bool) *lambda.InvokeOutput {
	funcErrStr := "lambda-return-expected-error"
	funcErr := &funcErrStr
	if !functionError {
		funcErr = nil
	}
	return &lambda.InvokeOutput{
		FunctionError: funcErr,
		Payload:       []byte("result-val"),
	}
}
