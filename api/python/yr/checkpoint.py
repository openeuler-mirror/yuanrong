#!/usr/bin/env python3
# coding=UTF-8

from dataclasses import dataclass, field


@dataclass
class SnapstartInfo:
    route_address: str = ""
    port_mappings: str = ""
    function_proxy_id: str = ""
    node_id: str = ""
    namespace: str = ""


@dataclass
class SnapstartResponse:
    instance_id: str = ""
    snapstart_info: SnapstartInfo = field(default_factory=SnapstartInfo)
