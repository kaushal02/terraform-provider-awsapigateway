terraform {
  required_providers {
	awsapigateway = {
	  source  = "kaushal02/awsapigateway"
	}
  }
}

resource "awsapigateway_resource" "test" {
  api_gateways = []
  action       = "exclude"
}
