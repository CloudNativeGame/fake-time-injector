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

### 步骤1: 部署fake-time-injector

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
        - image: registry.cn-hangzhou.aliyuncs.com/acs/fake-time-injector:v3     #  使用 fake-time-injector/Dockerfile 创建镜像
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
            - name: CLUSTER_MODE     # CLUSTER_MODE为true时，命名空间内的所有pod在一定时间范围内(40s)启动时获得一致的偏移量
              value: "true"
            - name: LIBFAKETIME_PLUGIN_IMAGE
              value: "registry.cn-hangzhou.aliyuncs.com/acs/libfaketime:v1"
            - name: FAKETIME_PLUGIN_IMAGE
              value: "registry.cn-hangzhou.aliyuncs.com/acs/fake-time-sidecar:v2"   # 使用 fake-time-injector/plugins/faketime/build/Dockerfile 创建镜像
      serviceAccountName:  fake-time-injector-sa
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

### step2: 修改时间

我们提供两种修改进程时间的方法，watchmaker指令和libfaketime链接库。

libfaketime链接库配置方法，添加annotation：
支持语言：python、c、ruby、php、c++、js、java、erlang
* cloudnativegame.io/fake-time: 设置虚假的时间

yaml配置示例:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test
  namespace: kube-system
  labels:
    app: myapp
    version: v1
  annotations:
    cloudnativegame.io/fake-time: "2024-01-01 00:00:00"  # 此处还可以配置时分秒组合的时间间隔，如'3h40s'和'-7h20m40s'， '-'表示过去的时间。
spec:
  containers:
    - name: test
      image: registry.cn-hangzhou.aliyuncs.com/acs/testc:v1
```

watchmaker配置方法,增加如下annotation。
支持语言：go、python、ruby、php、c++
* cloudnativegame.io/process-name: 设置需要修改时间的进程
* cloudnativegame.io/fake-time: 设置虚假的时间

yaml配置示例:

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
    cloudnativegame.io/fake-time: "2024-01-01 00:00:00"     # 此处还可以配置调整的秒数，'86400'表示时间向后漂移一天，watchmaker不支持过去的时间。
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

## 依赖项

此项目使用以下开源软件：

* [Chaos-mesh](https://github.com/chaos-mesh/chaos-mesh) - 引用 chaos-mesh 的 watchmaker 组件来模拟进程时间
* [Libfaketime](https://github.com/wolfcw/libfaketime) - 引用 libfaketime 动态链接库来模拟时间