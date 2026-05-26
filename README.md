# Gemini2OpenAI

Gemini2OpenAI 是一个基于 Go 语言编写的 API 代理服务，旨在将 Google Gemini API 转换为 OpenAI 兼容的 API 格式。这使得现有的 OpenAI 生态工具和应用能够无缝接入 Gemini 模型。

## 功能特性

- **协议转换**: 将 OpenAI `v1/chat/completions` 请求转换为 Gemini `generateContent` 格式。
- **流式支持**: 完整支持 SSE (Server-Sent Events) 流式响应。
- **工具调用**: 支持 Gemini 的 Function Calling 和 Tool Use 映射。
- **轻量高效**: 基于标准库，无多余依赖，支持 Docker 快速部署。

## 快速开始

### 1. 前置条件

- 已安装 [Go](https://golang.org/) 1.18+
- 已获取 [Google Gemini API Key](https://aistudio.google.com/)

### 2. 编译与运行

```bash
# 克隆项目并进入目录
git clone https://github.com/mogumc/gemini2openai.git
cd gemini2openai

# 编译
go build -o gemini2openai main.go

# 启动服务
export GEMINI_API_KEY=your_gemini_api_key
./gemini2openai
```

默认服务将启动在 `8080` 端口。

## 配置说明

服务通过环境变量进行配置：

### 基础配置

| 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `8080` | 服务监听端口 |
| `GEMINI_API_KEY` | - | **必需**：您的 Gemini API Key |
| `GEMINI_BASE_URL` | `https://generativelanguage.googleapis.com/v1beta` | Gemini API 基础 URL |
| `GEMINI_MODEL` | `gemini-2.5-flash` | 默认使用的 Gemini 模型 |
| `PROXY_API_KEY` | - | 可选：代理服务访问的鉴权令牌 |

### 高级配置

| 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `DEFAULT_TEMPERATURE` | `1.0` | 默认生成温度 (0.0-2.0) |
| `DEFAULT_MAX_TOKENS` | `1048576` | 默认最大输出 token 数 |
| `DEFAULT_TOP_P` | `0.95` | 默认 Top-P 采样参数 |
| `DEFAULT_TOP_K` | `40` | 默认 Top-K 采样参数 |
| `DEFAULT_SAFETY_SETTINGS` | `BLOCK_NONE` | 默认安全设置：`BLOCK_NONE`/`BLOCK_LOW`/`BLOCK_MEDIUM`/`BLOCK_HIGH` |
| `DEFAULT_THINKING_BUDGET` | `0` | 默认 thinking 预算：`-1`=动态，`0`=禁用，`>0`=token 限制 |
| `CACHE_TTL` | `10m` | 缓存过期时间 (如 10m, 30m, 1h) |

### 模型白名单

| 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `ALLOWED_MODELS` | - | 可选：允许使用的模型列表，逗号分隔。留空表示允许所有模型 |

### 完整配置示例

```bash
# 基础配置
export GEMINI_API_KEY=your_api_key
export PORT=8080
export GEMINI_MODEL=gemini-2.5-flash

# 高级配置
export DEFAULT_TEMPERATURE=1.0
export DEFAULT_MAX_TOKENS=1048576
export DEFAULT_TOP_P=0.95
export DEFAULT_TOP_K=40
export DEFAULT_SAFETY_SETTINGS=BLOCK_NONE
export DEFAULT_THINKING_BUDGET=0

# 模型白名单（可选）
export ALLOWED_MODELS=gemini-2.5-flash,gemini-2.5-pro

# 代理鉴权（可选）
export PROXY_API_KEY=your_proxy_key
```

## 部署指南

### Docker 部署

```bash
# 构建镜像
docker build -t gemini2openai .

# 运行容器
docker run -d \
  --name gemini2openai \
  -p 8080:8080 \
  -e GEMINI_API_KEY=your_api_key \
  gemini2openai
```

### 容器编排 (Docker Compose)

在根目录下创建 `docker-compose.yml`:

```yaml
services:
  gemini2openai:
    build: .
    container_name: gemini2openai
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - GEMINI_API_KEY=${GEMINI_API_KEY}
    restart: unless-stopped
```

启动命令：
```bash
docker-compose up -d
```

## 贡献

欢迎提交 Issue 或 Pull Request。
