# fake-time-injector
## override
Fake-time-injector is a lightweight and flexible tool. With this tool, you can easily inject fake time values into your containers, allowing you to simulate different time scenarios and test the behavior of your applications under various time conditions

### Example
Here is an example that modifies the container's process time. Fake-time-injector uses the webhook mechanism in Kubernetes to implement request parsing changes. After you deploy this component in the container, you can modify the specific container time in the pod by writing a yaml file through certain rules. The basic principle is to enable this component to have the ability to modify the container time by configuring the FAKETIME plugin and LIBFAKETIME plugin.
#### step1: Configure the secret containing the ca certificate
Configure webhook admission in the cluster, use the following yaml to generate a secret containing the CA certificate, note that there is no need to configure the webhookconfig.yaml file, fake-time-injector will automatically configure MutatingWebhookConfiguration

* First you need to install the 'cfssl' that you need to use to create the certificate
```shell
wget -q https://pkg.cfssl.org/R1.2/cfssl_linux-amd64 https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64
chmod +x cfssl_linux-amd64 cfssljson_linux-amd64 
sudo mv cfssl_linux-amd64 /usr/local/bin/cfssl
sudo mv cfssljson_linux-amd64 /usr/local/bin/cfssljson
```

* Create a CA certificate using the following json fil

```shell
cat > ca-config.json <<EOF
> {
>     "signing": {
>         "default": {
>             "expiry": "8760h"
>         },
>         "profiles": {
>             "server": {
>                 "usages": [
>                     "signing",
>                     "key encipherment",
>                     "server auth",
>                     "client auth"
>                 ],
>                 "expiry": "8760h"
>             }
>         }
>     }
> }
> EOF

cat > ca-csr.json <<EOF 
{
    "CN": "Kubernetes",
    "key": {
        "algo": "rsa",
        "size": 2048
    },
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

* Base64 encryption of the generated certificate

```shell
cat ca.pem | base64
cat server.pem | base64
cat server-key.pem | base64
```

* Use the key from the previous step to generate the secret

```yaml
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

#### step2: deploy the webhook and service
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-faketime-injector
  namespace: kube-system
  labels:
    app: kubernetes-faketime-injector
spec:
  replicas: 1 # The default is primary and standby mode (currently cold standby)
  selector:
    matchLabels:
      app: kubernetes-faketime-injector
  template:
    metadata:
      labels:
        app: kubernetes-faketime-injector
    spec:
      containers:
        - image: registry.cn-hangzhou.aliyuncs.com/acs/faketime:v2
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
            - name: FAKETIME_PLUGIN_IMAGE
              value: "registry.cn-hangzhou.aliyuncs.com/acs/watchmaker:v11"
          volumeMounts:
            - name: webhook-certs
              mountPath: /run/secrets/tls
      # todo change service account to kubernetes-webhook-injector as default
      serviceAccountName: admin
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
Deploy the yaml file.
```
kubectl apply -f deploy/kubernetes-faketime-injector.yaml 
```
#### step3: deploy the pod
You need to add two annotations to the pod.
* One of the annotations is 'game.cloudnative.io/modify-process-name', which sets the process that needs to modify the time
* Another annotation is 'game.cloudnative.io/delay-second', which sets how long the previous process needs to drift

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
    game.cloudnative.io/modify-process-name: "hello"
    game.cloudnative.io/delay-second: "86400"
spec:
  containers:
    - name: myhello
      image: registry.cn-hangzhou.aliyuncs.com/acs/hello:v1
```

#### step4: check the result  
Use the following command to enter the 'myhello' container，The hello process will record the time to the demo.txt file every 5 seconds

`
kubectl exec -it testpod -c myhello /bin/bash -n kube-system
`
![example2](example/example1.png)
As can be seen from the results, the time is delayed by 86400 seconds

## Alternative Solution

Here is another method recommended for you, directly add sidecar to the pod that needs to modify the time, then you need to set two environment variables are 'modify_process_name' and 'delay_second'

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
          value: hello
        - name: delay_second
          value: '86400'
      image: 'registry.cn-hangzhou.aliyuncs.com/acs/watchmaker:v11'
      imagePullPolicy: Always
      name: fake-time-sidecar
  shareProcessNamespace: true
```