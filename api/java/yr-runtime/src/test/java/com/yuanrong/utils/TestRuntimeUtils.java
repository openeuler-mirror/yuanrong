package com.yuanrong.utils;

import org.apache.logging.log4j.Logger;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.mockito.Mock;
import org.powermock.api.mockito.PowerMockito;
import org.powermock.core.classloader.annotations.PowerMockIgnore;
import org.powermock.core.classloader.annotations.PrepareForTest;
import org.powermock.modules.junit4.PowerMockRunner;

import com.yuanrong.executor.FaaSHandler;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;

@RunWith(PowerMockRunner.class)
@PrepareForTest({RuntimeUtils.class, Files.class, Paths.class})
@PowerMockIgnore({"javax.management.*", "jdk.internal.reflect.*"})
public class TestRuntimeUtils {
    @Mock
    private Logger logger;

    @Mock
    private Path path;

    @Test
    public void testLoadEnvFromFile() throws Exception {
        PowerMockito.mockStatic(Paths.class);
        PowerMockito.mockStatic(Files.class);
        PowerMockito.when(Paths.get("/test/.env")).thenReturn(path);
        PowerMockito.when(Files.exists(path)).thenReturn(true);
        List<String> lines = Arrays.asList("# This is a comment", "","KEY1=VALUE1", "KEY2=VALUE2");
        PowerMockito.when(Files.readAllLines(path, StandardCharsets.UTF_8)).thenReturn(lines);
        PowerMockito.field(RuntimeUtils.class, "LOGGER").set(null, logger);

        RuntimeUtils.loadEnvFromFile("/test/.env");

        System.setProperty("KEY1", "VALUE1");
        System.setProperty("KEY2", "VALUE2");
        verify(logger).debug("Loaded {} environment variables from {}", 2, "/test/.env");
    }

    @Test
    public void testLoadEnvFromFile_file_not_exist() throws Exception {
        PowerMockito.mockStatic(Paths.class);
        PowerMockito.mockStatic(Files.class);
        PowerMockito.when(Paths.get("/test/.env")).thenReturn(path);
        PowerMockito.when(Files.exists(path)).thenReturn(false);
        PowerMockito.field(RuntimeUtils.class, "LOGGER").set(null, logger);

        RuntimeUtils.loadEnvFromFile("/test/.env");

        verify(logger).warn("Environment variable file not found: {}", "/test/.env");
    }

    @Test
    public void testLoadEnvFromFile_read_exception() throws Exception {
        PowerMockito.mockStatic(Paths.class);
        PowerMockito.mockStatic(Files.class);
        PowerMockito.when(Paths.get("/test/.env")).thenReturn(path);
        PowerMockito.when(Files.exists(path)).thenReturn(true);
        IOException exception = new IOException("Permission denied");
        PowerMockito.when(Files.readAllLines(path, StandardCharsets.UTF_8)).thenThrow(exception);
        PowerMockito.field(RuntimeUtils.class, "LOGGER").set(null, logger);

        RuntimeUtils.loadEnvFromFile("/test/.env");

        verify(logger).error("Failed to load environment variables from {}", "/test/.env", exception);
    }

    @Test
    public void testLoadEnvFromFile_null_path() throws Exception {
        RuntimeUtils.loadEnvFromFile(null);
        // Should return without any action
    }

    @Test
    public void testLoadEnvFromFile_empty_path() throws Exception {
        RuntimeUtils.loadEnvFromFile("");
        // Should return without any action
    }

    @Test
    public void testLoadEnvFromFile_whitespace_path() throws Exception {
        RuntimeUtils.loadEnvFromFile("   ");
        // Should return without any action
    }

    @Test
    public void testLoadEnvFromFile_with_comments_and_empty_lines() throws Exception {
        PowerMockito.mockStatic(Paths.class);
        PowerMockito.mockStatic(Files.class);
        PowerMockito.when(Paths.get("/test/.env")).thenReturn(path);
        PowerMockito.when(Files.exists(path)).thenReturn(true);
        List<String> lines = Arrays.asList(
            "# This is a comment",
            "",
            "KEY1=VALUE1",
            "   ",
            "# Another comment",
            "KEY2=VALUE2"
        );
        PowerMockito.when(Files.readAllLines(path, StandardCharsets.UTF_8)).thenReturn(lines);
        PowerMockito.field(RuntimeUtils.class, "LOGGER").set(null, logger);

        RuntimeUtils.loadEnvFromFile("/test/.env");

        PowerMockito.verifyStatic(System.class, times(2));
        System.setProperty("KEY1", "VALUE1");
        System.setProperty("KEY2", "VALUE2");
        verify(logger).debug("Loaded {} environment variables from {}", 2, "/test/.env");
    }

    @Test
    public void testLoadEnvFromFile_invalid_lines() throws Exception {
        PowerMockito.mockStatic(Paths.class);
        PowerMockito.mockStatic(Files.class);
        PowerMockito.when(Paths.get("/test/.env")).thenReturn(path);
        PowerMockito.when(Files.exists(path)).thenReturn(true);
        List<String> lines = Arrays.asList(
            "KEY1=VALUE1",
            "INVALID_LINE_WITHOUT_EQUALS",
            "KEY2=VALUE2",
            "=VALUE_WITHOUT_KEY",
            "KEY3=VALUE3"
        );
        PowerMockito.when(Files.readAllLines(path, StandardCharsets.UTF_8)).thenReturn(lines);
        PowerMockito.field(RuntimeUtils.class, "LOGGER").set(null, logger);

        RuntimeUtils.loadEnvFromFile("/test/.env");

        PowerMockito.verifyStatic(System.class, times(2));
        System.setProperty("KEY1", "VALUE1");
        System.setProperty("KEY2", "VALUE2");
        verify(logger).warn("{}:{}  invalid line, missing '=': {}", "/test/.env", 2, "INVALID_LINE_WITHOUT_EQUALS");
        verify(logger).warn("{}:{}  empty key", "/test/.env", 4);
        verify(logger).debug("Loaded {} environment variables from {}", 2, "/test/.env");
    }

    @Test
    public void testLoadEnvFromFile_no_valid_entries() throws Exception {
        PowerMockito.mockStatic(Paths.class);
        PowerMockito.mockStatic(Files.class);
        PowerMockito.when(Paths.get("/test/.env")).thenReturn(path);
        PowerMockito.when(Files.exists(path)).thenReturn(true);
        List<String> lines = Arrays.asList(
            "# Only comments",
            "",
            "INVALID_LINE"
        );
        PowerMockito.when(Files.readAllLines(path, StandardCharsets.UTF_8)).thenReturn(lines);
        PowerMockito.field(RuntimeUtils.class, "LOGGER").set(null, logger);

        RuntimeUtils.loadEnvFromFile("/test/.env");

        // No system properties should be set
        verify(logger).warn("{}:{}  invalid line, missing '=': {}", "/test/.env", 3, "INVALID_LINE");
        // No debug message since loaded == 0
    }

    @Test
    public void testLoadEnvFromFile_with_spaces() throws Exception {
        PowerMockito.mockStatic(Paths.class);
        PowerMockito.mockStatic(Files.class);
        PowerMockito.when(Paths.get("/test/.env")).thenReturn(path);
        PowerMockito.when(Files.exists(path)).thenReturn(true);
        List<String> lines = Arrays.asList(
            "  KEY1  =  VALUE1  ",
            "KEY2=VALUE2",
            "   KEY3=VALUE3   "
        );
        PowerMockito.when(Files.readAllLines(path, StandardCharsets.UTF_8)).thenReturn(lines);
        PowerMockito.field(RuntimeUtils.class, "LOGGER").set(null, logger);

        RuntimeUtils.loadEnvFromFile("/test/.env");

        PowerMockito.verifyStatic(System.class, times(3));
        System.setProperty("KEY1", "VALUE1");
        System.setProperty("KEY2", "VALUE2");
        System.setProperty("KEY3", "VALUE3");
        verify(logger).debug("Loaded {} environment variables from {}", 3, "/test/.env");
    }

    @Test
    public void testSetJavaProcessEnvMap() throws Exception {
        // 测试setJavaProcessEnvMap是否能让System.getenv()获取到环境变量
        Map<String, String> testEnvMap = new HashMap<>();
        String testKey = "TEST_FAAS_ENV_KEY_" + System.currentTimeMillis();
        String testValue = "TEST_FAAS_ENV_VALUE_" + System.currentTimeMillis();

        testEnvMap.put(testKey, testValue);

        // 调用setJavaProcessEnvMap方法
        FaaSHandler.setJavaProcessEnvMap(testEnvMap);

        // 验证System.getenv()是否能获取到
        String retrievedValue = System.getenv(testKey);
        assert retrievedValue != null : "环境变量未设置成功";
        assert retrievedValue.equals(testValue) : "环境变量值不匹配";

        // 验证大小写不敏感（在某些系统上）
        String upperCaseKey = testKey.toUpperCase();
        if (!testKey.equals(upperCaseKey)) {
            String upperValue = System.getenv(upperCaseKey);
            // 注意：大小写不敏感取决于具体实现，可能不一定能获取到
            if (upperValue != null) {
                assert upperValue.equals(testValue) : "大小写不敏感环境变量值不匹配";
            }
        }
    }
}