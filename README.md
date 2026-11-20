<p align="center">
  <img src="frontend/public/logo.svg" alt="SubDeck Logo" width="240" />
</p>

# SubDeck - 轻量级安全 VPN 订阅聚合控制台

SubDeck 是一款专注**极轻量、高安全、免复杂脚本**的自托管 VPN 订阅与节点聚合管理系统。
支持将外部远程订阅链接与本地批量导入的私有节点订阅统一归纳管理，并通过图形化规则实现节点自动去重、正则过滤、批量重命名与协议筛选，最终导出为安全独立的专属客户端订阅链接。

---

## ✨ 核心特性

- 🚀 **极度轻量化**：Go 语言单静态二进制构建，静态内嵌 React 前端 SPA，启动即用，零额外依赖。
- 📦 **纯 Go 单文件数据库**：采用嵌入式纯 Go SQLite (`modernc.org/sqlite`，WAL 模式)，无 CGO 跨平台编译问题，所有数据存储于单文件 `data/subs.db`，备份迁移只需复制单个文件。
- 🗂️ **统一订阅管理模块**：
  - **单一入口**：远程订阅与本地订阅统一在“订阅管理”中维护，支持一键切换分类视图。
  - **本地订阅批量导入**：一次性批量粘贴多行单节点链接（支持 SS、VMess、VLESS、Trojan、Hysteria2、AnyTLS、TUIC、WireGuard、Snell 混合），直接打包生成一个独立的“本地订阅”，支持随时二次追加或修改内容。
  - **远程订阅定时同步**：支持自定义 User-Agent、可选自签证书跳过校验、后台定时自动拉取更新及本地持久化缓存（支持透传上游 `Subscription-Userinfo` 流量信息）。
- 🔀 **按订阅维度的组合与加工引擎**：
  - **订阅源组合**：组合以“订阅”为基本单元，自由勾选所需的本地订阅与远程订阅进行多源组合。
  - **自动去重**：根据 `IP:Port:Protocol` 指纹跨源自动消除重复节点。
  - **正则筛选**：支持包含正则（白名单，如 `香港|日本|新加坡`）与排除正则（黑名单，如 `官网|到期|剩余流量`）。
  - **批量重命名**：支持统一前缀（如 `[专线] `）、后缀及正则查找替换。
  - **协议过滤**：一键勾选仅保留指定协议类型。
- 🎯 **主流客户端无缝适配**：
  - **⚡ 极简万能自适应链接（Zero-Param URL）**：单 URL `http://your-server:8080/s/<TOKEN>`（或 `/api/sub/<TOKEN>`），无需拼接任何参数。服务端通过 `User-Agent` 自动识别下发 Clash 或 Sing-box 完整配置，浏览器访问则展示专属下载引导页。
  - **Mihomo (Clash.Meta) YAML**：支持作为 `proxy-providers` 纯节点列表，或生成带主流分流策略组的完整配置文件。
  - **Sing-box JSON**：支持纯 `outbounds` 节点数组，或开箱即用的完整 JSON 配置。
  - **通用 Base64 / URI**：兼容 Shadowrocket、V2RayN、Karing 等全平台通用客户端。
- 🛡️ **严格安全架构与防滥用保护**：
  - **管理后台鉴权**：管理员密码登录（`bcrypt`）+ 签名 JWT 保护；未显式指定密钥时首次启动自动生成随机密钥并持久化到数据库。
  - **登录防爆破保护**：IP 连续 5 次登录失败自动锁定 15 分钟，支持配置 `SUBDECK_TRUSTED_PROXIES` 防止反代场景下的 IP 伪造。
  - **订阅防刷频控（Rate Limiting）**：针对每个组合 Token 与客户端 IP 实现滑动窗口频控（每 30 秒最多 20 次），防止外部恶意高频刷取。
  - **独立安全 Token**：每个组合分配独立的随机 Token，客户端凭 Token 拉取配置；支持在 Web 界面随时一键吊销重置。
  - **上游隐私脱敏**：客户端仅能获取节点连接信息，上游机场订阅链接与原始凭据被完全隐藏在服务端。
- 💾 **一键数据热备份与恢复**：
  - 在“系统设置”中一键下载单文件 SQLite 完整热备份（带 WAL checkpoint 刷新）。
  - 支持在 Web 界面一键上传备份恢复，内置 SQLite 魔数、完整性校验及原子切换回滚保护。
- 🎨 **现代化响应式 Web UI**：
  - React 19 + Tailwind CSS v4 + HeroUI v3 构建，全面适配桌面与手机浏览器，支持深浅色模式无缝切换与扫码订阅二维码。

---

## 🚀 快速启动

### 方式一：Linux 一键安装脚本（推荐 VPS 部署）

支持 Ubuntu / Debian / CentOS / Alpine 等主流 Linux（`amd64` / `arm64`），自动下载对应 Release 二进制、校验 SHA256、引导配置端口与管理员账号密码，并注册 `systemd` 守护进程：

```bash
# 交互式一键安装
curl -fsSL https://raw.githubusercontent.com/dxer/SubDeck/main/install.sh | sudo bash

# 非交互式安装（指定端口与管理员账号密码）
curl -fsSL https://raw.githubusercontent.com/dxer/SubDeck/main/install.sh | sudo bash -s -- --port 8080 --user admin --pass YourStrongPass

# 国内网络可选 GitHub 镜像加速
curl -fsSL https://raw.githubusercontent.com/dxer/SubDeck/main/install.sh | sudo bash -s -- --proxy https://ghproxy.net/
```

安装完成后配置文件位于 `/etc/subdeck/config.yaml`，修改端口或管理员账号密码后执行 `systemctl restart subdeck` 即可生效。

---

### 方式二：直接运行单二进制与配置文件

从 [GitHub Releases](https://github.com/dxer/SubDeck/releases) 下载对应平台的压缩包解压：

```bash
# 生成默认配置文件 config.yaml（也可复制 config.example.yaml）
./subdeck -init-config

# 使用配置文件启动（默认自动加载同目录下的 config.yaml）
./subdeck -config config.yaml

# 或直接通过命令行参数指定（优先级高于环境变量与配置文件）
./subdeck -port 8888 -db /var/lib/subdeck/subs.db -admin-user admin -admin-pass YourStrongPass
```

配置优先级：**命令行参数 > 环境变量 (`SUBDECK_*`) > YAML 配置文件 (`config.yaml`) > 默认值**。
*(若未配置管理员密码，首次启动默认创建 `admin` / `admin123`)*

---

### 方式三：Docker Compose 一键部署

参考 `docker-compose.yml` 与 `.env.example`：

```yaml
services:
  subdeck:
    image: ghcr.io/dxer/subdeck:latest
    # 如需本地自行构建可取消下方注释
    # build: .
    container_name: subdeck
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - SUBDECK_JWT_SECRET=your-random-secret-key-here
```

运行服务：
```bash
docker compose up -d
```

---

## 🤖 GitHub Actions 自动构建与发布

仓库已内置开箱即用的 GitHub Actions 工作流：

1. **持续集成（`.github/workflows/ci.yml`）**：每次向 `main` / `master` 推送代码或发起 PR 时，自动编译前端、运行 `go vet` / `go test` 与验证 `Dockerfile`。
2. **自动打包发布（`.github/workflows/release.yml`）**：打版本 Tag 并推送后自动触发构建（也支持在 GitHub Actions 页面手动点击 `Run workflow` 触发）：
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```
   * 自动交叉编译 `linux/amd64`、`linux/arm64`、`windows/amd64`、`darwin/amd64`、`darwin/arm64` 单二进制压缩包与 `checksums.txt`，发布到 [GitHub Releases](https://github.com/dxer/SubDeck/releases)。
   * 自动构建多架构 Docker 镜像（`linux/amd64`、`linux/arm64`）并推送到 **GitHub Container Registry (`ghcr.io/dxer/subdeck`)**。

---

## 🛠️ 本地开发与构建

### 1. 构建完整单二进制
```bash
# 1. 构建前端静态资源
cd frontend && npm ci && npm run build && cd ..

# 2. 编译嵌入式单二进制
go build -trimpath -ldflags="-s -w" -o subdeck .
```

### 2. 前后端分离联调
```bash
# 终端 1：启动后端（无前端 embed 依赖）
go run -tags noembed .

# 终端 2：启动 Vite 前端开发服务器（已代理 /api 与 /s 到 :8080）
cd frontend && npm install && npm run dev
```

### 3. 运行单元测试
```bash
# 不依赖 frontend/dist 运行全部后端测试
go test -tags noembed ./...
```

---

## 📱 客户端接入方式

在 Web 控制台的“订阅组合”页面中，点击对应组合卡片即可复制以下格式的订阅 URL：

### 1. ⚡ 万能自适应链接（最推荐）
```
http://your-server:8080/s/<TOKEN>
```
- **Clash Verge / Mihomo Party / Flclash**：自动下发 Clash 全配置
- **Sing-box / SFA / SFM**：自动下发 Sing-box 全配置
- **Shadowrocket / V2RayN**：自动下发通用 Base64 节点列表
- **Chrome / Safari 浏览器直接打开**：展示一键导入与各格式下载指引页面

### 2. Mihomo / Clash Proxy Provider 模式
```yaml
proxy-providers:
  my_subdeck:
    type: http
    url: "http://your-server:8080/s/<TOKEN>?format=clash&type=provider"
    path: ./profiles/subdeck.yaml
    interval: 3600
    health-check:
      enable: true
      url: http://www.gstatic.com/generate_204
      interval: 300
```

### 3. 指定格式完整配置
- **Clash 全配置**：`http://your-server:8080/s/<TOKEN>?format=clash&type=full`
- **Sing-box 全配置**：`http://your-server:8080/s/<TOKEN>?format=singbox&type=full`
- **通用 Base64**：`http://your-server:8080/s/<TOKEN>?format=base64`

---

## 📂 项目结构概览

```
.
├── .github/workflows/         # GitHub Actions CI & 多平台自动发布工作流
├── frontend/                  # React 19 + Vite 6 + Tailwind v4 前端单页应用
│   ├── public/                # SubDeck 矢量图标 (favicon.svg / logo.svg)
│   ├── src/
│   │   ├── api/               # REST API 客户端封装与 JWT 处理
│   │   ├── components/        # 通用布局、品牌 Logo、弹窗与骨架屏组件
│   │   ├── pages/             # 页面（仪表盘、组合管理、订阅管理、系统设置、登录）
│   │   └── App.jsx
│   ├── vite.config.js         # Vite 构建与开发代理配置
│   └── package.json
├── internal/
│   ├── auth/                  # JWT 鉴权与 bcrypt 密码校验
│   ├── database/              # 纯 Go SQLite 持久化与热备份/恢复
│   ├── generator/             # 导出生成器 (Clash YAML / Singbox JSON / Base64)
│   ├── handlers/              # Gin RESTful API 与自适应订阅路由处理
│   ├── middleware/            # JWT 拦截鉴权与登录/拉取限流中间件
│   ├── models/                # 统一节点数据结构及业务模型
│   ├── parser/                # 各代理协议 URI 及订阅内容全自动解析器
│   ├── pipeline/              # 节点去重、正则过滤、批量重命名管道
│   └── services/              # 远程订阅拉取、节点聚合服务与后台定时调度器
├── main.go                    # 程序主入口与优雅停机
├── embed_dist.go              # 默认嵌入式前端资源
├── embed_nodist.go            # noembed 开发/测试构建支持
├── Dockerfile                 # 多阶段轻量化容器镜像构建文件
├── docker-compose.yml         # Compose 一键编排文件
└── README.md
```
