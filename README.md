# meta-operator

The ultimate pocket knife for Kubernetes. Meta-operator allows you to create operators for gluing stuff together using only [ytt](https://github.com/carvel-dev/ytt).

You provide a YAML configuration file consisting of one (or more) GroupVersionKind's and a path to a directory containing ytt templated YAML files. Meta-operator will then evaluate the ytt templates (passing the object as data) and apply the resulting YAML's to the cluster. You can't write operators that will change anything outside of Kubernetes, but you can use meta-operator to create operators that glue stuff together.

Sometimes wrestling with kubebuilder just isn't worth it.