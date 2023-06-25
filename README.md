# fake-time-injector

中文 | [English](./docs/en/README.md)

## 概述

fake-time-injector 是一个轻量级且灵活的工具。使用该工具，您可以轻松地将未来的虚假时间值注入到容器中，以便在不同的时间场景下模拟和测试应用程序的行为。

fake-time-injector是阿里云与莉莉丝游戏通过CloudNativeGame社区一起开源的用于云原生场景下修改模拟时间的组件。

![partners](images/partners.png)

## 插件支持编程语言

* Go
* C
* Erlang
* C++
* Ruby
* PHP
* JavaScript
* Python
* Java

## 示例

以下是使用 fake-time-injector 修改容器进程时间的示例。该工具使用 Kubernetes 中的 Webhook 机制实现请求解析更改。一旦在容器中部署了此组件，您就可以按照某些规则编写 YAML 文件来修改 pod 中特定容器的时间。基本原理是通过配置 WATCHMAKER 插件和 LIBFAKETIME 插件使此组件能够修改容器时间。

### 步骤 1：生成CA证书

要在群集中配置 webhook admission，请使用以下 YAML 生成包含 CA 证书的 secret。注意，无需配置 webhookconfig.yaml 文件，因为 Fake-Time-Injector 将自动配置 MutatingWebhookConfiguration。

* 首先，您需要安装 cfssl 以创建证书：

```shell
linux:
wget -q https://pkg.cfssl.org/R1.2/cfssl_linux-amd64 https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64
chmod +x cfssl_linux-amd64 cfssljson_linux-amd64 
sudo mv cfssl_linux-amd64 /usr/local/bin/cfssl
sudo mv cfssljson_linux-amd64 /usr/local/bin/cfssljson

mac:
brew install cfssl
```

* 使用以下 JSON 文件创建 CA 证书：

```shell
cat > ca-config.json <<EOF
{
    "signing": {
        "default": {
            "expiry": "26280h"
        },         //证书的有效期
        "profiles": {
            "server": {
                "usages": [
                    "signing",
                    "key encipherment",
                    "server auth",
                    "client auth"
                ],              //证书使用的场景
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

* 创建服务器证书

```shell
cat > server-csr.json <<EOF 
{
    "CN": "admission",
    "key": {
        "algo": "rsa",
        "size": 2048
    },        //  生成证书所需的算法和密钥长度
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

-hostname：命名方式为`{serviceName}.{serviceNamespace}.svc`，本示例中webhook的serviceName是kubernetes-faketime-injector，namespace是kube-system。

* 对生成的证书进行 Base64 加密：

```shell
cat ca.pem | base64
cat server.pem | base64
cat server-key.pem | base64
```

* 使用前面步骤中的密钥生成 secret：

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

### 步骤 2: 部署fake-time-injector

使用以下YAML文件，部署fake-time-injector：

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
        - image: registry.cn-hangzhou.aliyuncs.com/acs/fake-time-injector:v1     #  使用 fake-time-injector/Dockerfile 创建镜像
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
              value: "registry.cn-hangzhou.aliyuncs.com/acs/fake-time-sidecar:v1"   # 使用 fake-time-injector/plugins/faketime/build/Dockerfile 创建镜像
          volumeMounts:
            - name: webhook-certs
              mountPath: /run/secrets/tls
      serviceAccountName:  fake-time-injector-sa
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

将这个YAML文件保存到一个名为deploy.yaml的文件中。然后使用下面的命令来部署它：

```
kubectl apply -f deploy.yaml 
```

### step3: 修改时间

要使用fake-time-injector，你需要向pod添加两个注解：
* cloudnativegame.io/process-name: 设置需要修改时间的进程
* cloudnativegame.io/fake-time: 设置虚假的时间

下面是一个YAML文件的例子，说明了如何给pod添加annotation：

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
    cloudnativegame.io/process-name: "hello"     # 如果需要同时修改多个进程用`,`隔开进程名即可
    cloudnativegame.io/fake-time: "2024-01-01 00:00:00"
spec:
  containers:
    - name: myhello
      image: registry.cn-hangzhou.aliyuncs.com/acs/hello:v1
```
将这个YAML文件保存到一个名为testpod.yaml的本地文件。然后，使用下面的命令来部署它：

```yaml
kubectl apply -f testpod.yaml
```

要进入myhello容器并测试时间是否被修改，使用以下命令：

```
kubectl exec -it testpod -c myhello /bin/bash -n kube-system
```
![example1](images/watchmakerexample.png)

我们还提供了另一种方法修改容器的时间,你也可以让命令以虚拟时间内执行:

![example2](images/libfaketimeexample.png)

## 替代方案

我们还推荐另一种修改时间的方法，即直接在Pod上添加一个sidecar容器。下面是你的操作方法：

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
          value: hello               # 如果需要同时修改多个进程用`,`隔开进程名即可
        - name: delay_second
          value: '86400'
      image: 'registry.cn-hangzhou.aliyuncs.com/acs/fake-time-sidecar:v1'
      imagePullPolicy: Always
      name: fake-time-sidecar
  shareProcessNamespace: true
```

在这种方法中，你需要为sidecar容器设置两个环境变量：modify_process_name 和 delay_second。这将允许你指定哪个进程需要修改时间，以及未来相距此刻的时间差。

另外请注意，我们在`spec`中加入了 shareProcessNamespace，以确保两个容器共享相同的进程命名空间。