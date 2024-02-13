apiVersion: landscaper.gardener.cloud/v1alpha1
kind: DataObject
metadata:
  name: config
  namespace: ${namespace}
data:
  cluster-blue:
    color: blue
    cpu: 100m
    memory: 100Mi

  cluster-green:
    color: green
    cpu: 120m
    memory: 120Mi

  cluster-yellow:
    color: yellow
    cpu: 140m
    memory: 140Mi

  cluster-orange:
    color: orange
    cpu: 160m
    memory: 160Mi

  cluster-red:
    color: red
    cpu: 180m
    memory: 180Mi

