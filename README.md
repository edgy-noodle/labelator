# Labelator

Labelator is a Kubernetes operator designed to manage labels on namespaces declaratively. It allows administrators to define desired labels for specific namespaces, or all namespaces, using custom `Label` resource.

This project was created using [Operator SDK](https://sdk.operatorframework.io/).

## Features

- **Declarative Label Management**: Define the desired state of namespace labels in a simple YAML manifest.
- **Cluster-Scoped Rules**: Apply a common set of labels to multiple namespaces across the entire cluster using the `Label` resource.
- **Wildcard Support**: Use `"*"` to easily target all namespaces in the cluster.
- **Namespace Exclusion**: Exclude specific system or critical namespaces from wildcard labeling.
- **Automatic Cleanup**: Optionally configure labels to be automatically removed when the corresponding `Label` resource is deleted.

## Custom Resource Definitions (CRDs)

### `Label`

The `Label` resource is cluster-scoped and is used to apply labels to one or more namespaces across the cluster.

#### Example: Wildcard with Exclusion

This example applies the `environment=production` label to **all namespaces** except for `kube-system` and `monitoring`. It also enables the cleanup feature.

```yaml
apiVersion: namespace.labelator.io/v1alpha1
kind: Label
metadata:
  name: production-labels
spec:
  # Use "*" to target all namespaces
  namespaces:
    - "*"
  # Exclude specific namespaces from the wildcard
  excludedNamespaces:
    - kube-system
    - monitoring
  # Labels to apply
  labels:
    environment: production
  # Remove these labels when this resource is deleted
  cleanupOnDelete: true
```

#### Example: Specific Namespaces

This example applies `team=backend` and `billing-code=abc` to the `api-services` and `data-processing` namespaces.

```yaml
apiVersion: namespace.labelator.io/v1alpha1
kind: Label
metadata:
  name: backend-team-labels
spec:
  namespaces:
    - api-services
    - data-processing
  labels:
    team: backend
    billing-code: "abc"
  cleanupOnDelete: false
```

## Getting Started

### Prerequisites

- a Kubernetes cluster (e.g., Kind, Minikube, or a cloud provider)
- `kubectl` configured to connect to your cluster
- `make` installed

### Deployment

1. Deploy the Controller

    ```bash
    make deploy
    ```

2. Apply a `Label` resource to start labeling your namespaces

    ```bash
    kubectl apply -f <MANIFEST>
    ```

## LICENSE

This project is licensed under the [Apache 2.0 license](LICENSE).
