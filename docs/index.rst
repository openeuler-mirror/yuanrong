.. title:: 首页

.. raw:: html

    <h2 style="text-align:center">openYuanrong</h2>
    <h6 style="text-align:center">单机编程体验，分布式运行性能</h6>
    
    <hr style="border:none; border-top:1px solid #000; margin:20px 0; width:100%;">
    
    <p style="text-align:left; margin:20px 0; color:#000; line-height:1.6;">
        openYuanrong 是一个 Serverless 分布式计算引擎，致力于以一套统一的 Serverless 架构支持 AI、大数据、微服务等各类分布式应用。它提供多语言函数编程接口，以单机编程体验简化分布式应用开发；提供分布式动态调度和数据共享等能力，实现分布式应用的高性能运行和集群的高效资源利用。
    </p>



.. grid:: 2
    :gutter: 3 3 4 5

    .. grid-item::
        .. button-ref:: overview-getting-started
            :ref-type: ref
            :color: primary
            :align: right
            :shadow:

            快速入门

    .. grid-item::
        .. button-ref:: overview-installation
            :ref-type: ref
            :color: primary
            :shadow:

            安装部署

.. grid:: 3
    :gutter: 3 3 4 5

    .. grid-item-card::

        **了解使用案例**
        ^^^
        openYuanrong 可作为 AI 智算的基础设施，用于开发 AI 推理、强化学习等应用，也可作为通用计算的基础设施，开发大数据分析、HPC 等应用。
        +++
        :doc:`使用案例 <./use_cases/index>`

    .. grid-item-card::

        **开发分布式应用**
        ^^^
        openYuanrong 以函数为开发粒度，提供多语言函数编程接口，快速构建和运行分布式应用。
        +++
        :doc:`多语言函数编程接口 <./multi_language_function_programming_interface/index>`

    .. grid-item-card::

        **部署工作负载**
        ^^^
        openYuanrong 支持在单机上部署和运行，用于学习和开发，生产时可无缝扩展到大型集群。
        +++
        :doc:`部署 openYuanrong <./deploy/index>`

.. raw:: html

    <h2 style="text-align:center">动态</h2>


.. grid:: 2
    :gutter: 3 3 4 5

    .. grid-item-card::

        **博客**
        ^^^
        - `把集群变“单机”（下）——openYuanrong核心架构设计解析 <https://www.openeuler.openatom.cn/zh/blog/20260131-openYuanrong_02/20260131-openYuanrong_02.html>`_
        - `把集群变“单机”（上）——openYuanrong核心技术理念解析 <https://www.openeuler.openatom.cn/zh/blog/20260128-openYuanrong/20260128-openYuanrong.html>`_
        
        `openEuler 官网博客 <https://www.openeuler.openatom.cn/zh/interaction/blog-list/>`_ 标签栏搜索 openYuanrong 查看更多。

    .. grid-item-card::

        **新版本发布**
        ^^^
        v0.7.0 已发布，新增支持 k8s 部署和 spring 微服务兼容，以及分布式调度/缓存等功能。       
        `Release note & Package <https://gitcode.com/openeuler/yuanrong/releases/v0.7.0>`_ 查看更多。

.. raw:: html

    <h2 style="text-align:center">加入 openYuanrong</h2>

.. grid:: 2
    :gutter: 3 3 4 5    
    
    .. grid-item-card::

        **源码仓**
        ^^^^          
        - `yuanrong <https://atomgit.com/openeuler/yuanrong>`_：多语言函数运行时
        - `yuanrong-functionsystem <https://atomgit.com/openeuler/yuanrong-functionsystem>`_：函数系统
        - `yuanrong-datasystem <https://atomgit.com/openeuler/yuanrong-datasystem>`_：数据系统
        
        `openYuanrong 仓库列表 <https://www.openeuler.openatom.cn/zh/sig/sig-YuanRong#repositories>`_ 查看更多。
    
    .. grid-item-card::

        **会议与活动**
        ^^^
        .. grid:: 2
            :gutter: 2

            .. grid-item::
                :columns: 6
                
                参与 openYuanrong SIG 组会议：
                
                - `订阅邮件(yuanrong@openeuler.org) <https://mailweb.openeuler.org/postorius/lists/yuanrong@openeuler.org/>`_
                - `例会信息 <https://www.openeuler.openatom.cn/zh/sig/sig-YuanRong>`_
                

            .. grid-item::
                :columns: 6                
                
                .. image:: ./images/qrcode.png
                    :width: 150px
                    :align: center
        
        
.. toctree::
  :hidden:

  getting_started
  deploy/index.md
  use_cases/index
  multi_language_function_programming_interface/index.md
  more_usage/index.md
  observability/index.md
  FAQ/index.md
  contributor_guide/index.md
  reference/index.md
  security
