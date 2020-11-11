# k8sutil

## Overview

This command line utility can be used for dumping kubernetes resources from a cluster and creating secrets in bulk.
More commands will likely be added in the future. This is primarily used to automate certain tedious tasks I've encountered in my day to day at Rancher.

## Installation

`go install github.com/ryansann/k8sutil`

## Mocksecrets

#### Help
`k8sutil mocksecrets -h`

#### Example
`k8sutil --kube-config <path> --debug mocksecrets --num-secrets 1000 --num-workers 50 --namespace secrets-testing --secret-size 150`

If the specified namespace does not exist, it will be created.

## Dump

#### Help
`k8sutil dump -h`

#### Example
`k8sutil --kube-config <path> --debug dump --config <path>`

### Filters

Each resource dump is defined by a group version resource (gvr), namespace, and filters:

```yaml
dumps:
  - gvr:
      group: management.cattle.io
      version: v3
      resource: users
    namespace: ''
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

See [this file](example/dump.yaml) for a more complete example.