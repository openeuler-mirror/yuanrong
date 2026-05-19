import yr

@yr.instance
class Counter:
    def get(self):
        import os
        return os.environ["INSTANCE_ID"]


yr.init()
cnt=Counter.invoke()
ret=cnt.get.invoke()
print(yr.get(ret))

import time

time.sleep(3000)
