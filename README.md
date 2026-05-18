# Synora Gateway

Synora Gateway 是一款专为企业级 AI 应用设计的、高性能、高可用的 AI 模型中转与调度网关。它不仅实现了 OpenAI 协议的完美适配，更核心地提供了多通道智能容灾、实时计费监控以及异步审计日志功能，致力于为企业提供“稳定、合规、透明”的 AI 能力接入。

## 🚀 核心特性 (MVP 阶段)

- **多通道智能容灾 (Smart Failover)**：支持逻辑模型到物理通道的动态映射。当主通道出现 429、5xx 或网络超时时，网关将遵循“零字节切换”原则自动调度至备份通道。
- **高性能协议适配**：原生支持 OpenAI `/v1/chat/completions` 接口，支持流式 (SSE) 与非流式输出。
- **预付费计费系统**：
    - **秒级预检**：基于 Redis 缓存实现毫秒级的余额检查。
    - **安全扣费**：基于 PostgreSQL 事务与悲观锁，确保计费绝对精准，杜绝欠费。
- **异步审计日志 (ClickHouse)**：基于 ClickHouse 实现海量调用日志的异步持久化，支持 90 天数据留存 (TTL)，满足合规审计需求。
- **实时监控**：集成 Prometheus 指标端点，实时暴露 QPS、延迟及通道状态。
- **安全加固**：API Key 哈希存储，支持基于 Redis 的鉴权缓存。

## 🛠️ 技术栈

- **语言**：Go 1.26+
- **框架**：Gin (HTTP), pgx (PostgreSQL), go-redis (Redis)
- **存储**：PostgreSQL 16 (业务数据), Redis 7 (高速缓存), ClickHouse (日志分析)
- **部署**：Docker & Docker Compose

## 📦 快速开始

### 1. 克隆项目
```bash
git clone https://github.com/SynoraTech/synora-gateway.git
cd synora-gateway/synora-gateway
```

### 2. 环境配置
复制环境变量模板并根据需要调整：
```bash
cp .env.example .env
```

### 3. 启动基础环境
```bash
docker-compose -f deploy/docker-compose.yml up -d
```

### 4. 运行网关
```bash
go run cmd/server/main.go
```

## 📂 项目结构

```text
synora-gateway/
├── cmd/server/            # 程序入口
├── internal/
│   ├── adapter/           # 协议适配层 (OpenAI/Unified)
│   ├── api/               # API Handlers 与中间件 (鉴权/计费)
│   ├── audit/             # 审计日志 (ClickHouse)
│   ├── billing/           # 计费与钱包
│   ├── router/            # 通道管理与健康分算法
│   ├── upstream/          # 调度引擎与流式转发
│   └── storage/           # 数据库与缓存初始化
├── migrations/            # 数据库迁移脚本
└── deploy/                # Docker 与 CI/CD 配置
```

## 📜 授权协议

本项目采用 [GPLv3](./LICENSE) 协议开源。
