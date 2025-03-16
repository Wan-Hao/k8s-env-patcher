# Local Development & Deployment & Testing

## Pull code from github to your local working directory

```shell
git clone https://github.com/hmcts/k8s-env-injector.git
cd k8s-env-injector
```

## check your cluster environment

```shell
kubectl api-versions | grep admissionregistration.k8s.io/v1
# output should be like this
admissionregistration.k8s.io/v1
```

## Create a new cluster and a new namespace inside it

```shell
kind create cluster --name env-injector
kubectl create namespace admin
# namespace name can be any name as long as you fit the namespace name in the deployment yaml file
```

## Deploy the webhook to the cluster

### Create a certificate for the webhook

```shell
cd deployment && ./webhook-create-signed-cert.sh --service env-injector-webhook-svc --secret env-injector-webhook-certs --namespace admin
```

### Update MutatingWebhookConfiguration with the new certificate

```shell
cat mutatingwebhook.yaml | ./webhook-patch-ca-bundle.sh > mutatingwebhook-ca-bundle.yaml
```

### Deploy the webhook to the cluster

```shell
cd image && docker build -t k8s-env-injector:dev .
# If image is already built, you can skip this step

# check the image
docker images | grep k8s-env-injector
```

### Load the image to the cluster

```shell
kind load docker-image k8s-env-injector:dev --name env-injector
```

### Build ca-bundle

```shell
./webhook-create-signed-cert.sh --service env-injector-webhook-svc --secret env-injector-webhook-certs --namespace admin && cat mutatingwebhook.yaml | ./webhook-patch-ca-bundle.sh > mutatingwebhook-ca-bundle.yaml
```

### Deploy all resources

```shell
cd deployment && kubectl create -f configmap.yaml -n admin && kubectl create -f deployment.yaml -n admin && kubectl create -f service.yaml -n admin && kubectl create -f mutatingwebhook-ca-bundle.yaml
```

## Check deployment status

```shell
kubectl get pods -n admin
# it should be like this
NAME                                               READY   STATUS    RESTARTS   AGE
env-injector-webhook-deployment-6877c557cb-qmj4j   1/1     Running   0          6s
```

## Develop the webhook

After you make changes to the webhook, you need to rebuild the image and load the image to the cluster.

```shell
# rebuild the image
cd image && docker build -t k8s-env-injector:dev .
```

```shell
# reload the image to the cluster
kind load docker-image k8s-env-injector:dev --name env-injector
```

```shell
# recreate the webhook
./webhook-create-signed-cert.sh --service env-injector-webhook-svc --secret env-injector-webhook-certs --namespace admin && cat mutatingwebhook.yaml | ./webhook-patch-ca-bundle.sh > mutatingwebhook-ca-bundle.yaml
```

```shell
# apply the changes
cd ../deployment && kubectl delete -f deployment.yaml -n admin && kubectl delete -f service.yaml -n admin && kubectl delete -f configmap.yaml -n admin && kubectl delete -f mutatingwebhook-ca-bundle.yaml
```




 