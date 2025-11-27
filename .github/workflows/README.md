# GitHub Actions CI/CD 配置说明

## 概述

当您推送 tag 到 GitHub 仓库时，GitHub Actions 会自动：
1. 根据 Dockerfile 构建 Docker 镜像
2. 使用 tag 名称作为镜像标签
3. 同时打上 `latest` 标签
4. 推送到指定的 Docker 仓库

## 配置步骤

### 1. 设置 GitHub Secrets

在 GitHub 仓库中设置以下 Secrets（Settings → Secrets and variables → Actions）：

- `DOCKER_USERNAME`: Docker 仓库的用户名
- `DOCKER_PASSWORD`: Docker 仓库的密码或访问令牌

### 2. 修改工作流配置（可选）

编辑 `.github/workflows/docker-build-push.yml` 文件，根据需要修改以下变量：

```yaml
env:
  DOCKER_REGISTRY: docker.io  # Docker Hub 或其他 registry
  IMAGE_NAME: nursor-envoy-rpc  # 镜像名称
```

### 3. 支持的 Docker Registry

#### Docker Hub
```yaml
DOCKER_REGISTRY: docker.io
DOCKER_USERNAME: your-dockerhub-username
```

#### 阿里云容器镜像服务
```yaml
DOCKER_REGISTRY: registry.cn-hangzhou.aliyuncs.com
DOCKER_USERNAME: your-aliyun-username
```

#### 腾讯云容器镜像服务
```yaml
DOCKER_REGISTRY: ccr.ccs.tencentyun.com
DOCKER_USERNAME: your-tencent-username
```

#### 私有 Registry
```yaml
DOCKER_REGISTRY: your-registry.com
DOCKER_USERNAME: your-username
```

## 使用方法

### 创建并推送 Tag

```bash
# 创建 tag
git tag -a v1.0.0 -m "Release version 1.0.0"

# 推送 tag 到远程仓库
git push origin v1.0.0
```

### 触发条件

工作流会在以下情况触发：
- 推送以 `v` 开头的 tag（例如：v1.0.0, v1.2.3）

如果需要支持所有 tag，可以修改工作流文件中的触发条件：

```yaml
on:
  push:
    tags:
      - '*'  # 支持所有 tag
```

## 构建的镜像标签

- `{registry}/{username}/{image-name}:{tag}` - 使用 tag 名称
- `{registry}/{username}/{image-name}:latest` - latest 标签

例如，如果推送 `v1.0.0` tag，会生成：
- `docker.io/username/nursor-envoy-rpc:v1.0.0`
- `docker.io/username/nursor-envoy-rpc:latest`

## 查看构建状态

在 GitHub 仓库的 Actions 标签页可以查看构建状态和日志。

## 故障排除

### 构建失败

1. 检查 Dockerfile 是否正确
2. 检查 GitHub Secrets 是否配置正确
3. 查看 Actions 日志获取详细错误信息

### 推送失败

1. 确认 Docker 用户名和密码/令牌正确
2. 确认有推送权限
3. 检查网络连接

