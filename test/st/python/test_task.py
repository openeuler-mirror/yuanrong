#!/usr/bin/env python3
# coding=UTF-8
# Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.

import pytest
import yr
from yr import Affinity, AffinityKind, AffinityType, LabelOperator, OperatorType
from common import get, return_custom_envs

@pytest.mark.skip(reason="Check whether customextension has been written into the request body by viewing log file.")
def test_invoke_function_with_custom_extension(init_yr):
    opt = yr.InvokeOptions()
    opt.custom_extensions = {
        "endpoint": "InvokeFunction1",
        "app_name": "InvokeFunction2",
        "tenant_id": "InvokeFunction3",
    }
    ref = get.options(opt).invoke()
    assert yr.get(ref) == 1, "actual return of method get: {value}, 1 is expected."


@pytest.mark.skip(reason="Check whether 'antiOthers' has been written into the request body by viewing log file.")
def test_anti_other_labels_success(init_yr):
    opt = yr.InvokeOptions()
    opt.preferred_anti_other_labels = True
    op1 = LabelOperator(OperatorType.LABEL_EXISTS, "label1")
    affinity = Affinity(AffinityKind.RESOURCE, AffinityType.PREFERRED, [op1])
    opt.schedule_affinities = [affinity]
    ref = get.options(opt).invoke()
    assert yr.get(ref) == 1, "actual return of method get: {value}, 1 is expected."


@pytest.mark.smoke
def test_task_get_opts_env_var(init_yr):
    opt = yr.InvokeOptions()
    opt.env_vars = {"A" : "A_VARS"}
    ref = return_custom_envs.options(opt).invoke("A")
    assert yr.get(ref) == "A_VARS"


@pytest.mark.smoke
def test_kv_write_read(init_yr):
    yr.kv_write("key1", b"value1")
    v1 = yr.kv_read("key1")
    assert v1 == b"value1"
    yr.kv_del("key1")


@pytest.mark.smoke
def test_kv_write_read_with_param(init_yr):
    set_param = yr.SetParam()
    set_param.existence = yr.ExistenceOpt.NX
    set_param.write_mode = yr.WriteMode.NONE_L2_CACHE_EVICT
    set_param.ttl_second = 100
    # Check whether shared disk is enabled.
    yr.kv_write_with_param("key1", b"value1", set_param)
    v1 = yr.kv_read("key1")
    assert v1 == b"value1"
    yr.kv_del("key1")
