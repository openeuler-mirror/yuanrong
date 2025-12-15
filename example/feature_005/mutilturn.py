import yr

@yr.instance
class Counter:
    def __init__(self, init):
        self.cnt = init

    def add_one(self):
        self.cnt += 1
    
    def get(self):
        return self.cnt
    

yr.init()

cnt = Counter.invoke(0)

futures = []
futures.append(cnt.add_one.options(xx).invoke())
futures.append(cnt.add_one.invoke())
futures.append(cnt.add_one.invoke())

ready_list, unready_list=yr.wait(futures, timeout=30)

value = yr.get(cnt.get.invoke())
print(value)

yr.finalize()