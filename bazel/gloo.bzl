cc_import(
    name = "gloo",
    hdrs = glob(["include/gloo/**/*.h"]),
    shared_library = "lib/libgloo.so",
    visibility = ["//visibility:public"],
)