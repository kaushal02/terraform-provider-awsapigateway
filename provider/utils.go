package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"strings"
)

func contains(arr []string, val string) bool {
	for _, a := range arr {
		if a == val {
			return true
		}
	}
	return false
}

func removeDuplicates(arr []string) []string {
	var newArr []string
	chkMap := make(map[string]bool)
	for _, a := range arr {
		chkMap[a] = true
	}
	for a, _ := range chkMap {
		newArr = append(newArr, a)
	}
	return newArr
}

func getExecutionLogGroupName(apiId string, stageName string) string {
	return fmt.Sprintf("API-Gateway-Execution-Logs_%s/%s", apiId, stageName)
}

func getAccessLogGroupNameFromArn(arn string) string {
	// arn:aws:logs:REGION:ACCOUNT_ID:log-group:LOG_GROUP_NAME
	return strings.Join(strings.Split(arn, ":")[6:], ":")
}

func errorDiagnostic(summary string) *diag.Diagnostic {
	return newDiagnostic(diag.Error, summary)
}

func warnDiagnostic(summary string) *diag.Diagnostic {
	return newDiagnostic(diag.Warning, summary)
}

func newDiagnostic(severity diag.Severity, summary string) *diag.Diagnostic {
	return &diag.Diagnostic{
		Severity: severity,
		Summary:  summary,
	}
}

func errorDiagnostics(summary string) diag.Diagnostics {
	return newDiagnostics(diag.Error, summary)
}

func newDiagnostics(severity diag.Severity, summary string) diag.Diagnostics {
	return diag.Diagnostics{*newDiagnostic(severity, summary)}
}
