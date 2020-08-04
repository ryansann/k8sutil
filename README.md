# k8sdump

## Overview

This command can be used to dump Kubernetes resources from a cluster. It reads a config file to determine where your kubeconfig 
file is and what resources you want dumped. I built this for debugging and dumping related k8s resources for analysis.

## Build

Use: `make build`

## Run

If you want to use the default config file location, `./k8sdump.yaml` then you can use: `make run`

Otherwise you can use: 
- `make build`
- `k8sdump --config-file <path>`

## Filters

Each resource dump is defined by a GroupVersionResource and Namespace:

```yaml
dumps:
  - gvr:
      group: management.cattle.io
      version: v3
      resource: users
    filters:
      ors:
        - key: username
          value: alice
        - key: username
          value: bob
      ands:
        - key: key
          value: val
```

If namespace is omitted, as it is above, it will default to dumping the GVRs from all namespaces. The filters section defines 
a way for the list of GVRs to be filtered after it is retrieved from the api server. The filters key should be a [gjson](https://github.com/tidwall/gjson) path
that evaluates to a string. In order for a resource to pass the filtering criteria, it must satisfy at least 1 or condition as well as all and conditions.