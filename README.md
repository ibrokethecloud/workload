## kubectl-workload

A simple plugin to stop / start workloads by scaling them to 0 and restoring them back to their original state.

```cassandraql
▶ kubectl workload -h
The plugin interacts with the k8s api to generate a list of workloads in the specified
	namespaces. k8s has no concept of stopping / starting workloads. In certain scenarios, this may be required.
	the plugin performs a stop by saving the state of the workload in a configmap before scaling it to 0, and then
	using the same saved stage to start the workload by restoring the scale.

Usage:
  kubectl-workload [flags]

Flags:
  -a, --all-kinds          operate on all deployments and statefulsets
  -h, --help               help for kubectl-workload
  -n, --namespace string   namespace (default "default")
      --start              scale down specified workloads
      --stop               scale down specified workloads
```

Actual state for the workloads is stored in a configMap named snap-backup in the same namespace

```cassandraql
▶ kubectl get cm snap-backup -o yaml
apiVersion: v1
data:
  deployment-wordpress-t657b: '{"name":"wordpress-t657b","scale":1,"kind":"deployment"}'
  statefulset-wordpress-t657b-mariadb: '{"name":"wordpress-t657b-mariadb","scale":1,"kind":"statefulset"}'
kind: ConfigMap
metadata:
  creationTimestamp: "2020-07-20T07:58:42Z"
  name: snap-backup
  namespace: wordpress
  resourceVersion: "5190935"
  selfLink: /api/v1/namespaces/wordpress/configmaps/snap-backup
  uid: ef763368-5300-4eaa-97ee-37aad3aec2d9
```

The plugin allows users to only manage statefulsets and deployments.

### Sample usage scenarios

#### Scale down deployments
```cassandraql
kubectl workload deployment deployment1 deployment2... -n namespace --stop
```

#### Scale down statefulsets
```cassandraql
kubectl workload statefulset sts1 sts2... -n namespace --stop
```

#### Scale up deployments
```cassandraql
kubectl workload deployment deployment1 deployemnt2... -n namespace --start
```

#### Scale up statefulsets
```cassandraql
kubectl workload statefulset sts1 sts2... -n namespace --start
```

#### Scale down all deployments and statefulsets in a namespace
```cassandraql
kubectl workload -a --stop -n namespace
```

#### Scale up all deployments and statefulsets in a namespace
```cassandraql
kubectl workload -a --start -n namespace
```