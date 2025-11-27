# CI/CD 故障排查指南

## 常见问题及解决方案

### 1. 工作流文件未提交到仓库

**问题**: `.github/workflows/` 目录下的文件必须提交到仓库才能生效。

**解决**:
```bash
git add .github/workflows/docker-build-push.yml
git commit -m "Add CI/CD workflow"
git push origin master  # 或你的主分支名称
```

### 2. 工作流文件不在默认分支

**问题**: GitHub Actions 只在默认分支（通常是 `main` 或 `master`）上读取工作流文件。

**解决**: 确保工作流文件在默认分支上：
```bash
# 检查当前分支
git branch

# 切换到默认分支（如果需要）
git checkout master  # 或 main

# 确保工作流文件存在
ls -la .github/workflows/
```

### 3. GitHub Secrets 未配置

**问题**: Docker 登录需要配置 Secrets。

**解决**: 
1. 进入 GitHub 仓库
2. Settings → Secrets and variables → Actions
3. 添加以下 Secrets:
   - `DOCKER_USERNAME`: 你的 Docker Hub 用户名
   - `DOCKER_PASSWORD`: 你的 Docker Hub 密码或访问令牌

### 4. Tag 格式不匹配

**问题**: 工作流只触发以 `v` 开头的 tag（如 `v1.0.0`）。

**检查**:
```bash
# 查看所有 tag
git tag

# 如果 tag 不是以 v 开头，需要修改工作流文件或重新创建 tag
```

**解决**: 
- 选项 1: 创建符合格式的 tag
  ```bash
  git tag -a v1.0.2 -m "Release v1.0.2"
  git push origin v1.0.2
  ```

- 选项 2: 修改工作流文件支持所有 tag
  编辑 `.github/workflows/docker-build-push.yml`，将触发条件改为：
  ```yaml
  on:
    push:
      tags:
        - '*'  # 支持所有 tag
  ```

### 5. 工作流文件语法错误

**检查**: 在 GitHub 仓库的 Actions 标签页查看是否有错误提示。

**解决**: 确保 YAML 语法正确，特别注意缩进（使用空格，不是 Tab）。

### 6. 权限问题

**问题**: GitHub Actions 可能没有推送权限。

**解决**: 
- 检查仓库的 Actions 设置是否启用
- Settings → Actions → General → 确保 "Allow all actions and reusable workflows" 已启用

### 7. 工作流文件路径错误

**问题**: 工作流文件必须在 `.github/workflows/` 目录下。

**检查**:
```bash
ls -la .github/workflows/
```

应该看到 `docker-build-push.yml` 文件。

## 验证步骤

1. **检查文件是否存在**:
   ```bash
   ls -la .github/workflows/docker-build-push.yml
   ```

2. **检查文件是否已提交**:
   ```bash
   git ls-files .github/workflows/
   ```

3. **检查是否在正确的分支**:
   ```bash
   git branch --show-current
   ```

4. **测试触发**:
   ```bash
   # 创建测试 tag
   git tag -a v1.0.2 -m "Test CI/CD"
   git push origin v1.0.2
   ```

5. **查看 GitHub Actions**:
   - 进入 GitHub 仓库
   - 点击 "Actions" 标签页
   - 查看是否有新的工作流运行

## 快速修复命令

```bash
# 1. 确保工作流文件存在
ls -la .github/workflows/docker-build-push.yml

# 2. 添加到 git
git add .github/workflows/docker-build-push.yml

# 3. 提交
git commit -m "Add CI/CD workflow for Docker build and push"

# 4. 推送到远程
git push origin master  # 或你的主分支

# 5. 创建测试 tag
git tag -a v1.0.2 -m "Test CI/CD workflow"
git push origin v1.0.2

# 6. 检查 GitHub Actions
# 在浏览器中打开: https://github.com/your-username/your-repo/actions
```

