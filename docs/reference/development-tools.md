# 常用工具安装

本节将介绍如果在开发环境中安装一些常用的开发工具。

## 安装 MinIO Client(mc)

Minio Client 简称 mc, 是 minio 服务器的客户端，[官网地址](https://min.io/download)

### 下载地址

linux [下载地址](https://dl.min.io/client/mc/release/linux-amd64/mc), 下载安装包到主机，并执行以下命令安装

```bash
chmod +x mc
sudo mv ./mc /usr/loccal/bin/mc
```

### 常用命令

初始化配置

```bash
查看 openYuanrong 中的 MinIO 服务地址
echo http://$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}'):$(kubectl get svc minio -o jsonpath='{.spec.ports[0].nodePort}')
```

```bash
# mys3: 服务器别名
# http://x.x.x:30110: 服务器地址
# <AK> <SK>: 配置的 MinIO AK/SK
mc alias set mys3 http://x.x.x:9000 <AK> <SK>
# 查看 mys3 信息
mc admin info mys3
```

常用命令

```bash
# 查看 bucket 列表，其中 mys3 是服务端的别名
mc ls mys3

# 查看某个 bucket 下的文件
mc ls mys3/this-bucket

# 创建 bucket
mc mb mys3/this-bucket

# 删除 bucket
mc rm mys3/this-bucket

# 上传文件
mc cp ./demo.zip mys3/this-bucket/demo.zip

# 下载文件
mc cp mys3/this-bucket/demo.zip ./demo.zip

# 删除文件
mc rm mys3/this-bucket/demo.zip

```
