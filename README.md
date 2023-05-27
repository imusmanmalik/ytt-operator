# meta-operator

The ultimate pocket knife for Kubernetes. Meta-operator allows you to create operators using only [ytt](https://github.com/carvel-dev/ytt).

You provide a YAML config file consisting of one (or more) GroupVersionKind's and a path to a directory containing ytt templated YAML files. Meta-operator will then evaluate the ytt templates (passing the original object as a data value) and apply the resulting YAML's to the cluster. Meta-operator is particularly useful for creating abstractions on top of existing operators.

Sometimes wrestling with kubebuilder just isn't worth it.

## Installation

Note: As meta-operator does not know what resource kinds you will be watching or creating at build time, you will need to create a custom ClusterRole for your application (this has been omitted).

```bash
$ kubectl apply -k config/default
```