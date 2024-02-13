apiVersion: landscaper.gardener.cloud/v1alpha1
kind: Installation
metadata:
  name: targetlist-ref
  namespace: ${namespace}
  annotations:
    landscaper.gardener.cloud/operation: reconcile

spec:
  context: landscaper-examples

  componentDescriptor:
    ref:
      componentName: github.com/gardener/guided-tour/targetlists/guided-tour-targetlist-ref
      version: 1.0.0

  blueprint:
    ref:
      resourceName: blueprint-root

  imports:
    targets:
      - name: rootclusters
        targets:
          - cluster-red
          - cluster-green
          - cluster-blue
    data:
      - name: rootconfig
        dataRef: config

  importDataMappings:
    namespace: ${targetNamespace}
