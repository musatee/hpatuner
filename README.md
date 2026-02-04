# HPA Tuner Operator

## Overview

HPA can enter a blind spot during certain outage scenarios—core resource metrics may look normal while application error rates spike in observability platforms like Datadog. In such cases, teams often need to manually increase replicas just to stabilize services.

This project implements a Kubernetes operator designed as an automated first line of defense for those situations.

The operator reconciles a custom resource (CR) with its associated HPA. Reconciliation is triggered by both CR events and a periodic 30-second interval to continuously evaluate external error metrics.

During reconciliation:
- External error-rate metrics are fetched from a monitoring endpoint
- The value is compared against a CR-defined threshold
- When breached:
  - Desired `maxReplicas` is calculated using the current HPA state and a CR-defined `maxCeiling`
  - Desired `minReplicas` is recalculated accordingly
  - The HPA is patched with the new scaling bounds

### Design Principle

Scaling is intentionally **one-directional** — the operator only scales upward.  
This avoids flapping during unstable incidents and preserves capacity until engineers complete root cause analysis and recovery actions.

---

## Local Setup & Testing

### Prerequisites

- kubebuilder v4.10.1  
- Go v1.25.6  
- Local Kubernetes cluster v1.31  
- Metrics Server installed
- A running HPA in the cluster

---

### Custom Resource Example

Create a custom resource manifest:

```yaml
apiVersion: mycrds.akmusa.com/v1alpha1
kind: HpaTuner
metadata:
  name: <name-of-the-cr>
spec:
  hpaName: <hpa-name>
  hpaNamespace: <hpa-namespace>
  metricEndpoint: <metric-endpoint>
  metricThreshold: <threshold-value>
  hpaMaxReplicas: <max-ceiling-replica>
```


### Prepare Dummy Metric Endpoint

1. Deploy a dummy metric service that exposes an error-rate API.
2. Expose the deployment using a **ClusterIP** service.

Example:

```bash
kubectl expose deployment <metric-deployment-name> \
  --type=ClusterIP \
  --name=<metric-service-name> \
  --port=<service-port>
``` 
3. Port-forward the service locally:

```bash
kubectl port-forward svc/<metric-service-name> 7070:<service-port>
```
4. Use the forwarded endpoint in the Custom Resource spec:
```bash
http://localhost:7070/apis/error
```
5. Set this value under: 
```bash
spec:
  metricEndpoint: http://localhost:7070/apis/error 
``` 
### Install CRDs

Install the Custom Resource Definitions into your cluster:

```bash
make install 
```
Verify CRD installation: 
```bash
kubectl get crds | grep hpatuner
``` 
### Apply Custom Resource

Apply the Custom Resource manifest to the cluster:

```bash
kubectl apply -f <your-cr-manifest>.yaml
``` 
Verify that the resource has been created: 
```bash
kubectl get hpatuner
``` 
### Run Controller Locally

Run the controller manager locally against your cluster:

```bash
make run
``` 
The controller will start reconciling:

On Custom Resource events

Every 30 seconds via periodic reconciliation

---

## Dummy Metric Endpoint

For the dummy metric endpoint, check out the following GitHub repository:  
**[https://github.com/musatee/metric-endpoint](https://github.com/musatee/metric-endpoint)**