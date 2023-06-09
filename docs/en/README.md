# fake-time-injector

[中文](../../README.md) | English

## overview

fake-time-injector is a lightweight and flexible tool. using this tool, you can easily inject future fake time values into containers in order to simulate and test the behavior of your application in different time scenarios.

fake-time-injector is a component used to modify simulation time in cloud-native scenarios, open-sourced by Aliyun and Lilith Games together through the CloudNativeGame community.

![partners](../../images/partners.png)

## Plugin Supported Programming Languages

* Go
* C
* C++
* Erlang
* Ruby
* PHP
* JavaScript
* Python
* Java

## Example

Here's an example of how you can modify a container's process time using Fake-Time-Injector. This tool uses the webhook mechanism in Kubernetes to implement request parsing changes. Once you deploy this component in your container, you can modify the specific container time in your pod by writing a YAML file according to certain rules. The basic principle is to enable this component to modify the container time by configuring the FAKETIME plugin and LIBFAKETIME plugin.

### step1: generate CA certificate

Configure webhook admission in the cluster, use the following yaml to generate a secret containing the CA certificate, note that there is no need to configure the webhookconfig.yaml file, fake-time-injector will automatically configure MutatingWebhookConfiguration

To configure webhook admission in your cluster, use the following YAML to generate a secret containing the CA certificate. Note that there's no need to configure the webhookconfig.yaml file as Fake-Time-Injector will automatically configure MutatingWebhookConfiguration.

* First, you'll need to install cfssl, which you'll use to create the certificate:

```shell
linux:
wget -q https://pkg.cfssl.org/R1.2/cfssl_linux-amd64 https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64
chmod +x cfssl_linux-amd64 cfssljson_linux-amd64 
sudo mv cfssl_linux-amd64 /usr/local/bin/cfssl
sudo mv cfssljson_linux-amd64 /usr/local/bin/cfssljson

mac:
brew install cfssl
```

* Create a CA certificate using the following JSON file:

```shell
cat > ca-config.json <<EOF
{
    "signing": {
        "default": {
            "expiry": "26280h"         //  this specifies the expiration time for the certificate
        },
        "profiles": {
            "server": {
                "usages": [
                    "signing",
                    "key encipherment",
                    "server auth",
                    "client auth"
                ],               //   this specifies the key usages for this particular profile
                "expiry": "26280h"
            }
        }
    }
}
EOF

cat > ca-csr.json <<EOF 
{
    "CN": "Kubernetes",
    "key": {
        "algo": "rsa",
        "size": 2048
    },             //  the key-related parameters, including the algorithm and size
    "names": [
        {
            "C": "US",
            "L": "Portland",
            "O": "Kubernetes",
            "OU": "CA",
            "ST": "Oregon"
        }
    ]
} 
EOF

cfssl gencert -initca ca-csr.json | cfssljson -bare ca 
```

* Creating a Server Cert

```shell
cat > server-csr.json <<EOF 
{
    "CN": "admission",
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "US",
            "L": "Portland",
            "O": "Kubernetes",
            "OU": "Kubernetes",
            "ST": "Oregon"
        }
    ]
} 
EOF

cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -hostname=kubernetes-faketime-injector.kube-system.svc -profile=server server-csr.json | cfssljson -bare server
```

-hostname: the naming scheme is `{serviceName}. {serviceNamespace}.svc`, in this example webhook's serviceName is kubernetes-faketime-injector and namespace is kube-system.

* Base64-Encrypt the Generated Certificate:

```shell
cat ca.pem | base64
cat server.pem | base64
cat server-key.pem | base64
```

* Use the Key from the Previous Step to Generate the Secret:

```shell
cat > secret.yaml <<EOF
apiVersion: v1
data:
  ca-cert.pem: xxxxxxxxx
  server-cert.pem: xxxxxx
  server-key.pem: xxxxxxx
kind: Secret
metadata:
  name: kubernetes-faketime-injector
  namespace: kube-system
EOF

  kubectl apply -f secret.yaml
```

### step2: deploy fake-time-injector

Deploy fake-time-injector using the following YAML file:


```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: fake-time-injector-sa
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: fake-time-injector-cr
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "patch", "update", "watch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]
  - apiGroups: ["admissionregistration.k8s.io"]
    resources: ["mutatingwebhookconfigurations"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: fake-time-injector-rb
subjects:
  - kind: ServiceAccount
    name: fake-time-injector-sa
    namespace: kube-system
roleRef:
  kind: ClusterRole
  name: fake-time-injector-cr
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-faketime-injector
  namespace: kube-system
  labels:
    app: kubernetes-faketime-injector
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kubernetes-faketime-injector
  template:
    metadata:
      labels:
        app: kubernetes-faketime-injector
    spec:
      containers:
        - image: registry.cn-hangzhou.aliyuncs.com/acs/fake-time-injector:v1     # docker build -t fake-time-injector:v1 . -f fake-time-injector/Dockerfile
          imagePullPolicy: Always
          name: kubernetes-faketime-injector
          resources:
            limits:
              cpu: 100m
              memory: 100Mi
            requests:
              cpu: 100m
              memory: 100Mi
          env:
            - name: LIBFAKETIME_PLUGIN_IMAGE
              value: "registry.cn-hangzhou.aliyuncs.com/acs/libfaketime:v1"
            - name: FAKETIME_PLUGIN_IMAGE
              value: "registry.cn-hangzhou.aliyuncs.com/acs/fake-time-sidecar:v1"   # docker build -t fake-time-sidecar:v1 . -f fake-time-injector/plugins/faketime/build/Dockerfile
          volumeMounts:
            - name: webhook-certs
              mountPath: /run/secrets/tls
      serviceAccountName: fake-time-injector-sa
      volumes:
        - name: webhook-certs
          secret:
            secretName: kubernetes-faketime-injector
---
kind: Service
apiVersion: v1
metadata:
  name: kubernetes-faketime-injector
  namespace: kube-system
spec:
  ports:
    - port: 443
      targetPort: 443
      name: webhook
  selector:
    app: kubernetes-faketime-injector
```

Save this YAML file to a local file named deploy.yaml. Then, use the following command to deploy it:

```
kubectl apply -f deploy.yaml 
```

### step3: modify time

To use the injector, you need to add two annotations to the pod:
* cloudnativegame.io/process-name: sets the process that needs to modify the time
* cloudnativegame.io/fake-time: sets the fake time

Here's an example YAML file that illustrates how to add these annotations to a Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: testpod
  namespace: kube-system
  labels:
    app: myapp
    version: v1
  annotations:
    cloudnativegame.io/process-name: "hello"      # If you need to modify multiple processes at the same time, just separate the process names with `,`
    cloudnativegame.io/fake-time: "2024-01-01 00:00:00"
spec:
  containers:
    - name: myhello
      image: registry.cn-hangzhou.aliyuncs.com/acs/hello:v1
```
Save this YAML file to a local file named testpod.yaml. Then, use the following command to deploy it:

```yaml
kubectl apply -f testpod.yaml
```

To enter the myhello container and test that the time is modified, use the following command:

```
kubectl exec -it testpod -c myhello /bin/bash -n kube-system
```
![example1](../../images/watchmakerexample.png)

We also provide another method to modify the container's time, you can also have the command executed in virtual time

![example2](../../images/libfaketimeexample.png)

## Alternative Solution

We also recommend another approach for modifying time, which involves adding a sidecar container directly to the Pod. here's how you can do it:

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    name: hello
  name: hello
spec:
  containers:
    - image: 'registry.cn-hangzhou.aliyuncs.com/acs/hello:v1'
      imagePullPolicy: IfNotPresent
      name: myhello
    - env:
        - name: modify_process_name
          value: hello           # If you need to modify multiple processes at the same time, just separate the process names with `,`
        - name: delay_second
          value: '86400'
      image: 'registry.cn-hangzhou.aliyuncs.com/acs/fake-time-sidecar:v1'
      imagePullPolicy: Always
      name: fake-time-sidecar
  shareProcessNamespace: true
```

In this approach, you need to set two environment variables for the sidecar container: modify_process_name and delay_second. this will allow you to specify which process needs to modify the time, and the future time difference from this moment.

Also, note that we've added shareProcessNamespace to the spec to ensure that both containers share the same process namespace.

## Dependencies

This project uses the following open-source software:

* [Chaos-mesh](https://github.com/chaos-mesh/chaos-mesh) - Reference chaos-mesh's watchmaker component to simulate process time
* [Libfaketime](https://github.com/wolfcw/libfaketime) - Reference the libfaketime dynamic link library to simulate time