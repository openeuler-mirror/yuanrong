/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

import org.yuanrong.errorcode.ErrorCode;
import org.yuanrong.errorcode.ErrorInfo;
import org.yuanrong.errorcode.Pair;
import org.yuanrong.exception.LibRuntimeException;
import org.yuanrong.jni.LibRuntime;

/**
 * Custom gauge metric helper for Java FaaS functions.
 *
 * @since 2026/04/21
 */
public class CustomGauge {
    private final String name;
    private final String description;
    private final String unit;

    public CustomGauge(String name, String description, String unit) {
        this.name = name;
        this.description = description;
        this.unit = unit;
    }

    /**
     * Sets the gauge to an absolute value.
     *
     * @param value gauge value
     * @throws LibRuntimeException thrown when libruntime reports an error
     */
    public void set(double value) throws LibRuntimeException {
        throwIfError(LibRuntime.setGauge(name, description, unit, value));
    }

    /**
     * Increases the gauge by the given delta.
     *
     * @param value gauge delta
     * @throws LibRuntimeException thrown when libruntime reports an error
     */
    public void inc(double value) throws LibRuntimeException {
        throwIfError(LibRuntime.increaseGauge(name, description, unit, value));
    }

    /**
     * Decreases the gauge by the given delta.
     *
     * @param value gauge delta
     * @throws LibRuntimeException thrown when libruntime reports an error
     */
    public void dec(double value) throws LibRuntimeException {
        throwIfError(LibRuntime.decreaseGauge(name, description, unit, value));
    }

    /**
     * Returns the current gauge value.
     *
     * @return current gauge value
     * @throws LibRuntimeException thrown when libruntime reports an error
     */
    public double getValue() throws LibRuntimeException {
        Pair<ErrorInfo, Double> result = LibRuntime.getValueGauge(name, description, unit);
        if (result == null) {
            throw new LibRuntimeException("get gauge returns null");
        }
        throwIfError(result.getFirst());
        return result.getSecond();
    }

    private static void throwIfError(ErrorInfo errorInfo) throws LibRuntimeException {
        if (errorInfo != null && !ErrorCode.ERR_OK.equals(errorInfo.getErrorCode())) {
            throw new LibRuntimeException(errorInfo.getErrorCode(), errorInfo.getModuleCode(),
                errorInfo.getErrorMessage());
        }
    }
}
