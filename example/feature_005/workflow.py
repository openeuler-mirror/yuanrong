

import yr

@yr.invoke
def func_a(value): # --> instance
    return value + 1

@yr.invoke
def func_b(value):
    return value * 2

yr.init()

v = 1
ref_a = func_a.invoke(v)
ref_b = func_b.invoke(ref_a) # async

ret_b = yr.get(ref_b) # sync
print(ret_b)