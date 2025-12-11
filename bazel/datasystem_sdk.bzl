load("@yuanrong_multi_language_runtime//bazel:yr.bzl", "filter_files_with_suffix")

cc_library(
    name = "lib_datasystem_sdk",
    srcs = [
        "lib/libdatasystem.so",
        "lib/libsecurec.so",
        "lib/libzmq.so.5",
    ] + glob([
        "lib/libds-spdlog.so.*",
        "lib/libtbb.so*",
        "lib/libcurl.so*",
    ]),
    hdrs = glob(["include/**/*.h"]),
    strip_include_prefix = "include",
    visibility = ["//visibility:public"],
    alwayslink = True,
)

filter_files_with_suffix(
    name = "shared",
    srcs = glob(["lib/lib*.so*"]),
    suffix = ".so",
    visibility = ["//visibility:public"],
)
