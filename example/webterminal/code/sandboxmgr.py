import yr
import os


@yr.instance
class MockSandbox:
    def hello(self):
        return os.environ.get("INSTANCE_ID")


def init(ctx):
   # cfg = yr.Config()
   # cfg.log_level="DEBUG"
   # cfg.function_id = "sn:cn:yrk:default:function:0-defaultservice-py39:$latest"
   # yr.init(cfg)
    return


def create():
    print("create", yr.__file__)
    opt = yr.InvokeOptions()
    opt.custom_extensions["lifecycle"] = "detached"
    opt.idle_timeout = 60 * 60 * 24 * 10

    sandbox = MockSandbox.options(opt).invoke()
    instance = yr.get(sandbox.hello.invoke())   
    print(f"sandbox created, name={instance}")
    return instance


def delete(instance_name):
    yr.kill_instance(instance_name)


def handler(event, ctx):
    cfg = yr.Config()
    cfg.log_level="DEBUG"
    cfg.function_id = "sn:cn:yrk:default:function:0-defaultservice-py39:$latest"
    yr.init(cfg)
    try:
        if event.get("action") == "create":
            instance_name = create()
            return {"instance": instance_name}
        elif event.get("action") == "delete":
            instance_name = event.get("instance")
            if instance_name:
                delete(instance_name)
                return {"message": f"Instance {instance_name} deleted successfully"}
            else:
                return {"error": "Instance name is required for deletion"}
    except Exception as e:
        return {"error": str(e)}
    return {"error": f"unknown action: {event.get('action')}"}
