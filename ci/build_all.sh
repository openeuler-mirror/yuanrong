#!/bin/bash
# 终极全量编译脚本 - 全手动绕过版
set -e

echo "=== [1/4] 环境初始化 ==="
export PATH=/opt/buildtools/python3.9/bin:/usr/local/bin:/usr/bin:$PATH
export CC=gcc-10
export CXX=g++-10
export PIP_BREAK_SYSTEM_PACKAGES=1

CACHE_BASE="/mnt/paas/build-cache"
mkdir -p $CACHE_BASE/ccache $CACHE_BASE/go-mod $CACHE_BASE/go-cache $CACHE_BASE/opensource
export CCACHE_DIR=$CACHE_BASE/ccache
export GOMODCACHE=$CACHE_BASE/go-mod
export GOCACHE=$CACHE_BASE/go-cache
export DS_OPENSOURCE_DIR=$CACHE_BASE/opensource
export CMAKE_C_COMPILER_LAUNCHER=ccache
export CMAKE_CXX_COMPILER_LAUNCHER=ccache

echo "=== [2/4] 源码同步 ==="
git clone https://gitcode.com/openeuler/yuanrong-frontend.git frontend || echo "Frontend clone failed or exists"
git clone https://gitcode.com/openeuler/yuanrong-datasystem.git datasystem || echo "Datasystem clone failed or exists"
git clone https://gitcode.com/yuchaow/yuanrong-functionsystem.git functionsystem || echo "Functionsystem clone failed or exists"

echo "=== [3/4] 逐组件手动编译 (Bypass Makefile) ==="

# 1. Frontend
echo ">>> Building Frontend..."
cd frontend && bash build.sh && cd ..

# 2. Datasystem
echo ">>> Building Datasystem..."
cd datasystem && bash build.sh -X off -G on -i on && cd ..

# 3. Functionsystem
echo ">>> Building Functionsystem..."
cd functionsystem && bash run.sh build -j 24 && cd ..

echo "=== [4/4] 汇总与上传 ==="
mkdir -p output
cp frontend/output/*.tar.gz output/ 2>/dev/null || true
cp datasystem/output/*.tar.gz output/ 2>/dev/null || true
# ... 其它产物拷贝

ls -lh output/
