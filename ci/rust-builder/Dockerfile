FROM swr.cn-southwest-2.myhuaweicloud.com/yuanrong-dev/compile-ubuntu2004:v20260428_cmake33110

ARG RUST_TOOLCHAIN=stable

ENV RUSTUP_HOME=/root/.rustup \
    CARGO_HOME=/root/.cargo \
    RUSTUP_DIST_SERVER=https://rsproxy.cn \
    RUSTUP_UPDATE_ROOT=https://rsproxy.cn/rustup \
    CARGO_REGISTRIES_CRATES_IO_PROTOCOL=sparse \
    PATH=/root/.cargo/bin:$PATH

RUN curl -fsSL https://rsproxy.cn/rustup-init.sh -o /tmp/rustup.sh \
 && sh /tmp/rustup.sh -y --profile minimal --default-toolchain ${RUST_TOOLCHAIN} \
 && rm -f /tmp/rustup.sh \
 && mkdir -p /root/.cargo \
 && printf '%s\n' \
      '[source.crates-io]' \
      'replace-with = "rsproxy-sparse"' \
      '' \
      '[source.rsproxy-sparse]' \
      'registry = "sparse+https://rsproxy.cn/index/"' \
      '' \
      '[registries.rsproxy]' \
      'index = "https://rsproxy.cn/crates.io-index"' \
      '' \
      '[net]' \
      'git-fetch-with-cli = true' \
      > /root/.cargo/config.toml \
 && rustc --version \
 && cargo --version
