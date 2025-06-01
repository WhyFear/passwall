# 第一阶段：构建前端
FROM node:18-alpine AS frontend-builder

# 设置工作目录
WORKDIR /app

# 复制前端项目文件
COPY web/package*.json ./

# 安装依赖
RUN npm install

# 复制源代码
COPY web/ ./

# 构建前端
RUN npm run build

# 第二阶段：构建后端
FROM golang:1.21-alpine AS backend-builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git gcc musl-dev

# 复制go.mod和go.sum文件
COPY passwall/go.mod passwall/go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY passwall/ ./

# 编译应用
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o passwall ./cmd/server/

# 第三阶段：运行阶段
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置时区为亚洲/上海
ENV TZ=Asia/Shanghai

# 创建工作目录
WORKDIR /app

# 创建数据目录
RUN mkdir -p /app/data

# 从构建阶段复制编译好的应用
COPY --from=backend-builder /app/passwall /app/
COPY --from=frontend-builder /app/build /app/web/build

# 复制配置文件示例
COPY config.yaml.example /app/config.yaml

# 设置环境变量
ENV CONFIG_PATH=/app/config.yaml
ENV GIN_MODE=release

# 暴露端口
EXPOSE 8080

# 创建数据卷
VOLUME ["/app/data"]

# 运行应用
CMD ["./passwall"] 