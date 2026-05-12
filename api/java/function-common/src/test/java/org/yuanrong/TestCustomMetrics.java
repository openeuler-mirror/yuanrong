/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package org.yuanrong;

import static org.junit.Assert.assertEquals;
import static org.mockito.Mockito.when;

import org.yuanrong.errorcode.ErrorCode;
import org.yuanrong.errorcode.ErrorInfo;
import org.yuanrong.errorcode.ModuleCode;
import org.yuanrong.errorcode.Pair;
import org.yuanrong.exception.LibRuntimeException;
import org.yuanrong.jni.LibRuntime;

import org.junit.Test;
import org.junit.runner.RunWith;
import org.powermock.api.mockito.PowerMockito;
import org.powermock.core.classloader.annotations.PowerMockIgnore;
import org.powermock.core.classloader.annotations.PrepareForTest;
import org.powermock.core.classloader.annotations.SuppressStaticInitializationFor;
import org.powermock.modules.junit4.PowerMockRunner;

@RunWith(PowerMockRunner.class)
@PrepareForTest({LibRuntime.class})
@SuppressStaticInitializationFor({"org.yuanrong.jni.LibRuntime"})
@PowerMockIgnore("javax.management.*")
public class TestCustomMetrics {
    private static final String NAME = "metric_name";
    private static final String DESCRIPTION = "metric_description";
    private static final String UNIT = "1";

    @Test
    public void testCustomGaugeSuccess() throws LibRuntimeException {
        PowerMockito.mockStatic(LibRuntime.class);
        ErrorInfo ok = new ErrorInfo(ErrorCode.ERR_OK, ModuleCode.RUNTIME, "");
        when(LibRuntime.setGauge(NAME, DESCRIPTION, UNIT, 3.5)).thenReturn(ok);
        when(LibRuntime.increaseGauge(NAME, DESCRIPTION, UNIT, 1.5)).thenReturn(ok);
        when(LibRuntime.decreaseGauge(NAME, DESCRIPTION, UNIT, 0.5)).thenReturn(ok);
        when(LibRuntime.getValueGauge(NAME, DESCRIPTION, UNIT)).thenReturn(new Pair<>(ok, 4.5));

        CustomGauge gauge = new CustomGauge(NAME, DESCRIPTION, UNIT);
        gauge.set(3.5);
        gauge.inc(1.5);
        gauge.dec(0.5);

        assertEquals(4.5, gauge.getValue(), 0.0001);
    }

    @Test
    public void testCustomGaugeThrowsWhenLibRuntimeReturnsError() throws LibRuntimeException {
        PowerMockito.mockStatic(LibRuntime.class);
        ErrorInfo err = new ErrorInfo(ErrorCode.ERR_PARAM_INVALID, ModuleCode.RUNTIME, "bad gauge");
        when(LibRuntime.increaseGauge(NAME, DESCRIPTION, UNIT, 1.0)).thenReturn(err);

        CustomGauge gauge = new CustomGauge(NAME, DESCRIPTION, UNIT);
        try {
            gauge.inc(1.0);
        } catch (LibRuntimeException ex) {
            assertEquals(ErrorCode.ERR_PARAM_INVALID, ex.getErrorCode());
            return;
        }
        org.junit.Assert.fail("expected LibRuntimeException");
    }

    @Test
    public void testCustomGaugeThrowsWhenGetReturnsNull() throws LibRuntimeException {
        PowerMockito.mockStatic(LibRuntime.class);
        when(LibRuntime.getValueGauge(NAME, DESCRIPTION, UNIT)).thenReturn(null);

        CustomGauge gauge = new CustomGauge(NAME, DESCRIPTION, UNIT);
        try {
            gauge.getValue();
        } catch (LibRuntimeException ex) {
            assertEquals(ErrorCode.ERR_PARAM_INVALID, ex.getErrorCode());
            return;
        }
        org.junit.Assert.fail("expected LibRuntimeException");
    }

    @Test
    public void testCustomCounterSuccess() throws LibRuntimeException {
        PowerMockito.mockStatic(LibRuntime.class);
        ErrorInfo ok = new ErrorInfo(ErrorCode.ERR_OK, ModuleCode.RUNTIME, "");
        when(LibRuntime.increaseUInt64Counter(NAME, DESCRIPTION, UNIT, 4L)).thenReturn(ok);
        when(LibRuntime.getValueUInt64Counter(NAME, DESCRIPTION, UNIT)).thenReturn(new Pair<>(ok, 7L));

        CustomCounter counter = new CustomCounter(NAME, DESCRIPTION, UNIT);
        counter.inc(4L);

        assertEquals(7L, counter.getValue());
    }

    @Test
    public void testCustomCounterThrowsWhenLibRuntimeReturnsError() throws LibRuntimeException {
        PowerMockito.mockStatic(LibRuntime.class);
        ErrorInfo err = new ErrorInfo(ErrorCode.ERR_INNER_SYSTEM_ERROR, ModuleCode.RUNTIME, "bad counter");
        when(LibRuntime.increaseUInt64Counter(NAME, DESCRIPTION, UNIT, 2L)).thenReturn(err);

        CustomCounter counter = new CustomCounter(NAME, DESCRIPTION, UNIT);
        try {
            counter.inc(2L);
        } catch (LibRuntimeException ex) {
            assertEquals(ErrorCode.ERR_INNER_SYSTEM_ERROR, ex.getErrorCode());
            return;
        }
        org.junit.Assert.fail("expected LibRuntimeException");
    }
}
