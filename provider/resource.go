package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/aws/aws-sdk-go-v2/service/apigateway"
	v2 "github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func AwsApiGatewayResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCreateUpdate,
		ReadContext:   resourceRead,
		UpdateContext: resourceCreateUpdate,
		DeleteContext: resourceDelete,

		Schema: map[string]*schema.Schema{
			"api_gateways": {
				Type:     schema.TypeList,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"action": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      string(INCLUDE),
				ValidateFunc: validation.StringInSlice(ApiGatewayActions, false),
			},
			"identifier": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"ignore_access_log_settings": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"log_group_names": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceCreateUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	apiGateways := d.Get("api_gateways").([]interface{})
	action := d.Get("action").(string)
	ignoreAccessLogSettings := d.Get("ignore_access_log_settings").(bool)
	diagnostics, logGroupNames := checkApiGateways(apiGateways, action == string(EXCLUDE), ignoreAccessLogSettings, meta)
	if d.Id() == "" {
		d.SetId(uuid.New().String())
	}
	if err := d.Set("log_group_names", logGroupNames); err != nil {
		return diag.FromErr(err)
	}
	return diagnostics
}

func resourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}

func checkApiGateways(apiGateways []interface{}, exclude bool, ignoreAccessLogSettings bool, meta interface{}) (diag.Diagnostics, []string) {
	var diagnostics diag.Diagnostics
	var summary string
	if !exclude && len(apiGateways) == 0 {
		summary = "api_gateways cannot be empty when action is include."
		return errorDiagnostics(summary), []string{}
	}

	// apiAllStages stores api ids where all stages need to be considered
	// apiWithStage is a map of api id to list of api stages that need to be considered
	// any api id can only belong to either apiAllStages slice or apiWithStage map
	var apiAllStages []string
	apiWithStage := make(map[string][]string)
	for _, elem := range apiGateways {
		apiDetails := strings.Split(elem.(string), "/")
		if len(apiDetails) == 2 {
			if !contains(apiAllStages, apiDetails[0]) {
				apiWithStage[apiDetails[0]] = append(apiWithStage[apiDetails[0]], apiDetails[1])
			}
		} else if len(apiDetails) == 1 {
			apiAllStages = append(apiAllStages, apiDetails[0])
			if _, ok := apiWithStage[apiDetails[0]]; ok {
				delete(apiWithStage, apiDetails[0])
			}
		} else {
			summary = fmt.Sprintf("api gateway syntax is wrong for %s", elem)
			diagnostics = append(diagnostics, *errorDiagnostic(summary))
		}
	}

	conn := meta.(AwsApiGatewayProvider)
	restApiGatewaysDiagnostics, logGroupNames := checkRestApiGateways(conn, apiAllStages, apiWithStage, exclude, ignoreAccessLogSettings)
	apiGatewayV2Diagnostics, apiGatewayV2LogGroupNames := checkApiGatewaysV2(conn, apiAllStages, apiWithStage, exclude, ignoreAccessLogSettings)
	diagnostics = append(diagnostics, restApiGatewaysDiagnostics...)
	diagnostics = append(diagnostics, apiGatewayV2Diagnostics...)
	return diagnostics, removeDuplicates(append(logGroupNames, apiGatewayV2LogGroupNames...))
}

func checkRestApiGateways(conn AwsApiGatewayProvider, apiAllStages []string, apiWithStage map[string][]string, exclude bool, ignoreAccessLogSettings bool) (diag.Diagnostics, []string) {
	var diagnostics diag.Diagnostics
	var summary string
	// apiStageMappingRest is a map of api id to list of api stages that need to be considered
	// if the value list is empty, it means that all stages in this api should be considered
	apiStageMappingRest := make(map[string][]string)
	restApisPaginator := conn.getAwsGetRestApisPaginator()
	for restApisPaginator.HasMorePages() {
		res, err := restApisPaginator.NextPage(context.TODO())
		if err != nil {
			summary = fmt.Sprintf("Error while invoking getRestApis sdk call: %s", err.Error())
			diagnostics = append(diagnostics, *errorDiagnostic(summary))
		}
		for _, restApi := range res.Items {
			apiId := *restApi.Id
			apiStages, partial := apiWithStage[apiId]
			if partial {
				apiStageMappingRest[apiId] = apiStages
			} else if contains(apiAllStages, apiId) != exclude {
				apiStageMappingRest[apiId] = []string{}
			}
		}
	}
	stagesDiagnostics, logGroupNames := checkRestApiGatewayStages(conn, apiStageMappingRest, exclude, ignoreAccessLogSettings)
	diagnostics = append(diagnostics, stagesDiagnostics...)
	return diagnostics, logGroupNames
}

func checkApiGatewaysV2(conn AwsApiGatewayProvider, apiAllStages []string, apiWithStage map[string][]string, exclude bool, ignoreAccessLogSettings bool) (diag.Diagnostics, []string) {
	var diagnostics diag.Diagnostics
	var summary string
	// apiStageMappingRest is a map of api id to list of api stages that need to be considered
	// if the value list is empty, it means that all stages in this api should be considered
	apiStageMappingV2 := make(map[string][]string)
	apiGatewayV2Client := conn.getApiGatewayV2Client()
	res, err := apiGatewayV2Client.GetApis(context.TODO(), &v2.GetApisInput{})
	if err != nil {
		summary = fmt.Sprintf("Error while invoking getApis sdk call: %s", err.Error())
		diagnostics = append(diagnostics, *errorDiagnostic(summary))
	}
	for _, httpApi := range res.Items {
		apiId := *httpApi.ApiId
		apiStages, partial := apiWithStage[apiId]
		if partial {
			apiStageMappingV2[apiId] = apiStages
		} else if contains(apiAllStages, apiId) != exclude {
			apiStageMappingV2[apiId] = []string{}
		}
	}
	if !ignoreAccessLogSettings {
		stagesDiagnostics, logGroupNames := checkApiGatewayV2Stages(conn, apiStageMappingV2, exclude)
		diagnostics = append(diagnostics, stagesDiagnostics...)
		return diagnostics, logGroupNames
	}
	return diagnostics, []string{}
}

func checkRestApiGatewayStages(conn AwsApiGatewayProvider, apiStageMappingRest map[string][]string, exclude bool, ignoreAccessLogSettings bool) (diag.Diagnostics, []string) {
	var diagnostics diag.Diagnostics
	var logGroupNames []string
	var summary string
	apiGatewayClient := conn.getApiGatewayClient()
	for apiId, apiStages := range apiStageMappingRest {
		res, err := apiGatewayClient.GetStages(context.TODO(), &v1.GetStagesInput{
			RestApiId: &apiId,
		})
		if err != nil {
			summary = fmt.Sprintf("Error while invoking getStages sdk call: %s", err.Error())
			diagnostics = append(diagnostics, *errorDiagnostic(summary))
		}
		for _, stage := range res.Item {
			stageName := *(stage.StageName)
			if len(apiStages) > 0 && contains(apiStages, stageName) == exclude {
				continue
			}
			if settings, ok := stage.MethodSettings["*/*"]; ok {
				if *(settings.LoggingLevel) == "INFO" && settings.DataTraceEnabled {
					logGroupNames = append(logGroupNames, getExecutionLogGroupName(apiId, stageName))
				} else if *(settings.LoggingLevel) == "INFO" {
					summary = fmt.Sprintf("Full Request and Response Logs not enabled for API %s stage %s", apiId, stageName)
					diagnostics = append(diagnostics, *warnDiagnostic(summary))
				} else if *(settings.LoggingLevel) == "ERROR" {
					summary = fmt.Sprintf("Execution logs set to Errors Only for API %s stage %s", apiId, stageName)
					diagnostics = append(diagnostics, *warnDiagnostic(summary))
				} else {
					summary = fmt.Sprintf("Execution logs not enabled for API %s stage %s", apiId, stageName)
					diagnostics = append(diagnostics, *warnDiagnostic(summary))
				}
			}
			if ignoreAccessLogSettings {
				continue
			}
			if stage.AccessLogSettings == nil || stage.AccessLogSettings.DestinationArn == nil {
				summary = fmt.Sprintf("Access logs not enabled for REST API %s stage %s", apiId, stageName)
				diagnostics = append(diagnostics, *warnDiagnostic(summary))
			} else {
				diagnosticAccessLog := verifyAccessLogFormat(*(stage.AccessLogSettings.Format))
				if diagnosticAccessLog != nil {
					diagnosticAccessLog.Summary = fmt.Sprintf("%s for API %s stage %s", diagnosticAccessLog.Summary, apiId, stageName)
					diagnostics = append(diagnostics, *diagnosticAccessLog)
				} else {
					logGroupNames = append(logGroupNames, getAccessLogGroupNameFromArn(*(stage.AccessLogSettings.DestinationArn)))
				}
			}
		}
	}
	return diagnostics, logGroupNames
}

func checkApiGatewayV2Stages(conn AwsApiGatewayProvider, apiStageMappingV2 map[string][]string, exclude bool) (diag.Diagnostics, []string) {
	var diagnostics diag.Diagnostics
	var logGroupNames []string
	var summary string
	apiGatewayV2Client := conn.getApiGatewayV2Client()
	for apiId, apiStages := range apiStageMappingV2 {
		res, err := apiGatewayV2Client.GetStages(context.TODO(), &v2.GetStagesInput{
			ApiId: &apiId,
		})
		if err != nil {
			summary = fmt.Sprintf("Error while invoking getStages sdk call: %s", err.Error())
			diagnostics = append(diagnostics, *errorDiagnostic(summary))
		}
		for _, stage := range res.Items {
			stageName := *(stage.StageName)
			if len(apiStages) > 0 && contains(apiStages, stageName) == exclude {
				continue
			}
			if stage.AccessLogSettings == nil || stage.AccessLogSettings.DestinationArn == nil {
				summary = fmt.Sprintf("Access logs not enabled for REST API %s stage %s", apiId, stageName)
				diagnostics = append(diagnostics, *warnDiagnostic(summary))
			} else {
				diagnosticAccessLog := verifyAccessLogFormat(*(stage.AccessLogSettings.Format))
				if diagnosticAccessLog != nil {
					diagnosticAccessLog.Summary = fmt.Sprintf("%s for API %s stage %s", diagnosticAccessLog.Summary, apiId, stageName)
					diagnostics = append(diagnostics, *diagnosticAccessLog)
				} else {
					logGroupNames = append(logGroupNames, getAccessLogGroupNameFromArn(*(stage.AccessLogSettings.DestinationArn)))
				}
			}
		}
	}
	return diagnostics, logGroupNames
}

func verifyAccessLogFormat(format string) *diag.Diagnostic {
	var parsed map[string]string
	var foundValues []string
	if err := json.Unmarshal([]byte(format), &parsed); err != nil {
		if err = json.Unmarshal([]byte("{"+format+"}"), &parsed); err != nil {
			return warnDiagnostic("Access log format is not JSON parsable")
		}
	}
	for _, value := range parsed {
		if contains(AccessLogFormatValues, value) {
			foundValues = append(foundValues, value)
		}
	}
	var missingValues []string
	for _, value := range AccessLogFormatValues {
		if !contains(foundValues, value) {
			missingValues = append(missingValues, value)
		}
	}
	if len(missingValues) > 0 {
		return warnDiagnostic(fmt.Sprintf("Access log format is missing required values %v", missingValues))
	}
	return nil
}
