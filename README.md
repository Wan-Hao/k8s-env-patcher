# k8s-env-injector

一个 Kubernetes Mutating Webhook，用于根据 Pod 标签自动注入环境变量和其他配置。

## 功能特性

- 基于 Pod 标签选择性注入环境变量
- 支持配置 DNS 选项
- 支持配置节点亲和性
- 支持配置容忍度
- 支持配置拓扑分布约束

## 快速开始

### 使用 Make 命令

```bash
# 部署（创建新集群或使用现有集群）
make deploy

# 运行测试
make test

# 部署并测试
make test-all

# 清理资源
make clean
```

### 手动部署

#### 1. 拉取代码

```shell
git clone https://github.com/your-username/k8s-env-injector.git
cd k8s-env-injector
```

#### 2. 检查集群环境

```shell
kubectl api-versions | grep admissionregistration.k8s.io/v1
# 输出应该包含
admissionregistration.k8s.io/v1
```

#### 3. 创建集群和命名空间

```shell
kind create cluster --name env-injector
kubectl create namespace injector
```

#### 4. 部署 webhook

##### 构建镜像

```shell
cd image && docker build -t k8s-env-injector:dev .
# 检查镜像
docker images | grep k8s-env-injector
```

##### 加载镜像到集群

```shell
kind load docker-image k8s-env-injector:dev --name env-injector
```

##### 生成证书

```shell
cd ../deployment
./webhook-create-signed-cert.sh \
    --service env-injector-webhook-svc \
    --secret env-injector-webhook-certs \
    --namespace injector

cat mutatingwebhook.yaml | ./webhook-patch-ca-bundle.sh > mutatingwebhook-ca-bundle.yaml
```

##### 部署资源

```shell
kubectl create -f configmap.yaml -n injector
kubectl create -f deployment.yaml -n injector
kubectl create -f service.yaml -n injector
kubectl create -f mutatingwebhook-ca-bundle.yaml
```

#### 5. 检查部署状态

```shell
kubectl get pods -n injector
# 输出应该类似
NAME                                               READY   STATUS    RESTARTS   AGE
env-injector-webhook-deployment-xxxxxx-xxxxx       1/1     Running   0          6s
```

## 配置说明

### Pod 标签要求

需要同时满足以下标签才会注入环境变量：
- `inject-env: "true"`
- `app-type` 为 "web" 或 "api"

### 命名空间标签要求

命名空间需要添加标签：
- `wh/envInjector: enabled`

### 环境变量配置

通过 ConfigMap 配置要注入的环境变量：
```yaml
env:
  - name: INJECTOR_TEST
    value: enabled
```

### 节点亲和性配置等

同样操作，这里省略

## 测试

### 运行集成测试

```bash
# 使用默认命名空间
./bin/test.sh

# 指定命名空间
./bin/test.sh -n custom-namespace
```

### 测试场景

1. 正确标签的 Pod：应该注入环境变量
2. 无标签的 Pod：不应该注入环境变量
3. 错误类型的 Pod：不应该注入环境变量

## 开发

### 目录结构
