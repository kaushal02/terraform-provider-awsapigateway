# Terraform AWS-API-Gateway-resource Provider

The AWS-API-Gateway-Resource Provider is a plugin for Terraform that allows working with AWS API Gateways. This provider is maintained by Traceableai.

For a more comprehensive explanation see [awsapigateway_resource](./docs/resources/awsapigateway_resource.md) documentation.

## Usage

```hcl
terraform {
  required_providers {
    awsapigateway = {
      source  = "kaushal02/awsapigateway"
      version = "~> 0.1.0"
    }
  }
}

provider "awsapigateway" {
  region = "us-east-1"
}
```

See the complete example [here](./examples/default)

## Testing

```shell
make test
```
