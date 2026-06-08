# Development Tools

This section introduces how to install some commonly used development tools in a Linux environment.

(tools-minio-client)=

## MinIO Client (mc)

[Minio Client](https://minio.org.cn/docs/minio/linux/reference/minio-mc.html){target="_blank"} (referred to as mc) is the client for MinIO servers.

### Installation

Download the `mc` file from the [installation package](https://dl.min.io/client/mc/release){target="_blank"} to the host and execute the following commands to install:

```bash
chmod +x mc
sudo mv ./mc /usr/local/bin/mc
```

### Usage

Initialize configuration:

```bash
View MinIO service address in openYuanrong cluster
echo http://$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}'):$(kubectl get svc minio -o jsonpath='{.spec.ports[0].nodePort}')
```

```bash
# mys3: server alias
# http://x.x.x:30110: server address
# <AK> <SK>: configured MinIO AK/SK
mc alias set mys3 http://x.x.x:30110 <AK> <SK>
# View mys3 information
mc admin info mys3
```

Common commands

```bash
# View bucket list, where mys3 is the server alias
mc ls mys3

# View files under a specific bucket
mc ls mys3/this-bucket

# Create bucket
mc mb mys3/this-bucket

# Delete bucket
mc rm mys3/this-bucket

# Upload file
mc cp ./demo.zip mys3/this-bucket/demo.zip

# Download file
mc cp mys3/this-bucket/demo.zip ./demo.zip

# Delete file
mc rm mys3/this-bucket/demo.zip

```
