# drone-gemini-plugin

[English](README.md) | 中文

一个将 Google Gemini AI 集成到 Drone CI/CD 的插件，用于自动化代码分析、审查和文档生成。

## 主要功能

- **AI 代码审查** - 使用 Google Gemini 自动分析代码
- **Git Diff 分析** - 只分析变更文件，降低成本
- **成本追踪** - 实时显示 Token 消耗和费用估算
- **双认证支持** - 支持 Google AI Studio API Key 和 Vertex AI 服务账号
- **全球/区域端点** - 支持 global 和区域性 Vertex AI 端点

## 快速开始

### 方案 A: Gemini API Key（最简单）

从 [Google AI Studio](https://aistudio.google.com/apikey) 获取免费 API Key。

```bash
# 添加 Drone Secret
drone secret add --repository your-org/your-repo \
  --name gemini_api_key --data "AIzaSy..."
```

```yaml
kind: pipeline
type: docker
name: ai-review

steps:
  - name: code-review
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: "审查代码中的 bug 和安全问题"
      model: gemini-2.5-flash
      api_key:
        from_secret: gemini_api_key
```

### 方案 B: Vertex AI 服务账号（企业级）

适用于企业环境或在 AWS/Azure/自建基础设施上运行。

```bash
# 1. 创建具有 Vertex AI User 角色的 GCP 服务账号
gcloud iam service-accounts create gemini-sa
gcloud projects add-iam-policy-binding YOUR_PROJECT \
  --member="serviceAccount:gemini-sa@YOUR_PROJECT.iam.gserviceaccount.com" \
  --role="roles/aiplatform.user"

# 2. 下载密钥并添加到 Drone
gcloud iam service-accounts keys create sa-key.json \
  --iam-account=gemini-sa@YOUR_PROJECT.iam.gserviceaccount.com
drone secret add --repository your-org/your-repo \
  --name gcp_credentials --data @sa-key.json
```

```yaml
kind: pipeline
type: docker
name: ai-review

steps:
  - name: code-review
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: "对代码进行安全审查"
      model: gemini-3-pro-preview
      gcp_project: your-gcp-project-id
      gcp_location: global
      gcp_credentials:
        from_secret: gcp_credentials
```

## 配置参数

| 参数 | 环境变量 | 类型 | 默认值 | 说明 |
|-----|---------|------|-------|------|
| `prompt` | `PLUGIN_PROMPT` | string | **必填** | AI 指令/提示词 |
| `target` | `PLUGIN_TARGET` | string | `.` | 要分析的目录或文件 |
| `model` | `PLUGIN_MODEL` | string | `gemini-2.5-pro` | 使用的模型 |
| `api_key` | `PLUGIN_API_KEY` | string | | Gemini API Key (Google AI Studio) |
| `gcp_project` | `PLUGIN_GCP_PROJECT` | string | | GCP 项目 ID (Vertex AI) |
| `gcp_location` | `PLUGIN_GCP_LOCATION` | string | `us-central1` | GCP 区域 (gemini-3-* 用 `global`) |
| `gcp_credentials` | `PLUGIN_GCP_CREDENTIALS` | string | | 服务账号 JSON 内容 |
| `git_diff` | `PLUGIN_GIT_DIFF` | bool | `false` | 仅分析 git 变更 |
| `max_files` | `PLUGIN_MAX_FILES` | int | `50` | 最大包含文件数 |
| `timeout` | `PLUGIN_TIMEOUT` | int | `300` | 超时时间（秒） |
| `debug` | `PLUGIN_DEBUG` | bool | `false` | 启用调试输出 |

## 使用示例

### PR 代码审查

```yaml
steps:
  - name: ai-review
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: |
        审查此 PR：
        - 检查安全漏洞
        - 识别性能问题
        - 评估代码质量和可维护性
      git_diff: true
      model: gemini-2.5-flash
      api_key:
        from_secret: gemini_api_key
    when:
      event: pull_request
```

### 安全审计（Vertex AI）

```yaml
steps:
  - name: security-audit
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: |
        执行全面的安全审计：
        1. 检查 SQL/NoSQL 注入
        2. 审查认证和授权
        3. 识别敏感数据暴露
      model: gemini-3-pro-preview
      gcp_project: my-project
      gcp_location: global
      gcp_credentials:
        from_secret: gcp_credentials
```

## 支持的模型

| 模型 | 上下文 | 适用场景 | 定价 |
|-----|-------|---------|------|
| `gemini-2.5-pro` | 100万 tokens | 大型代码库、复杂分析 | $1.25-$2.50/百万输入 |
| `gemini-2.5-flash` | 100万 tokens | 快速、经济的审查 | $0.15-$0.30/百万输入 |
| `gemini-3-pro-preview` | 100万 tokens | 最新模型（global 区域） | $4/百万输入 |

## 成本追踪

插件在每次运行后显示 Token 使用量和估算成本：

```
+--------------------------------------------------------------+
|                      Token 消耗统计                            |
+--------------------------------------------------------------+
|  模型: Gemini 2.5 Pro                                         |
|  输入 Tokens: 4199  |  输出 Tokens: 72                         |
|  思考 Tokens: 1576                                             |
|  本次总成本: $0.021729                                         |
+--------------------------------------------------------------+
```

## 本地测试

```bash
# 编译插件
go build -o drone-gemini-plugin .

# 使用 API Key 测试
PLUGIN_PROMPT="描述这个项目" \
PLUGIN_API_KEY="your-api-key" \
PLUGIN_MODEL="gemini-2.5-flash" \
./drone-gemini-plugin

# 使用 Vertex AI 测试
PLUGIN_PROMPT="描述这个项目" \
PLUGIN_GCP_PROJECT="your-project-id" \
PLUGIN_GCP_LOCATION="global" \
PLUGIN_GCP_CREDENTIALS="$(cat service-account.json)" \
PLUGIN_MODEL="gemini-3-pro-preview" \
./drone-gemini-plugin
```

## 构建 Docker 镜像

```bash
# 构建镜像
docker build -t ghcr.io/jimmaabinyamin/drone-gemini-plugin .

# 或使用自定义仓库
docker build -t your-registry.com/drone-gemini:latest .
docker push your-registry.com/drone-gemini:latest
```

## 贡献

欢迎贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解指南。

## 许可证

Apache License 2.0 - 详见 [LICENSE](LICENSE)。
