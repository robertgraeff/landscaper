apiVersion: landscaper.gardener.cloud/v1alpha1
kind: DataObject
metadata:
  name: config
  namespace: ${namespace}
data:
  bluecluster:
    cpu: 100m
    memory: 100Mi

  greencluster:
    cpu: 120m
    memory: 120Mi

  yellowcluster:
    cpu: 140m
    memory: 140Mi

  orangecluster:
    cpu: 160m
    memory: 160Mi

  redcluster:
    cpu: 180m
    memory: 180Mi

