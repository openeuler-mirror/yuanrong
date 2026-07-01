package(default_visibility = ["//visibility:public"])

cc_import(
    name = "datasystem_shared_import",
    shared_library = "output/sdk/cpp/lib/libdatasystem.so",
)

cc_library(
    name = "datasystem_client_lib",
    hdrs = glob(["output/sdk/cpp/include/**/*.h"]),
    includes = ["output/sdk/cpp/include"],
    deps = [":datasystem_shared_import"],
)

cc_library(
    name = "lib_datasystem_sdk",
    deps = [":datasystem_client_lib"],
)

filegroup(
    name = "shared",
    srcs = glob(["output/sdk/cpp/lib/*.so*"]),
)

filegroup(
    name = "public_hdrs",
    srcs = glob(["output/sdk/cpp/include/**/*.h"]),
)
