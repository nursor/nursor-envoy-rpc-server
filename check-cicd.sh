#!/bin/bash

echo "=== CI/CD 诊断脚本 ==="
echo ""

# 1. 检查工作流文件是否存在
echo "1. 检查工作流文件..."
if [ -f ".github/workflows/docker-build-push.yml" ]; then
    echo "   ✓ 工作流文件存在"
else
    echo "   ✗ 工作流文件不存在"
    exit 1
fi

# 2. 检查文件是否在 git 中
echo ""
echo "2. 检查文件是否已提交到 git..."
if git ls-files --error-unmatch .github/workflows/docker-build-push.yml > /dev/null 2>&1; then
    echo "   ✓ 文件已在 git 中"
else
    echo "   ✗ 文件未提交到 git"
    echo "   运行: git add .github/workflows/docker-build-push.yml && git commit -m 'Add workflow'"
fi

# 3. 检查当前分支
echo ""
echo "3. 检查当前分支..."
CURRENT_BRANCH=$(git branch --show-current)
echo "   当前分支: $CURRENT_BRANCH"
if [ "$CURRENT_BRANCH" = "master" ] || [ "$CURRENT_BRANCH" = "main" ]; then
    echo "   ✓ 在默认分支上"
else
    echo "   ⚠ 不在默认分支上，GitHub Actions 只在默认分支读取工作流文件"
fi

# 4. 检查最近的 tag
echo ""
echo "4. 检查最近的 tag..."
RECENT_TAGS=$(git tag --sort=-creatordate | head -3)
if [ -z "$RECENT_TAGS" ]; then
    echo "   ⚠ 没有找到 tag"
else
    echo "   最近的 tag:"
    echo "$RECENT_TAGS" | while read tag; do
        if [[ $tag == v* ]]; then
            echo "   ✓ $tag (符合触发条件)"
        else
            echo "   ⚠ $tag (不符合 v* 格式)"
        fi
    done
fi

# 5. 检查工作流文件语法
echo ""
echo "5. 检查工作流文件语法..."
if command -v yamllint &> /dev/null; then
    yamllint .github/workflows/docker-build-push.yml 2>&1 || echo "   ⚠ yamllint 发现一些问题（可能不影响运行）"
else
    echo "   ℹ 未安装 yamllint，跳过语法检查"
fi

# 6. 检查触发条件
echo ""
echo "6. 检查触发条件..."
TRIGGER=$(grep -A 2 "tags:" .github/workflows/docker-build-push.yml | grep -v "#" | grep -E "v\*|\*" | head -1 | tr -d ' -')
echo "   触发条件: $TRIGGER"
if [[ "$TRIGGER" == *"v*"* ]]; then
    echo "   ✓ 配置为触发 v* 格式的 tag"
elif [[ "$TRIGGER" == *"*"* ]]; then
    echo "   ✓ 配置为触发所有 tag"
fi

# 7. 检查远程仓库
echo ""
echo "7. 检查远程仓库..."
REMOTE_URL=$(git remote get-url origin 2>/dev/null)
if [ -n "$REMOTE_URL" ]; then
    echo "   远程仓库: $REMOTE_URL"
    if [[ "$REMOTE_URL" == *"github.com"* ]]; then
        echo "   ✓ 是 GitHub 仓库"
    else
        echo "   ⚠ 不是 GitHub 仓库，GitHub Actions 只在 GitHub 上运行"
    fi
else
    echo "   ⚠ 未配置远程仓库"
fi

# 8. 检查文件是否已推送
echo ""
echo "8. 检查文件是否已推送到远程..."
if git ls-remote --heads origin | grep -q "$CURRENT_BRANCH"; then
    LOCAL_HASH=$(git rev-parse HEAD)
    REMOTE_HASH=$(git ls-remote origin "$CURRENT_BRANCH" | cut -f1)
    if [ "$LOCAL_HASH" = "$REMOTE_HASH" ]; then
        echo "   ✓ 本地和远程同步"
    else
        echo "   ⚠ 本地和远程不同步，需要推送"
        echo "   运行: git push origin $CURRENT_BRANCH"
    fi
else
    echo "   ⚠ 分支未推送到远程"
    echo "   运行: git push -u origin $CURRENT_BRANCH"
fi

echo ""
echo "=== 诊断完成 ==="
echo ""
echo "如果所有检查都通过但 CI/CD 仍不运行，请检查："
echo "1. GitHub 仓库 Settings → Actions → General → 确保 Actions 已启用"
echo "2. GitHub Secrets 是否配置（DOCKER_USERNAME, DOCKER_PASSWORD）"
echo "3. 在 GitHub 仓库的 Actions 标签页查看是否有错误信息"
echo "4. 尝试创建一个新的 tag 来触发工作流："
echo "   git tag -a v1.0.2 -m 'Test CI/CD'"
echo "   git push origin v1.0.2"

