# AI Conformance End-To-End (E2E) Tests

## Usage

To run this plugin, run the following command:
```bash
sonobuoy run --plugin https://raw.githubusercontent.com/carlory/sonobuoy-plugins/master/ai-conformance/plugin.yaml
```

output like this:
```
INFO[0000] create request issued                         name=sonobuoy namespace= resource=namespaces
INFO[0000] create request issued                         name=sonobuoy-serviceaccount namespace=sonobuoy resource=serviceaccounts
INFO[0000] create request issued                         name=sonobuoy-serviceaccount-sonobuoy namespace= resource=clusterrolebindings
INFO[0000] create request issued                         name=sonobuoy-serviceaccount-sonobuoy namespace= resource=clusterroles
INFO[0000] create request issued                         name=sonobuoy-config-cm namespace=sonobuoy resource=configmaps
INFO[0000] create request issued                         name=sonobuoy-plugins-cm namespace=sonobuoy resource=configmaps
INFO[0000] create request issued                         name=sonobuoy namespace=sonobuoy resource=pods
INFO[0000] create request issued                         name=sonobuoy-aggregator namespace=sonobuoy resource=services
```

The plugin status can be checked using the command:
```bash
sonobuoy status
```

output like this:
```
           PLUGIN    STATUS   RESULT   COUNT   PROGRESS
   ai-conformance   running                1

Sonobuoy is still running. Runs can take 60 minutes or more depending on cluster and plugin configuration.
```

Once the plugin is complete, the status will be like this:
```
           PLUGIN     STATUS   RESULT   COUNT                 PROGRESS
   ai-conformance   complete   passed       1   Passed:  9, Failed:  0

Sonobuoy has completed. Use `sonobuoy retrieve` to get results.
```

Retrieve the results using the command:
```bash
sonobuoy retrieve ./results
```

output like this:
```
results/202509120807_sonobuoy_1bb476a7-bc67-44ad-905d-7637ebfb1da8.tar.gz
```

After extracting the archive, it should contain the following directory structure:

```
...
plugins
└── ai-conformance
    ├── definition.json
    ├── results
    │   └── global
    │       └── out.json
    └── sonobuoy_results.yaml
...
```

The `out.json` file is generated to answer the [self-certification questionnaire template](https://github.com/cncf/ai-conformance/blob/main/self-cert-questionnaire-template.yaml), you can use it as evidence to self-certify.