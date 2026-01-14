/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

package com.yuanrong.api;

import com.yuanrong.InvokeOptions;
import com.yuanrong.call.InstanceCreator;
import com.yuanrong.call.InstanceHandler;
import com.yuanrong.errorcode.ErrorCode;
import com.yuanrong.errorcode.ErrorInfo;
import com.yuanrong.errorcode.ModuleCode;
import com.yuanrong.exception.YRException;
import com.yuanrong.exception.LibRuntimeException;
import com.yuanrong.exception.handler.traceback.StackTraceUtils;
import com.yuanrong.function.YRFunc4;
import com.yuanrong.jni.LibRuntime;
import com.yuanrong.jobexecutor.JobExecutor;
import com.yuanrong.jobexecutor.RuntimeEnv;
import com.yuanrong.jobexecutor.YRJobInfo;
import com.yuanrong.jobexecutor.YRJobParam;
import com.yuanrong.jobexecutor.YRJobStatus;
import com.yuanrong.libruntime.generated.Libruntime.ApiType;
import com.yuanrong.runtime.client.ObjectRef;
import com.yuanrong.runtime.config.RuntimeContext;
import com.yuanrong.runtime.util.Constants;
import com.yuanrong.storage.InternalWaitResult;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.InvalidPathException;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * The local methods for the invocation for remote runtime JobExecutor actor
 * member functions.
 *
 * @since 2023 /06/06
 */
public class JobExecutorCaller {
    private static final Logger LOGGER = LoggerFactory.getLogger(JobExecutorCaller.class);

    private static final int DEFAULT_JOB_EXECUTOR_INVOKE_TIMEOUT_MS = 30000;

    private static ConcurrentHashMap<String, ConcurrentHashMap<String, YRJobInfo>> jobInfoCaches =
                    new ConcurrentHashMap<>();

    private static final List<ErrorCode> JOB_ERROR_CODES = Arrays.asList(ErrorCode.ERR_JOB_USER_CODE_EXCEPTION,
            ErrorCode.ERR_JOB_RUNTIME_EXCEPTION,
            ErrorCode.ERR_JOB_INNER_SYSTEM_EXCEPTION);

    private static final List<ErrorCode> USER_CODE_ERROR_CODES = Arrays.asList(ErrorCode.ERR_INCORRECT_INIT_USAGE,
            ErrorCode.ERR_INCORRECT_INVOKE_USAGE,
            ErrorCode.ERR_PARAM_INVALID);

    private static final List<ErrorCode> RUNTIME_ERROR_CODES = Arrays.asList(ErrorCode.ERR_INIT_CONNECTION_FAILED,
            ErrorCode.ERR_USER_CODE_LOAD,
            ErrorCode.ERR_PARSE_INVOKE_RESPONSE_ERROR, ErrorCode.ERR_INSTANCE_ID_EMPTY);

    /**
     * Invokes the JobExecutor actor instance.
     *
     * @param yrJobParam a YRJobParam object. All fields in this object are
     *                   required except runtimeEnv.
     * @return String the instanceID of the JobExecutor actor.
     * @throws YRException the actor task exception.
     */
    public static String submitJob(YRJobParam yrJobParam) throws YRException {
        InstanceCreator<JobExecutor> jobExecutor = new InstanceCreator<JobExecutor>(
                (YRFunc4<String, RuntimeEnv, ArrayList<String>, String, JobExecutor>) JobExecutor::new);
        ArrayList<String> entryPoint = yrJobParam.getLocalEntryPoint();
        InvokeOptions invokeOptions = yrJobParam.extractInvokeOptions();
        String objectID = "";
        try {
            InstanceHandler handler = jobExecutor
                .options(invokeOptions)
                .invoke(yrJobParam.getJobName(),
                        yrJobParam.getRuntimeEnv(),
                        entryPoint,
                        invokeOptions.affinityMsgToJsonStr());
            objectID = handler.getInstanceId();
        } catch (YRException e) {
            throw adaptException(e);
        }

        String userJobID;
        try {
            userJobID = LibRuntime.GetRealInstanceId(objectID);
        } catch (LibRuntimeException e) {
            throw new YRException(e.getErrorCode(), e.getModuleCode(), e.getMessage());
        }
        InternalWaitResult waitResult;
        try {
            waitResult = LibRuntime.Wait(Collections.singletonList(objectID), 1, Constants.NO_TIMEOUT);
        } catch (LibRuntimeException e) {
            throw new YRException(e.getErrorCode(), e.getModuleCode(), e.getMessage());
        }
        if (waitResult == null) {
            throw new YRException(ErrorCode.ERR_INNER_SYSTEM_ERROR, ModuleCode.RUNTIME,
                    "failed to create instance");
        }
        if (!waitResult.getExceptionIds().isEmpty()) {
            LOGGER.warn("wait objIds, exception ids size is {}", waitResult.getExceptionIds().size());
            Iterator<Map.Entry<String, ErrorInfo>> it = waitResult.getExceptionIds().entrySet().iterator();
            Map.Entry<String, ErrorInfo> entry = it.next();
            StackTraceUtils.checkErrorAndThrowForInvokeException(entry.getValue(),
                "wait objIds(" + entry.getKey() + "...)");
        }
        // A record needs to be initialized here. Otherwise,
        // no record is found when the 'listjobs()' is invoked.
        getJobInfoCache().put(userJobID, new YRJobInfo());
        return userJobID;
    }

    public static String submitJob(YRJobParam yrJobParam, ObjectRef entryPointObjRef, String pathStr) throws YRException {
        byte[] data = (byte[]) YR.getRuntime().get(entryPointObjRef, DEFAULT_JOB_EXECUTOR_INVOKE_TIMEOUT_MS);
        if (!saveBytesToFile(data, pathStr)) {
            throw new YRException(ErrorCode.ERR_INNER_SYSTEM_ERROR, ModuleCode.RUNTIME,
                    "failed to load entryPoint file");
        }
        return submitJob(yrJobParam);
    }

    public static boolean saveBytesToFile(byte[] data, String outputPath) {
        if (data == null) {
            LOGGER.warn("entryPoint data is null");
            return false;
        }
        if (outputPath == null || outputPath.trim().isEmpty()) {
            LOGGER.warn("outputPath is empty");
            return false;
        }
        try {
            Path path = Paths.get(outputPath);
            Path parent = path.getParent();
            if (parent != null && !Files.exists(parent)) {
                Files.createDirectories(parent);
            }
            // 写入文件
            Files.write(path, data);
            LOGGER.info("File saved successfully: " + outputPath);
            return true;
        } catch (InvalidPathException e) {
            LOGGER.warn("Invalid output path: " + outputPath);
            return false;
        } catch (IOException e) {
            LOGGER.warn("Failed to save the file: " + e.getMessage());
            return false;
        }
    }

    /**
     * Stops the job and release resources of the related attached-runtime process.
     * The JobExecutor actor instance remains alive.
     *
     * @param userJobID the instanceID of remote JobExecutor actor.
     * @throws YRException the actor task exception.
     */
    public static void stopJob(String userJobID) throws YRException {
        try {
            InstanceHandler handler = getInstanceHandler(userJobID);
            handler.function(JobExecutor::stop).invoke();
        } catch (YRException e) {
            throw adaptException(e);
        }
    }

    /**
     * Gets the YRjobInfo object according to a given userJobID.
     *
     * @param userJobID the instanceID of remote JobExecutor actor.
     * @return YRJobInfo object.
     * @throws YRException the actor task exception.
     */
    public static YRJobInfo getYrJobInfo(String userJobID) throws YRException {
        return updateYRJobInfo(userJobID);
    }

    /**
     * Gets the current status of a specified job. The Status can be
     * one of RUNNING/SUCCEEDED/STOPPED or FAILED.
     *
     * @param userJobID the instanceID of remote JobExecutor actor.
     * @return YRJobStatus object indicates the current status of the job.
     * @throws YRException the actor task exception.
     */
    public static YRJobStatus getJobStatus(String userJobID) throws YRException {
        YRJobInfo jobInfo = updateYRJobInfo(userJobID);
        return jobInfo.getStatus();
    }

    /**
     * Obtains Specified jobs information in the current SDK domain.
     *
     * @param userJobIDList String[] or Strings of userJobIDs.
     * @return the Map<String, YRJobInfo> map contains YRjobInfos related to given
     *         userJobIDs.
     * @throws YRException the actor task exception.
     */
    public static Map<String, YRJobInfo> listJobs(String... userJobIDList) throws YRException {
        Map<String, YRJobInfo> jobsMap = new HashMap<>();
        for (String userJobID : userJobIDList) {
            jobsMap.put(userJobID, updateYRJobInfo(userJobID));
        }
        return jobsMap;
    }

    /**
     * Obtains all jobs information in the SDK domain.
     *
     * @return the Map<String, YRJobInfo> map contains YRjobInfo objects.
     * @throws YRException Failed to update jobs information.
     */
    public static Map<String, YRJobInfo> listJobs() throws YRException {
        List<String> keys = new ArrayList<String>(getJobInfoCache().keySet());
        return listJobs(keys.toArray(new String[0]));
    }

    /**
     * Deletes all cached information related to user's job and
     * terminates the corresponding instance.
     *
     * @param userJobID the instanceID of remote JobExecutor actor.
     * @throws YRException the actor task exception.
     */
    public static void deleteJob(String userJobID) throws YRException {
        if (userJobID == null || userJobID.isEmpty()) {
            throw new YRException(ErrorCode.ERR_JOB_USER_CODE_EXCEPTION, ModuleCode.RUNTIME_INVOKE,
                    "Instance ID is empty");
        }
        YR.getRuntime().terminateInstance(userJobID);
        getJobInfoCache().remove(userJobID);
    }

    /**
     * Note that this method may return a null value, which needs to be verified
     * before using its.
     *
     * @param userJobID the job instanceID returned by submitJob.
     * @return null or YRJobInfo object which contains jobInfo.
     * @throws YRException the actor task exception.
     */
    private static YRJobInfo updateYRJobInfo(String userJobID) throws YRException {
        YRJobInfo jobInfo = getJobInfoCache().get(userJobID);
        if (jobInfo != null && jobInfo.ifFinalized()) {
            return new YRJobInfo(jobInfo);
        }

        boolean isWithStatic = false;
        if (jobInfo == null || jobInfo.getJobName() == null) {
            isWithStatic = true;
            jobInfo = new YRJobInfo();
        } else {
            jobInfo = new YRJobInfo(jobInfo);
        }

        Object yrObj;
        try {
            InstanceHandler handler = getInstanceHandler(userJobID);
            ObjectRef objectRef = handler.function(JobExecutor::getJobInfo).invoke(isWithStatic);
            yrObj = YR.getRuntime().get(objectRef, DEFAULT_JOB_EXECUTOR_INVOKE_TIMEOUT_MS);
        } catch (YRException e) {
            LOGGER.error("(JobExecutor) Failed to invoke remote job info to update the job (userJobID: {}). ",
                    userJobID);
            throw adaptException(e);
        }

        if (yrObj instanceof YRJobInfo) {
            jobInfo.update((YRJobInfo) yrObj);
        } else {
            throw new YRException(ErrorCode.ERR_PARAM_INVALID, ModuleCode.RUNTIME,
                "yrObj is not an instance of YRJobInfo");
        }
        getJobInfoCache().put(userJobID, jobInfo);
        return new YRJobInfo(jobInfo);
    }

    private static InstanceHandler getInstanceHandler(String userJobID) {
        return new InstanceHandler(userJobID, ApiType.Function);
    }

    private static YRException adaptException(YRException exception) {
        if (JOB_ERROR_CODES.contains(exception.getErrorCode())) {
            return exception;
        }

        if (USER_CODE_ERROR_CODES.contains(exception.getErrorCode())) {
            return new YRException(ErrorCode.ERR_JOB_USER_CODE_EXCEPTION, exception.getModuleCode(),
                    exception.getErrorMessage());
        }

        if (RUNTIME_ERROR_CODES.contains(exception.getErrorCode())) {
            return new YRException(ErrorCode.ERR_JOB_RUNTIME_EXCEPTION, exception.getModuleCode(),
                    exception.getErrorMessage());
        }

        return new YRException(ErrorCode.ERR_JOB_INNER_SYSTEM_EXCEPTION, exception.getModuleCode(),
                exception.getErrorMessage());
    }

    private static Map<String, YRJobInfo> getJobInfoCache() {
        String runtimeCtx = RuntimeContext.RUNTIME_CONTEXT.get();
        jobInfoCaches.putIfAbsent(runtimeCtx, new ConcurrentHashMap<String, YRJobInfo>());
        return jobInfoCaches.get(runtimeCtx);
    }
}