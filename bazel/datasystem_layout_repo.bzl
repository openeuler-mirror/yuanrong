def _impl(repository_ctx):
    path = repository_ctx.attr.path
    if path[0] != "/":
        cur_dir = repository_ctx.path(Label("//:BUILD.bazel")).dirname
        path = str(cur_dir) + "/" + path

    source_patch = path + "/third_party/patches/spdlog/change-namespace.patch"
    prebuilt_lib = path + "/output/sdk/cpp/lib/libdatasystem.so"
    prebuilt_headers = path + "/output/sdk/cpp/include"
    has_source = repository_ctx.execute(["test", "-f", source_patch]).return_code == 0
    has_prebuilt = (
        repository_ctx.execute(["test", "-f", prebuilt_lib]).return_code == 0 and
        repository_ctx.execute(["test", "-d", prebuilt_headers]).return_code == 0
    )
    use_prebuilt = has_prebuilt and not has_source

    repository_ctx.file("WORKSPACE", "")
    repository_ctx.file("BUILD", "")
    repository_ctx.file(
        "defs.bzl",
        "USE_DATASYSTEM_PREBUILT = %s\nUSE_DATASYSTEM_SOURCE = %s\n" % (
            "True" if use_prebuilt else "False",
            "False" if use_prebuilt else "True",
        ),
    )

datasystem_layout_repo = repository_rule(
    implementation = _impl,
    attrs = {
        "path": attr.string(mandatory = True),
    },
    local = True,
)
