FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git

# 复制go.mod和go.sum文件
COPY passwall/go.mod passwall/go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY passwall/ ./

# 编译应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o passwall ./cmd/server/

# 第二阶段：运行阶段
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置时区为亚洲/上海
ENV TZ=Asia/Shanghai

# 创建工作目录
WORKDIR /app

# 从构建阶段复制编译好的应用
COPY --from=builder /app/passwall /app/

# 设置环境变量
ENV CONFIG_PATH=/app/config.yaml
ENV GIN_MODE=release

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./passwall"] 