apiVersion: landscaper.gardener.cloud/v1alpha1
kind: DataObject
metadata:
  name: config
  namespace: ${namespace}
data:
  clusterblue:
    color: blue
    cpu: 100m
    memory: 100Mi

  clustergreen:
    color: green
    cpu: 120m
    memory: 120Mi

  clusteryellow:
    color: yellow
    cpu: 140m
    memory: 140Mi

  clusterorange:
    color: orange
    cpu: 160m
    memory: 160Mi

  clusterred:
    color: red
    cpu: 180m
    memory: 180Mi

