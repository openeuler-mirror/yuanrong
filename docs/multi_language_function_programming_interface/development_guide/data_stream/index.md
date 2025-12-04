# 数据流

openYuanrong 提供了基于发布-订阅（pub/sub）模型的数据流，可实现函数间无界数据流的数据交换，支持复杂的数据交互关系。同时，数据流解耦了数据生产者和消费者，支持它们各自按需异步调度。

数据流中有四个关键概念：生产者（producer）、消费者（consumer）、流（stream）、数据项（element）。

- 生产者：生产者是无界数据流的发起端，产生并发送数据。
- 消费者：消费者是无界数据流的接收端，消费数据。
- 数据项：数据以数据项的粒度在生产者和消费者间发送和接收。
- 流：生产者和消费者间不相互感知，通过流进行关联。生产者向流中发送数据，消费者订阅流并从流中接收数据。一个应用中可以有多条流，通过流名称区分。

数据流支持多生产者和多消费者间的数据交互。在实际场景中，使用最多的是一个生产者一个消费者（one-to-one）、多个生产者一个消费者（many-to-one）及一个生产者多个消费者（one-to-many）。

## 使用限制

- 数据流无持久化能力，发送节点故障时，数据会丢失，应用需要考虑对应的故障处，比如重启业务等。
- 数据流都是同步接口，没有类似 epoll 的多路复用能力。
- 流解耦了生产者和消费者，彼此间不感知对方。当生产者关闭时，消费者无法感知，需要业务层进行处理。

## 创建数据流

流代表了生产者和消费者间的发布订阅交互关系。流随着生产者或消费者的创建而隐式创建，无需应用显式创建流。

创建一个生产者或消费者时，需要指定它所关联的流，不同的流通过流名称区分。如果流已经存在，新创建的生产者或消费者会关联到这条流上。如流不存在，在创建生产者或消费者时，系统会隐式创建一个新流，并关联它到指定的流名称。

一条流上的生产者和消费者都关闭后，如果未操作接口删除流，流仍然存在，应用可以在这条流上继续关联新的生产者和消费者。您可以在创建生产者时指定流为自动删除（配置 `autoCleanup` 选项），当该流上所有生产者和消费者都关闭后，流将被系统自动删除。

:::::{tab-set}
::::{tab-item} Python

```python
import yr

stream_name = "this-stream"
try:
    # 配置流自动删除
    producer_config = yr.ProducerConfig(delay_flush_time=5, page_size=1024 * 1024, max_stream_size=1024 * 1024 * 1024, auto_clean_up=True)
    # 创建生产者将隐式创建流 this-stream
    producer = yr.create_stream_producer(stream_name, producer_config)
    # 关闭生产者，流 this-stream 已无生产者或消费者关联，将被自动删除
    producer.close()

    config = yr.SubscriptionConfig("local-consumer")
    # 创建消费者将再次隐私创建流 this-stream
    consumer = yr.create_stream_consumer(stream_name, config)
    consumer.close()

    # 消费者新创建的流需要显示删除
    yr.delete_stream(stream_name)
except RuntimeError as exp:
    # 处理异常
```

::::
::::{tab-item} C++

```cpp
#include "yr/yr.h"

int main(int argc, char *argv[])
{
    std::string streamName = "this-stream";
    try {
        // 配置流自动删除
        YR::ProducerConf producerConf{.delayFlushTime=5, .pageSize=1024 * 1024ul, .maxStreamSize=1024 * 1024 * 1024ul, .autoCleanup=true};
        // 创建生产者将隐式创建流 this-stream
        std::shared_ptr<YR::Producer> producer = YR::CreateProducer(streamName, producerConf);
        // 关闭生产者，流 this-stream 已无生产者或消费者关联，将被自动删除
        producer->Close();
    
        YR::SubscriptionConfig config("local-consumer", YR::SubscriptionType::STREAM);
        // 创建消费者将再次隐私创建流 this-stream
        std::shared_ptr<YR::Consumer> consumer = YR::Subscribe("streamName", config, true);
        consumer->Close();

        // 消费者新创建的流需要显示删除
        YR::DeleteStream(streamName);
    } catch (YR::Exception &e) {
        // 处理异常
    }
}
```

::::
::::{tab-item} Java

```java
package com.example;

import com.yuanrong.stream.Producer;
import com.yuanrong.stream.ProducerConfig;
import com.yuanrong.stream.Consumer;
import com.yuanrong.stream.SubscriptionConfig;
import com.yuanrong.stream.SubscriptionType;
import com.yuanrong.stream.Element;

public class Main {
    public static void main(String[] args) {
        String streamName = "this-stream";
        try {
            // 配置流自动删除
            ProducerConfig pCfg = ProducerConfig.builder().delayFlushTimeMs(5L).pageSizeByte(1024 * 1024L).maxStreamSize(1024 * 1024 * 1024L).autoCleanup(true).build();
            // 创建生产者将隐式创建流 this-stream
            Producer producer = YR.createProducer(streamName, pCfg);
            // 关闭生产者，流 this-stream 已无生产者或消费者关联，将被自动删除
            producer.close();

            SubscriptionConfig config = SubscriptionConfig.builder().subscriptionName("local_consumer").build();
            // 创建消费者将再次隐私创建流 this-stream
            Consumer consumer = YR.subscribe(streamName, config);

            // 消费者新创建的流需要显示删除
            YR.deleteStream(streamName);
        } catch (YRException e) {
            // handle exception
        }
    }
}
```

::::
:::::

## 生产数据

生产者（Producer）可向流中发送数据。生产者发送的数据会先放入缓冲区，系统根据生产者配置的 Flush 策略（发送间隔一段时间或者缓冲写满）刷新缓冲使其对消费者可见。生产者不再使用时，需要主动关闭。

:::::{tab-set}
::::{tab-item} Python

```python
import yr

stream_name = "this-stream"
try:
    producer_config = yr.ProducerConfig(delay_flush_time=5, page_size=1024 * 1024, max_stream_size=1024 * 1024 * 1024, auto_clean_up=True)
    producer = yr.create_stream_producer(stream_name, producer_config)
    # 生产一条数据
    element = yr.Element(data=b"hello", id=0)
    producer.send(element)

    # 关闭生产者
    producer.close()
except RuntimeError as exp:
    # 处理异常
```

::::
::::{tab-item} C++

```cpp
#include "yr/yr.h"

int main(int argc, char *argv[])
{
    std::string streamName = "this-stream";
    try {
        YR::ProducerConf producerConf{.delayFlushTime=5, .pageSize=1024 * 1024ul, .maxStreamSize=1024 * 1024 * 1024ul, .autoCleanup=true};
        std::shared_ptr<YR::Producer> producer = YR::CreateProducer(streamName, producerConf);
        
        // 生产一条数据
        std::string str = "hello";
        YR::Element element((uint8_t *)(str.c_str()), str.size());
        producer->send(element);

        producer->Close();
    } catch (YR::Exception &e) {
        // 处理异常
    }
}
```

::::
::::{tab-item} Java

```java
package com.example;

import com.yuanrong.stream.Producer;
import com.yuanrong.stream.ProducerConfig;
import com.yuanrong.stream.Consumer;
import com.yuanrong.stream.SubscriptionConfig;
import com.yuanrong.stream.SubscriptionType;
import com.yuanrong.stream.Element;

public class Main {
    public static void main(String[] args) {
        String streamName = "this-stream";
        try {
            ProducerConfig pCfg = ProducerConfig.builder().delayFlushTimeMs(5L).pageSizeByte(1024 * 1024L).maxStreamSize(1024 * 1024 * 1024L).autoCleanup(true).build();
            Producer producer = YR.createProducer(streamName, pCfg);

            // 生产一条数据
            String data = "hello";
            ByteBuffer buffer = ByteBuffer.wrap(data.getBytes());
            Element element = new Element(0L, buffer);
            producer.send(element);

            // 关闭生产者
            producer.close();
        } catch (YRException e) {
            // handle exception
        }
    }
}
```

::::
:::::

## 消费数据

消费者（Consumer）可接收流中的数据，使用 `Ack` 方法确认数据接收。消费者不再使用时，需要主动关闭。

消费者接收数据后，需要对数据进行 ACK 操作，以确认该数据及之前收到的数据都已消费完。对确认消费完的数据，系统会回收内存资源。数据流提供了自动 ACK 功能，只需在创建消费者时配置 `autoAck` 为 `true`。应用每次调用 `Receive` 操作后，系统会自动确认上一次接收的数据，应用无需再主动调用 `Ack` 方法。

:::{note}

开启自动 ACK 后，用户需保证消费者每次调用 `Receive` 前，上一次接收到的数据已消费完。调用 `Receive` 后，继续消费上一次接收到的数据系统未定义。

:::

:::{hint}

当消费者调用 `Receive` 方法时，会获取 `Element` 对象，对象的内部指针指向实际的数据，这些数据处于应用函数和数据系统间的共享内存之中。应用需要通过 ACK 操作确认该数据已经消费完，此时数据系统才能回收该数据所占的内存资源。如应用不调用 ACK 操作，数据系统无法判断数据是否被消费，则无法回收内存资源，最终导致内存资源耗尽，系统异常。

:::

:::::{tab-set}
::::{tab-item} Python

```python
import yr

stream_name = "this-stream"
try:
    producer_config = yr.ProducerConfig(delay_flush_time=5, page_size=1024 * 1024, max_stream_size=1024 * 1024 * 1024, auto_clean_up=True)
    producer = yr.create_stream_producer(stream_name, producer_config)

    config = yr.SubscriptionConfig("local_consumer")
    consumer = yr.create_stream_consumer(stream_name, config)

    element_in = yr.Element(data=b"hello", id=0)
    producer.send(data_in)
    # 等待到一条数据或者1秒超时
    element_out = consumer.receive(1000, 1)
    print("receive:" + element_out.data.decode())

    producer.close()
    consumer.close()
except RuntimeError as exp:
    # 处理异常
```

::::
::::{tab-item} C++

```cpp
#include "yr/yr.h"

int main(int argc, char *argv[])
{
    std::string streamName = "this-stream";
    try {
        YR::ProducerConf producerConf{.delayFlushTime=5, .pageSize=1024 * 1024ul, .maxStreamSize=1024 * 1024 * 1024ul, .autoCleanup=true};
        std::shared_ptr<YR::Producer> producer = YR::CreateProducer(streamName, producerConf);

        YR::SubscriptionConfig config("local-consumer", YR::SubscriptionType::STREAM);
        std::shared_ptr<YR::Consumer> consumer = YR::Subscribe("streamName", config, true);

        // 生产一条数据
        std::string str = "hello";
        YR::Element element((uint8_t *)(str.c_str()), str.size());
        producer->send(element);

        std::vector<YR::Element> elements;
        consumer->Receive(1, 6000, elements);

        producer->Close();
        consumer->Close();
    } catch (YR::Exception &e) {
        // 处理异常
    }
}
```

::::
::::{tab-item} Java

```java
package com.example;

import com.yuanrong.stream.Producer;
import com.yuanrong.stream.ProducerConfig;
import com.yuanrong.stream.Consumer;
import com.yuanrong.stream.SubscriptionConfig;
import com.yuanrong.stream.SubscriptionType;
import com.yuanrong.stream.Element;

public class Main {
    public static void main(String[] args) {
        String streamName = "this-stream";
        try {
            ProducerConfig pCfg = ProducerConfig.builder().delayFlushTimeMs(5L).pageSizeByte(1024 * 1024L).maxStreamSize(1024 * 1024 * 1024L).autoCleanup(true).build();
            Producer producer = YR.createProducer(streamName, pCfg);

            // 生产一条数据
            String data = "hello";
            ByteBuffer buffer = ByteBuffer.wrap(data.getBytes());
            Element element = new Element(0L, buffer);
            producer.send(element);

            List<Element> recv = consumer.receive(3, 6000);
            if (recv.isEmpty()) {
                // handle empty.
            }
            Element e = recv.get(0);
            Charset charset = Charset.forName("UTF-8");
            String res = charset.decode(e.getBuffer()).toString();
            consumer.ack(e.getId());

            producer.close();
            consumer.close();
        } catch (YRException e) {
            // handle exception
        }
    }
}
```

::::
:::::

## 数据项

数据流以数据项为单位发送数据，数据项结构体如下:

| **字段** | **类型**  | **说明**       |
| -------- | --------- | -------------- |
| **ptr**  | uint8_t * | 指向数据的指针。 |
| **size** | unit64_t  | 数据长度。       |
| **id**   | uint64_t  | 数据项的 id。 |

`ptr` 字段指向数据的存放地址。在生产者端, `ptr` 字段指向应用的内存空间，因为发送的数据是应用填充的。在消费者端, `ptr` 字段指向的地址空间位于应用函数和数据系统间的共享内存中，这也是为什么会需要 ACK 操作的原因。
