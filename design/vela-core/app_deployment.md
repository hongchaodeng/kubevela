# Multi-Version, Multi-Cluster Application Deployment Design

## Background

Currently Vela supports Application CRD which templates the low level resources and exposes high level parameters to users. But that's not enough. It requires a couple of standard techniques to deploy an application in production:

1. **Rolling upgrade** (aka rollout): To continuously deploy apps requires to rollout in a safe manner which usually involves step by step rollout batches and analysis.
2. **Traffic shifting**: When rolling upgrade an app, it needs to split the traffic onto both the old and new revisions to verify the new version while preserving service availability.
3. **Multi-cluster**: Modern application infrastructure involves multiple clusters to ensure high availability and maximize service throughput.

## Proposal

This issue proposes to add a new ApplicationDeployment CRD to satisfy the above requirements.

```yaml
kind: ApplicationDeployment
name: example-app
spec:
  trafficRules:
    - name: web
      match:
        - uri:
            prefix: "/web"
      weightedTargets:
        - revisionName: 1
          percent: 50
        - revisionName: 2
          percent: 50

    - name: mobile
      ...

  appRevisions:
    - name: 1

      # Reference to the underlying data of the spec.
      # Each modification to Application would generate a new AppRevision.
      # The AppDeployment controller will use the AppRev name as the reference.
      reference: example-app-v1

      # Cluster specific workload placement config
      placement:
        - clusterSelector:
            # You can select Clusters by name or labels.
            # If multiple clusters is selected, one will be picked by some hashing algorithm based on component name.
            labels:
              usage: production
            name: production

          # Distributes 50% workload to this cluster.
          # The total replicas will be derived from the underlying workload, e.g. Deployment spec.replicas.
          distribution:
            weight: 50

        - # If no clusterSelector is given, it will use the same cluster as this CR
          distribution:
            weight: 50

      # Service publication configuration
      service:
        http:
          host: example-app-v1.aliyun.com
          tls:
            mode: mutual

    - name: 2
      ref: example-app-v2
      # No placement specified means distributing 100% workload to the same cluster as this CR
      service:
        http:
          host: example-app-v2.aliyun.com
          tls: ..
```

In above proposal, the `placementStrategies` part requires a Cluster CRD that are proposed as below:

```yaml
kind: Cluster
metadata:
  name: prod-cluster-1
  labels:
    usage: production
spec:
  kubeconfig: ...
```

## Technical details

Here are some ideas how we can implement the above API.

First of all, we will add a new AppDeployment controller to do the reconcile logic. For each feature they are implemented as follows: 

### 1. Rollout


In the following example, we are assuming the app has deployed v1 now and is upgrading to v2. Here are the workflow:

- User modifies Application to trigger revision change.
  - Add annotation `app.oam.dev/rollout-template=true` to create a new revision instead of replace existing one.
- User gets the names of the v1 and v2 AppRevision to complete AppDeployment spec.
- User applies AppDeployment which includes both the v1 and v2 revisions.
- The AppDeployment controller will calculate the diff between previous and current AppDeployment specs. The diff consists of three parts:
  - Del: the revisions that existed before and do not exist in the new spec. They should be scaled to 0 and removed.
  - Mod: The revisions that still exist but needs to be changed.
  - Add: The revisions that did not exist before and will be deployed fresh new.
- After the diff calculation, the AppDeployment controller will execute the plan as follows:
  - Handle Del: remove revisions.
  - Handle Add/Mod: handle distribution, handle scaling. A special case is in current cluster no need to do distribution.

### 2. Traffic

We will make sure the spec works for the following environments:

- K8s ingress + service (traffic split percetange determined by replica number)
- Istio service mesh

Here is the workflow with Istio:

- User applies AppDeployment to split traffic between v1 and v2 each 50%
- The AppDeployment controller will create VirtualService object:

  ```yaml
  apiVersion: networking.istio.io/v1alpha3
  kind: VirtualService
  spec:
    http:
      - match:
        - uri:
            prefix: "/web"

        route:
        - destination:
            host: example-app-v1
          weight: 50
        - destination:
            host: example-app-v2
          weight: 50
  ```

> Note: The service name is a convention which could be inferred from the app name.

Here is the workflow with Ingress:

- User applies AppDeployment to split traffic between v1 and v2, but didn't and shouldn't specify `percent`.
- The AppDeployment controller will create/update Ingress object:
  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: Ingress
  spec:
    rules:
    - http:
        paths:
        - path: /v1
          pathType: prefix
          backend:
            service:
              name: example-app-v1
              port:
                number: 80
        
        - path: /v2
          pathType: prefix
          backend:
            service:
              name: example-app-v2
              port:
                number: 80
  ```

### 3. Multi Cluster

We will implement the logic inside the AppDeployment controller itself.

Here is the workflow:

- User applies AppDeployment with `placement` of each revision.
- The AppDeployment controller will select the clusters, get their credentials.
  Then deployed specified number of replicas to that cluster.

## Considerations

- Build multi-stage rollout strategies like [argo-progressive-rollout](https://github.com/Skyscanner/argocd-progressive-rollout/)
- AppDeployment could adopt native k8s workloads (e.g. Deployment, Statefulset) in the future.
