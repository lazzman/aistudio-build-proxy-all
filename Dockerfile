# --- Stage 1: Go Builder ---
# 使用官方的 Go 镜像作为构建环境
FROM golang:1.22-alpine AS builder-go

# 设置工作目录
WORKDIR /src

# 复制 Go 项目的模块文件并下载依赖
COPY golang/go.mod ./
RUN go mod download

# 复制 Go 项目的源代码
COPY golang/ .

# 编译 Go 应用。CGO_ENABLED=0 创建一个静态链接的二进制文件，更适合容器环境
# -o 指定输出文件名和路径
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go_app_binary .

# --- Stage 2: Node Builder (for React log viewer) ---
FROM node:20-alpine AS builder-node

WORKDIR /app

# 复制 package.json 并安装依赖
COPY log-viewer/package*.json ./
RUN npm install

# 复制源代码并构建
COPY log-viewer/ .
RUN npm run build


# --- Stage 2: Final Image ---
# 使用 Playwright 官方镜像，已包含所有浏览器依赖
FROM mcr.microsoft.com/playwright/python:v1.52.0-noble

# 设置主工作目录
WORKDIR /app

# 安装 Supervisor
RUN apt-get update && \
    apt-get install -y --no-install-recommends supervisor && \
    rm -rf /var/lib/apt/lists/*

# 从 Go 构建阶段复制编译好的二进制文件到最终镜像中
COPY --from=builder-go /go_app_binary .

# 从 Node 构建阶段复制编译好的前端文件到最终镜像中
COPY --from=builder-node /app/dist ./log-viewer/dist

# 复制 Python 项目的 requirements.txt 并安装依赖
COPY camoufox-py/requirements.txt ./camoufox-py/requirements.txt
# 增加超时时间
RUN pip install --no-cache-dir --timeout=1000 -r ./camoufox-py/requirements.txt

# 运行 camoufox fetch
RUN camoufox fetch

# 复制 Python 项目的所有文件
COPY camoufox-py/ .

# 复制 Supervisor 的配置文件
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# 容器启动时，运行 Supervisor
# 它会根据 supervisord.conf 的配置来启动你的 Python 和 Go 应用
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
