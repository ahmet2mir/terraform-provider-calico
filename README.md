# Calico Terraform Provider

## About

Terraform provider for use with Calico > 3

I just need the IPPool resource for now until Calico fix server-side apply https://github.com/projectcalico/calico/issues/4959

Once fixed it will be possible to use directly `kubernetes_manifest`

Tested with terraform 1.0.5 and Calcio 3.21.4

## Usage

```
provider "calico" {
  # See kubernetes provider https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs
  # to see all possibilities, for the example we use a simple kubeconfig file
  kubernetes {
    config_path = "/tmp/kubeconfig"
  }
}

resource "calico_ippool" "test" {
  metadata {
    name = "test"
  }
  spec {
    cidr               = "10.144.1.0/24" # no update
    block_size         = 27 # no update
    disable_bgp_export = false # update allowed
    disabled           = false # update allowed
    nat_outgoing       = true # update allowed
    ipip_mode          = "Always" # conflict with vxlan_mode
    # vxlan_mode        = "Never" # conflict with ipip_mode
  }
}
```

## Develop

Build and run in debug

```
make build
./terraform-provider-calico -debug
```

Will return something like

```
{"@level":"debug","@message":"plugin address","@timestamp":"2022-03-01T17:55:39.782596+01:00","address":"/tmp/plugin4289848107","network":"unix"}
Provider started, to attach Terraform set the TF_REATTACH_PROVIDERS env var:

  TF_REATTACH_PROVIDERS='{"registry.terraform.io/ahmet2mir/calico":{"Protocol":"grpc","ProtocolVersion":5,"Pid":19958,"Test":true,"Addr":{"Network":"unix","String":"/tmp/plugin4289848107"}}}'

```

export `TF_REATTACH_PROVIDERS` and run usual terraform init/plan/apply commands
