# Cross-Language Invocation of Stateless and Stateful Functions

openYuanrong supports cross-language invocation of stateless and stateful functions. This section introduces how to develop through example projects.

- [Calling C++ Stateless and Stateful Functions from Python Programs](cross-language-python-invoke-cpp)
- [Calling Java Stateless and Stateful Functions from Python Programs](cross-language-python-invoke-java)
- [Calling Python Stateless and Stateful Functions from C++ Programs](cross-language-cpp-invoke-python)
- [Calling Java Stateless and Stateful Functions from C++ Programs](cross-language-cpp-invoke-java)
- [Calling C++ Stateless and Stateful Functions from Java Programs](cross-language-java-invoke-cpp)

## Prerequisites

Refer to [Deploy on Hosts](../../deploy/deploy_processes/index.md) to complete openYuanrong deployment.

:::{Note}

Ensure the `--enable_meta_service=true` parameter is configured during deployment to start the meta service component for function registration.

:::

(cross-language-python-invoke-cpp)=

## Calling C++ Stateless and Stateful Functions from Python Programs

We compile C++ stateless and stateful functions into dynamic libraries, and use [yr.cpp_function](../api/distributed_programming/Python/yr.cpp_function.rst) and [yr.cpp_instance_class](../api/distributed_programming/Python/yr.cpp_instance_class.rst) APIs in the Python main program for invocation.

1. Prepare the example project

    Create a new directory on all nodes in the openYuanrong cluster, for example: `/opt/mycode/python-invoke-cpp`, to store C++ stateless and stateful function code packages.

    Create the project directory `python-invoke-cpp` and files as follows:
   - calculator.cpp: Defines C++ stateless function `Square` and stateful function `Counter`.
   - CMakeLists.txt: CMake configuration file for building the C++ project.
   - build: Empty directory for storing files generated during C++ compilation.
   - main.py: Python main program, calling C++ stateless function `Square` and stateful function `Counter`.

    ```bash
    python-invoke-cpp
    ├── calculator.cpp
    ├── CMakeLists.txt
    ├── build
    └── main.py
    ```

    :::{dropdown} calculator.cpp file content
    :chevron: down-up
    :icon: chevron-down

    ```cpp
    #include "yr/yr.h"
    int Square(int x)
    {
        return x * x;
    }

    // Define stateless function Square
    YR_INVOKE(Square)

    class Counter {
    public:
        Counter() {}
        Counter(int init) { count = init; }

        static Counter *FactoryCreate(int init) {
            return new Counter(init);
        }

        void Add() {
            count += 1;
        }

        int Get() {
            return count;
        }

        YR_STATE(count);

    public:
        int count;
    };

    // Define stateful function Counter
    YR_INVOKE(Counter::FactoryCreate, &Counter::Add, &Counter::Get);
    ```

    :::
    :::{dropdown} CMakeLists.txt file content, **need to modify YR_INSTALL_PATH to your openYuanrong installation path**
    :chevron: down-up
    :icon: chevron-down

    ```cmake
    cmake_minimum_required(VERSION 3.16.1)
    project(calculator CXX)

    set(CMAKE_CXX_STANDARD 17)
    set(SOURCE_DIR ${CMAKE_CURRENT_SOURCE_DIR})
    set(BINARY_DIR ${SOURCE_DIR}/build)
    set(BUILD_SHARED_LIBS ON)

    # Replace YR_INSTALL_PATH value with actual openYuanrong installation path
    set(YR_INSTALL_PATH "/usr/local/lib/python3.9/site-packages/yr")
    link_directories(${YR_INSTALL_PATH}/cpp/lib)
    include_directories(
        ${YR_INSTALL_PATH}/cpp/include
    )

    add_library(calculator SHARED calculator.cpp)
    set(CMAKE_RUNTIME_OUTPUT_DIRECTORY ${BINARY_DIR})
    ```

    :::
    :::{dropdown} main.py file content
    :chevron: down-up
    :icon: chevron-down

    ```python
    import yr

    yr.init()

    # We will register C++ functions in subsequent steps, generating the following URN
    cpp_function_urn = "sn:cn:yrk:default:function:0-yr-mycpp:$latest"

    # Call C++ stateless function Square
    cpp_function = yr.cpp_function("Square", cpp_function_urn)
    res = cpp_function.invoke(2)
    print(yr.get(res))

    # Call C++ stateful function Counter's Add and Get methods
    cpp_function_instance = yr.cpp_instance_class("Counter", "Counter::FactoryCreate", cpp_function_urn).invoke(0)
    res = cpp_function_instance.Add.invoke()
    yr.get(res)
    res = cpp_function_instance.Get.invoke()
    print(yr.get(res))

    # Destroy function instance
    cpp_function_instance.terminate()
    yr.finalize()
    ```

    :::

2. Build C++ functions into dynamic library

    In the `python-invoke-cpp/build` directory, execute the following command to build:

    ```bash
    cmake ..
    make
    ```

    Successful build will generate dynamic library file `libcalculator.so` in this directory. Copy the file to the code package directory you created on all nodes, for example: `/opt/mycode/python-invoke-cpp`.

3. Register C++ functions as openYuanrong functions

    Use curl tool to call the [Register Function API](../api/function_service/register_function.md) to register an openYuanrong function named `0-yr-mycpp`, with corresponding function URN `sn:cn:yrk:default:function:0-yr-mycpp:$latest`.

    ```bash
    # Replace /opt/mycode/python-invoke-cpp with your compiled dynamic library directory
    META_SERVICE_ENDPOINT=<meta service component endpoint, default: http://{master node IP}:31182>
    curl -H "Content-type: application/json" -X POST -i ${META_SERVICE_ENDPOINT}/serverless/v1/functions -d '{"name":"0-yr-mycpp","runtime":"cpp11","kind":"yrlib","cpu":500,"memory":500,"timeout":60,"storageType":"local","codePath":"/opt/mycode/python-invoke-cpp"}'
    ```

4. Run Python main program for testing

    Run the main program in the `python-invoke-cpp` directory

    ```bash
    python main.py
    # Outputs 4 and 1
    ```

(cross-language-python-invoke-java)=

## Calling Java Stateless and Stateful Functions from Python Programs

We use [yr.cpp_function](../api/distributed_programming/Python/yr.cpp_function.rst) and [yr.cpp_instance_class](../api/distributed_programming/Python/yr.cpp_instance_class.rst) APIs in the Python main program to call Java stateless and stateful functions.

1. Prepare the example project

    Create a new directory on all nodes in the openYuanrong cluster, for example: `/opt/mycode/python-invoke-java`, to store Java stateless and stateful function code packages.

    Create the project directory `python-invoke-java` and files as follows:
   - main.py: Python main program, calling Java stateless function `square` and stateful function `Counter`.
   - pom.xml: Maven configuration file for building the Java project.
   - Square.java: Defines stateless function `square`.
   - Counter.java: Defines stateful function `Counter`.

    ```bash
    python-invoke-java
    ├── main.py
    ├── pom.xml
    └── src
        └── main
            └── java
                └── com
                    └── yuanrong
                        └── demo
                            ├── Counter.java
                            └── Square.java
    ```

    :::{dropdown} main.py file content
    :chevron: down-up
    :icon: chevron-down

    ```python
    import yr

    yr.init()

    # We will register Java functions in subsequent steps, generating the following URN
    java_function_urn = "sn:cn:yrk:default:function:0-yr-myjava:$latest"

    # Call Java stateless function square
    java_function = yr.java_function("com.yuanrong.demo.Square", "square", java_function_urn)
    res = java_function.invoke(2)
    print(yr.get(res))


    # Call Java stateful function Counter's Add and Get methods
    java_instance = yr.java_instance_class("com.yuanrong.demo.Counter", java_function_urn).invoke(1)
    res = java_instance.add.invoke()
    yr.get(res)
    res = java_instance.get.invoke()
    print(yr.get(res))

    # Destroy function instance
    java_instance.terminate()
    yr.finalize()
    ```

    :::
    :::{dropdown} pom.xml file content
    :chevron: down-up
    :icon: chevron-down

    ```xml
    <?xml version="1.0" encoding="UTF-8"?>

    <project xmlns="http://maven.apache.org/POM/4.0.0"
            xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
            xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
        <modelVersion>4.0.0</modelVersion>

        <groupId>com.yuanrong.demo</groupId>
        <artifactId>calculator</artifactId>
        <version>1.0.0</version>

        <properties>
            <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
            <maven.compiler.source>1.8</maven.compiler.source>
            <maven.compiler.target>1.8</maven.compiler.target>
        </properties>

        <build>
            <plugins>
                <plugin>
                    <groupId>org.apache.maven.plugins</groupId>
                    <artifactId>maven-assembly-plugin</artifactId>
                    <executions>
                        <execution>
                            <phase>package</phase>
                            <goals>
                                <goal>single</goal>
                            </goals>
                        </execution>
                    </executions>
                    <configuration>
                        <descriptorRefs>
                            <descriptorRef>jar-with-dependencies</descriptorRef>
                        </descriptorRefs>
                        <appendAssemblyId>false</appendAssemblyId>
                    </configuration>
                </plugin>
            </plugins>
        </build>
    </project>
    ```

    :::
    :::{dropdown} Square.java file content
    :chevron: down-up
    :icon: chevron-down

    ```java
    package com.yuanrong.demo;

    public class Square {
        public static int square(int x) {
            return x * x;
        }
    }
    ```

    :::
    :::{dropdown} Counter.java file content
    :chevron: down-up
    :icon: chevron-down

    ```java
    package com.yuanrong.demo;

    public class Counter {
        private int count = 0;

        public Counter(int count) {
            this.count = count;
        }

        public void add() {
            this.count += 1;
        }

        public int get() {
            return this.count;
        }
    }
    ```

    :::

2. Build Java project

    In the `python-invoke-java` directory, execute the following command to build:

    ```bash
    mvn clean package
    ```

    Successful build will generate `calculator-1.0.0.jar` file in the `python-invoke-java/target` directory. Copy the file to the code package directory you created on all nodes, for example: `/opt/mycode/python-invoke-java`.

3. Register Java functions as openYuanrong functions

    Use curl tool to call the [Register Function API](../api/function_service/register_function.md) to register an openYuanrong function named `0-yr-myjava`, with corresponding function URN `sn:cn:yrk:default:function:0-yr-myjava:$latest`.

     ```bash
    # Replace /opt/mycode/python-invoke-java with your code package directory
    META_SERVICE_ENDPOINT=<meta service component endpoint, default: http://{master node IP}:31182>
    curl -H "Content-type: application/json" -X POST -i ${META_SERVICE_ENDPOINT}/serverless/v1/functions -d '{"name":"0-yr-myjava","runtime":"java1.8","kind":"yrlib","cpu":500,"memory":500,"timeout":60,"storageType":"local","codePath":"/opt/mycode/python-invoke-java"}'
    ```

4. Run Python main program for testing

    Run the main program in the `python-invoke-java` directory

    ```bash
    python main.py
    # Outputs 4 and 2
    ```

(cross-language-cpp-invoke-python)=

## Calling Python Stateless and Stateful Functions from C++ Programs

We use [PyFunction](../api/distributed_programming/Cpp/PyFunction.md) and [PyInstanceClass::FactoryCreate](../api/distributed_programming/Cpp/PyInstanceClass-FactoryCreate.md) APIs in the C++ main program to call Python stateless and stateful functions.

1. Prepare the example project

    Create a new directory on all nodes in the openYuanrong cluster, for example: `/opt/mycode/cpp-invoke-python`, to store Python stateless and stateful function code packages.

    Create the project directory `cpp-invoke-python` and files as follows:
   - calculator.py: Defines stateless function `square` and stateful function `Counter`.
   - main.cpp: C++ main program, calling Python stateless function `square` and stateful function `Counter`.
   - CMakeLists.txt: CMake configuration file for building the C++ project.
   - build: Empty directory for storing files generated during C++ compilation.

    ```bash
    cpp-invoke-python
    ├── calculator.py
    ├── main.cpp
    ├── CMakeLists.txt
    └── build
    ```

    :::{dropdown} calculator.py file content
    :chevron: down-up
    :icon: chevron-down

    ```python
    def square(x):
        return x * x

    class Counter:
        def __init__(self, count):
            self.count = count

        def add(self):
            self.count += 1

        def get(self):
            return self.count
    ```

    :::
    :::{dropdown} main.cpp file content
    :chevron: down-up
    :icon: chevron-down

    ```cpp
    #include <iostream>
    #include "yr/yr.h"

    int main(int argc, char **argv)
    {
        // We will register Python functions in subsequent steps, generating the following URN
        std::string pyFunctionUrn = "sn:cn:yrk:default:function:0-yr-mypython:$latest";

        YR::Config conf;
        YR::Init(conf, argc, argv);

        // Call Python stateless function square
        auto resFutureSquare = YR::PyFunction<int>("calculator", "square").SetUrn(pyFunctionUrn).Invoke(2);
        auto resSquare = *YR::Get(resFutureSquare);
        std::cout << resSquare << std::endl;

        // Call Python stateful function Counter's add and get methods
        auto instanceHandler = YR::PyInstanceClass::FactoryCreate("calculator", "Counter");
        auto instance = YR::Instance(instanceHandler).SetUrn(pyFunctionUrn).Invoke(0);
        auto resFutureAdd = instance.PyFunction<void>("add").Invoke();
        YR::Wait(resFutureAdd);

        auto resFutureGet = instance.PyFunction<int>("get").Invoke();
        auto resGet = *YR::Get(resFutureGet);
        std::cout << resGet << std::endl;

        // Destroy function instance
        instance.Terminate();
        YR::Finalize();
        return 0;
    }
    ```

    :::
    :::{dropdown} CMakeLists.txt file content, **need to modify YR_INSTALL_PATH to your openYuanrong installation path**
    :chevron: down-up
    :icon: chevron-down

    ```cmake
    cmake_minimum_required(VERSION 3.16.1)
    project(main CXX)

    set(CMAKE_CXX_STANDARD 17)
    set(SOURCE_DIR ${CMAKE_CURRENT_SOURCE_DIR})
    set(BINARY_DIR ${SOURCE_DIR}/build)

    # Replace YR_INSTALL_PATH value with actual openYuanrong installation path
    set(YR_INSTALL_PATH "/usr/local/lib/python3.9/site-packages/yr")
    link_directories(${YR_INSTALL_PATH}/cpp/lib)
    include_directories(
        ${YR_INSTALL_PATH}/cpp/include/faas
        ${YR_INSTALL_PATH}/cpp/include
    )

    add_executable(main main.cpp)
    target_link_libraries(main yr-api)
    set(CMAKE_RUNTIME_OUTPUT_DIRECTORY ${BINARY_DIR})
    ```

2. Register Python functions as openYuanrong functions

    First copy the `calculator.py` file to the code package directory on all nodes. Then use curl tool to call the [Register Function API](../api/function_service/register_function.md) to register an openYuanrong function named `0-yr-mypython`, with corresponding function URN `sn:cn:yrk:default:function:0-yr-mypython:$latest`.

    ```bash
    # Replace /opt/mycode/cpp-invoke-python with your code package directory
    META_SERVICE_ENDPOINT=<meta service component endpoint, default: http://{master node IP}:31182>
    curl -H "Content-type: application/json" -X POST -i ${META_SERVICE_ENDPOINT}/serverless/v1/functions -d '{"name":"0-yr-mypython","runtime":"python3.9","kind":"yrlib","cpu":500,"memory":500,"timeout":60,"storageType":"local","codePath":"/opt/mycode/cpp-invoke-python"}'
    ```

3. Run C++ main program for testing

    In the `cpp-invoke-python/build` directory, execute the following command to build:

    ```bash
    cmake ..
    make
    ```

    Successful build will generate executable file `main` in this directory. Execute the following command to run the main program.

    ```bash
    ./main
    # Outputs 4 and 1
    ```

(cross-language-cpp-invoke-java)=

## Calling Java Stateless and Stateful Functions from C++ Programs

We use [PyFunction](../api/distributed_programming/Cpp/PyFunction.md) and [PyInstanceClass::FactoryCreate](../api/distributed_programming/Cpp/PyInstanceClass-FactoryCreate.md) APIs in the C++ main program to call Java stateless and stateful functions.

1. Prepare the example project

    Create a new directory on all nodes in the openYuanrong cluster, for example: `/opt/mycode/cpp-invoke-java`, to store Java stateless and stateful function code packages.
    
    Create the project directory `cpp-invoke-java` and files as follows:
   - main.cpp: C++ main program, calling Java stateless function `square` and stateful function `Counter`.
   - CMakeLists.txt: CMake configuration file for building the C++ project.
   - build: Empty directory for storing files generated during C++ compilation.
   - pom.xml: Maven configuration file for building the Java project.
   - Counter.java: Defines Java stateful function `Counter`.
   - Square.java: Defines Java stateless function `square`.

    ```bash
    cpp-invoke-java
    ├── cpp-project
    │   ├── main.cpp
    │   ├── CMakeLists.txt
    │   └── build
    └── java-project
        ├── pom.xml
        └── src
            └── main
                └── java
                    └── com
                        └── yuanrong
                            └── demo
                                ├── Counter.java
                                └── Square.java
    ```

    :::{dropdown} main.cpp file content
    :chevron: down-up
    :icon: chevron-down

    ```cpp
    #include <iostream>
    #include "yr/yr.h"

    int main(int argc, char **argv)
    {
        // We will register Java functions in subsequent steps, generating the following URN
        std::string javaFunctionUrn = "sn:cn:yrk:default:function:0-yr-myjava:$latest";

        YR::Config conf;
        YR::Init(conf, argc, argv);

        // Call Java stateless function square
        auto resFutureSquare = YR::JavaFunction<int>("com.yuanrong.demo.Square", "square").SetUrn(javaFunctionUrn).Invoke(2);
        auto resSquare = *YR::Get(resFutureSquare);
        std::cout << resSquare << std::endl;

        // Call Java stateful function Counter's add and get methods
        auto instanceHandler = YR::JavaInstanceClass::FactoryCreate("com.yuanrong.demo.Counter");
        auto instance = YR::Instance(instanceHandler).SetUrn(javaFunctionUrn).Invoke(1);
        auto resFutureAdd = instance.JavaFunction<void>("add").Invoke();
        // YR::Get(resFutureAdd);

        auto resFutureGet = instance.JavaFunction<int>("get").Invoke();
        auto resGet = *YR::Get(resFutureGet);
        std::cout << resGet << std::endl;

        // Destroy function instance
        instance.Terminate();
        YR::Finalize();
    }
    ```

    :::
    :::{dropdown} CMakeLists.txt file content, **need to modify YR_INSTALL_PATH to your openYuanrong installation path**
    :chevron: down-up
    :icon: chevron-down

    ```cmake
    cmake_minimum_required(VERSION 3.16.1)
    project(main CXX)

    set(CMAKE_CXX_STANDARD 17)
    set(SOURCE_DIR ${CMAKE_CURRENT_SOURCE_DIR})
    set(BINARY_DIR ${SOURCE_DIR}/build)

    # Replace YR_INSTALL_PATH value with actual openYuanrong installation path
    set(YR_INSTALL_PATH "/usr/local/lib/python3.9/site-packages/yr")
    link_directories(${YR_INSTALL_PATH}/cpp/lib)
    include_directories(
        ${YR_INSTALL_PATH}/cpp/include/faas
        ${YR_INSTALL_PATH}/cpp/include
    )

    add_executable(main main.cpp)
    target_link_libraries(main yr-api)
    set(CMAKE_RUNTIME_OUTPUT_DIRECTORY ${BINARY_DIR})
    ```

    :::
    :::{dropdown} pom.xml file content
    :chevron: down-up
    :icon: chevron-down

    ```xml
    <?xml version="1.0" encoding="UTF-8"?>

    <project xmlns="http://maven.apache.org/POM/4.0.0"
            xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
            xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
        <modelVersion>4.0.0</modelVersion>

        <groupId>com.yuanrong.demo</groupId>
        <artifactId>calculator</artifactId>
        <version>1.0.0</version>

        <properties>
            <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
            <maven.compiler.source>1.8</maven.compiler.source>
            <maven.compiler.target>1.8</maven.compiler.target>
        </properties>

        <build>
            <plugins>
                <plugin>
                    <groupId>org.apache.maven.plugins</groupId>
                    <artifactId>maven-assembly-plugin</artifactId>
                    <executions>
                        <execution>
                            <phase>package</phase>
                            <goals>
                                <goal>single</goal>
                            </goals>
                        </execution>
                    </executions>
                    <configuration>
                        <descriptorRefs>
                            <descriptorRef>jar-with-dependencies</descriptorRef>
                        </descriptorRefs>
                        <appendAssemblyId>false</appendAssemblyId>
                    </configuration>
                </plugin>
            </plugins>
        </build>
    </project>
    ```

    :::
    :::{dropdown} Square.java file content
    :chevron: down-up
    :icon: chevron-down

    ```java
    package com.yuanrong.demo;

    public class Square {
        public static int square(int x) {
            return x * x;
        }
    }
    ```

    :::
    :::{dropdown} Counter.java file content
    :chevron: down-up
    :icon: chevron-down

    ```java
    package com.yuanrong.demo;

    public class Counter {
        private int count = 0;

        public Counter(int count) {
            this.count = count;
        }

        public void add() {
            this.count += 1;
        }

        public int get() {
            return this.count;
        }
    }
    ```

    :::

2. Build Java project

    In the `cpp-invoke-java/java-project` directory, execute the following command to build:

    ```bash
    mvn clean package
    ```

    Successful build will generate `calculator-1.0.0.jar` file in the `cpp-invoke-java/java-project/target` directory. Copy the file to the code package directory on all nodes, for example: `/opt/mycode/cpp-invoke-java`.

3. Register Java functions as openYuanrong functions

    Use curl tool to call the [Register Function API](../api/function_service/register_function.md) to register an openYuanrong function named `0-yr-myjava`, with corresponding function URN `sn:cn:yrk:default:function:0-yr-myjava:$latest`.

    ```bash
    # Replace /opt/mycode/cpp-invoke-java with your code package directory
    META_SERVICE_ENDPOINT=<meta service component endpoint, default: http://{master node IP}:31182>
    curl -H "Content-type: application/json" -X POST -i ${META_SERVICE_ENDPOINT}/serverless/v1/functions -d '{"name":"0-yr-myjava","runtime":"java1.8","kind":"yrlib","cpu":500,"memory":500,"timeout":60,"storageType":"local","codePath":"/opt/mycode/cpp-invoke-java"}'
    ```

4. Build and run C++ main program for testing

    In the `cpp-invoke-java/cpp-project/build` directory, execute the following command to build:

    ```bash
    cmake ..
    make
    ```

    Successful build will generate executable file `main` in this directory. Execute the following command to run the main program.

    ```bash
    ./main
    # Outputs 4 and 2
    ```

(cross-language-java-invoke-cpp)=

## Calling C++ Stateless and Stateful Functions from Java Programs

We compile C++ stateless and stateful functions into dynamic libraries, and use [CppFunction](../api/distributed_programming/Java/cpp-function.md) and [CppInstanceClass](../api/distributed_programming/Java/cpp_instance_class.md) APIs in the Java main program for invocation.

1. Prepare the example project

    Create a new directory on all nodes in the openYuanrong cluster, for example: `/opt/mycode/java-invoke-cpp`, to store C++ stateless and stateful function code packages.

    Create the project directory `java-invoke-cpp` and files as follows:
   - calculator.cpp: Defines C++ stateless function `Square` and stateful function `Counter`.
   - CMakeLists.txt: CMake configuration file for building the C++ project.
   - build: Empty directory for storing files generated during C++ compilation.
   - pom.xml: Maven configuration file for building the Java project.
   - Main.java: Java main program, calling C++ stateless function `Square` and stateful function `Counter`.

    ```bash
    java-invoke-cpp
    ├── cpp-project
    │   ├── calculator.cpp
    │   ├── CMakeLists.txt
    │   └── build
    └── java-project
        ├── pom.xml
        └── src
            └── main
                └── java
                    └── com
                        └── yuanrong
                            └── demo
                                └── Main.java
    ```

    :::{dropdown} Main.java file content
    :chevron: down-up
    :icon: chevron-down

    ```java
    package com.yuanrong.demo;

    import org.yuanrong.api.YR;
    import org.yuanrong.runtime.client.ObjectRef;
    import org.yuanrong.call.CppInstanceHandler;
    import org.yuanrong.function.CppFunction;
    import org.yuanrong.function.CppInstanceMethod;
    import org.yuanrong.function.CppInstanceClass;

    public class Main {
        public static void main(String[] args) throws Exception {
            YR.init();
            // We will register C++ functions in subsequent steps, generating the following URN
            String cppFunctionUrn = "sn:cn:yrk:default:function:0-yr-mycpp:$latest";

            try {
                // Call stateless function Square
                ObjectRef ref = YR.function(CppFunction.of("Square", int.class))
                    .setUrn(cppFunctionUrn)
                    .invoke(2);
                int res = (int) YR.get(ref, 30);
                System.out.println(res);

                // Call stateful function Counter's Add and Get methods
                CppInstanceHandler cppInstance = YR.instance(CppInstanceClass.of("Counter", "FactoryCreate"))
                    .setUrn(cppFunctionUrn)
                    .invoke(1);
                ObjectRef addRef = cppInstance.function(CppInstanceMethod.of("Add", void.class)).invoke();
                YR.get(addRef, 30);

                ObjectRef getRef = cppInstance.function(CppInstanceMethod.of("Get", int.class)).invoke();
                res = (int) YR.get(getRef, 30);

                System.out.println(res);
                cppInstance.terminate();
            } finally {
                YR.Finalize();
            }
        }
    }
    ```

    :::
    :::{dropdown} pom.xml file content, **refer to [Install Java SDK](install-yuanrong-java-sdk) and configure dependency distributed-actortask-sdk**
    :chevron: down-up
    :icon: chevron-down

    ```xml
    <?xml version="1.0" encoding="UTF-8"?>
    
    <project xmlns="http://maven.apache.org/POM/4.0.0"
            xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
            xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
        <modelVersion>4.0.0</modelVersion>
    
        <groupId>com.yuanrong.demo</groupId>
        <artifactId>yrdemo</artifactId>
        <version>1.0.0</version>
    
        <properties>
            <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
            <maven.compiler.source>1.8</maven.compiler.source>
            <maven.compiler.target>1.8</maven.compiler.target>
        </properties>
    
        <dependencies>
            <dependency>
                <!-- Modify version number to your actual version -->
                <groupId>org.yuanrong</groupId>
                <artifactId>yr-api-sdk</artifactId>
                <version>9.9.9</version>
            </dependency>
            <dependency>
                <!-- Modify version number to your actual version -->
                <groupId>org.yuanrong</groupId>
                <artifactId>faas-function-sdk</artifactId>
                <version>9.9.9</version>
            </dependency>
        </dependencies>
    
        <build>
            <plugins>
                <plugin>
                    <groupId>org.apache.maven.plugins</groupId>
                    <artifactId>maven-assembly-plugin</artifactId>
                    <executions>
                        <execution>
                            <phase>package</phase>
                            <goals>
                                <goal>single</goal>
                            </goals>
                        </execution>
                    </executions>
                    <configuration>
                        <archive>
                            <manifest>
                                <mainClass>com.yuanrong.demo.Main</mainClass>
                            </manifest>
                        </archive>
                        <descriptorRefs>
                            <descriptorRef>jar-with-dependencies</descriptorRef>
                        </descriptorRefs>
                        <appendAssemblyId>false</appendAssemblyId>
                    </configuration>
                </plugin>
            </plugins>
        </build>
    </project>
    ```

    :::
    :::{dropdown} calculator.cpp file content
    :chevron: down-up
    :icon: chevron-down

    ```cpp
    #include "yr/yr.h"
    int Square(int x)
    {
        return x * x;
    }

    // Define stateless function Square
    YR_INVOKE(Square)

    class Counter {
    public:
        Counter() {}
        Counter(int init) { count = init; }

        static Counter *FactoryCreate(int init) {
            return new Counter(init);
        }

        void Add() {
            count += 1;
        }

        int Get() {
            return count;
        }

        YR_STATE(count);

    public:
        int count;
    };

    // Define stateful function Counter
    YR_INVOKE(Counter::FactoryCreate, &Counter::Add, &Counter::Get);
    ```

    :::
    :::{dropdown} CMakeLists.txt file content, **need to modify YR_INSTALL_PATH to your openYuanrong installation path**
    :chevron: down-up
    :icon: chevron-down

    ```cmake
    cmake_minimum_required(VERSION 3.16.1)
    project(calculator CXX)

    set(CMAKE_CXX_STANDARD 17)
    set(SOURCE_DIR ${CMAKE_CURRENT_SOURCE_DIR})
    set(BINARY_DIR ${SOURCE_DIR}/build)
    set(BUILD_SHARED_LIBS ON)

    # Replace YR_INSTALL_PATH value with actual openYuanrong installation path
    set(YR_INSTALL_PATH "/usr/local/lib/python3.9/site-packages/yr")
    link_directories(${YR_INSTALL_PATH}/cpp/lib)
    include_directories(
        ${YR_INSTALL_PATH}/cpp/include
    )

    add_library(calculator SHARED calculator.cpp)
    set(CMAKE_RUNTIME_OUTPUT_DIRECTORY ${BINARY_DIR})
    ```

    :::

2. Build C++ functions into dynamic library

    In the `java-invoke-cpp/cpp-project/build` directory, execute the following command to build:

    ```bash
    cmake ..
    make
    ```

    Successful build will generate dynamic library file `libcalculator.so` in this directory. Copy the file to the code package directory on all nodes, for example: `/opt/mycode/java-invoke-cpp`.

3. Register C++ functions as openYuanrong functions

    Use curl tool to call the [Register Function API](../api/function_service/register_function.md) to register an openYuanrong function named `0-yr-mycpp`, with corresponding function URN `sn:cn:yrk:default:function:0-yr-mycpp:$latest`.

    ```bash
    # Replace /opt/mycode/java-invoke-cpp with your code package directory
    META_SERVICE_ENDPOINT=<meta service component endpoint, default: http://{master node IP}:31182>
    curl -H "Content-type: application/json" -X POST -i ${META_SERVICE_ENDPOINT}/serverless/v1/functions -d '{"name":"0-yr-mycpp","runtime":"cpp11","kind":"yrlib","cpu":500,"memory":500,"timeout":60,"storageType":"local","codePath":"/opt/mycode/java-invoke-cpp"}'
    ```

4. Run Java main program for testing

    In the `java-invoke-cpp/java-project` directory, execute the following command to build and package:

    ```bash
    mvn clean package
    ```

    Successful build will generate `yrdemo-1.0.0.jar` file in the `java-invoke-cpp/java-project/target` directory. Execute the following command to run the main program.

    ```bash
    java -jar yrdemo-1.0.0.jar
    # Outputs 4 and 2
    ```
