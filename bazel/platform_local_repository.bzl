def _impl(repository_ctx):
    os_name = repository_ctx.os.name.lower()
    use_stub = ("mac" in os_name or "darwin" in os_name) and repository_ctx.attr.stub_build_file

    repository_ctx.file("WORKSPACE", "")
    if use_stub:
        repository_ctx.symlink(repository_ctx.attr.stub_build_file, repository_ctx.path("BUILD"))
        return

    path = repository_ctx.attr.path
    if path[0] != "/":
        cur_dir = repository_ctx.path(Label("//:BUILD.bazel")).dirname
        path = repository_ctx.path(str(cur_dir) + "/" + path)
    else:
        path = repository_ctx.path(path)

    if not path.exists:
        if repository_ctx.attr.stub_build_file:
            repository_ctx.symlink(repository_ctx.attr.stub_build_file, repository_ctx.path("BUILD"))
            return
        fail("Repository path does not exist: %s" % path)

    source_patch = str(path) + "/third_party/patches/spdlog/change-namespace.patch"
    prebuilt_lib = str(path) + "/output/sdk/cpp/lib/libdatasystem.so"
    prebuilt_headers = str(path) + "/output/sdk/cpp/include"
    has_source = repository_ctx.execute(["test", "-f", source_patch]).return_code == 0
    has_prebuilt = (
        repository_ctx.execute(["test", "-f", prebuilt_lib]).return_code == 0 and
        repository_ctx.execute(["test", "-d", prebuilt_headers]).return_code == 0
    )
    use_prebuilt = repository_ctx.attr.prebuilt_build_file and has_prebuilt and not has_source

    result = repository_ctx.execute([
        "rsync",
        "-a",
        "--delete",
        "--delete-excluded",
        "--exclude=/BUILD/",
        "--exclude=BUILD",
        "--exclude=BUILD.bazel",
        "--exclude=/build/",
    ] + ([] if use_prebuilt else ["--exclude=/output/"]) + [
        "--exclude=/.git/",
        str(path) + "/",
        ".",
    ])
    if result.return_code != 0:
        fail("Failed to copy %s: %s %s" % (path, result.stderr, result.stdout))

    for build_file in ["BUILD", "BUILD.bazel"]:
        build_path = str(repository_ctx.path(build_file))
        result = repository_ctx.execute(["rm", "-rf", build_path])
        if result.return_code != 0:
            fail("Failed to remove existing %s entry %s: %s %s" % (
                build_file,
                build_path,
                result.stderr,
                result.stdout,
            ))

    build_file = repository_ctx.attr.prebuilt_build_file if use_prebuilt else repository_ctx.attr.build_file
    repository_ctx.symlink(build_file, repository_ctx.path("BUILD"))

platform_local_repository = repository_rule(
    implementation = _impl,
    attrs = {
        "path": attr.string(mandatory = True),
        "build_file": attr.label(mandatory = True, allow_single_file = True),
        "prebuilt_build_file": attr.label(allow_single_file = True),
        "stub_build_file": attr.label(allow_single_file = True),
    },
    local = True,
)
