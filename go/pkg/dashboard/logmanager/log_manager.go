/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
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

package logmanager

import (
	"context"
	"strings"
	"sync"

	"yuanrong.org/kernel/pkg/common/faas_common/grpc/pb/logservice"
	"yuanrong.org/kernel/pkg/common/faas_common/logger/log"
	"yuanrong.org/kernel/pkg/common/faas_common/types"
	"yuanrong.org/kernel/pkg/dashboard/etcdcache"
)

const userStdStreamPrefix = "/log/runtime/std/"

type manager struct {
	Collectors   map[string]collectorClient
	pendingItems chan *logservice.LogItem
	LogDB

	mtx sync.RWMutex // TO protect Collectors
}

var (
	managerSingleton *manager
	logManagerOnce   sync.Once
)

func streamTopicOfInstanceJobID(instance *types.InstanceSpecification) string {
	return userStdStreamPrefix + instance.JobID
}

func instanceJobIDOfStreamTopic(topic string) string {
	if !strings.HasPrefix(topic, userStdStreamPrefix) {
		return ""
	}
	return strings.TrimPrefix(topic, userStdStreamPrefix)
}

func init() {
	logManagerOnce.Do(func() {
		managerSingleton = &manager{
			Collectors: map[string]collectorClient{},
			LogDB:      newGeneralLogDBImpl(),
		}
		etcdcache.InstanceCache.RegisterInstanceStartHandler(managerSingleton.OnInstanceStart)
		etcdcache.InstanceCache.RegisterInstanceExitHandler(managerSingleton.OnInstanceExit)
	})
}

// RegisterLogCollector -
func (m *manager) RegisterLogCollector(collectorInfo collectorClientInfo) error {
	client := collectorClient{
		collectorClientInfo: collectorInfo,
	}
	err := client.Connect()
	if err != nil {
		return err
	}
	go client.Healthcheck(func() {
		// unregister self if shutdown
		m.UnregisterLogCollector(collectorInfo.ID)
	})
	go m.recycleLogStream(&client)
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.Collectors[collectorInfo.ID] = client
	return nil
}

func (m *manager) recycleLogStream(client *collectorClient) {
	queryRsp, err := client.logClient.QueryLogStream(context.TODO(), &logservice.QueryLogStreamRequest{})
	if err != nil {
		log.GetLogger().Errorf("failed to query log stream of client %s at %s", client.ID, client.Address)
		return
	}
	for _, streamName := range queryRsp.Streams {
		jobID := instanceJobIDOfStreamTopic(streamName)
		driverExists := false
		for _, instance := range etcdcache.InstanceCache.GetByJobID(jobID) {
			if isDriverInstance(instance) {
				driverExists = true
				break
			}
		}
		if driverExists {
			continue
		}
		// otherwise, should recycle the stream
		log.GetLogger().Infof("going to recycle log stream %s at collector %s", streamName, client.ID)
		stopRsp, err := client.logClient.StopLogStream(context.TODO(),
			&logservice.StopLogStreamRequest{StreamName: streamName})
		if err != nil || stopRsp.Code != 0 {
			log.GetLogger().Errorf("failed to recycle log stream %s at collector %s", streamName, client.ID)
		}
	}
}

// UnregisterLogCollector -
func (m *manager) UnregisterLogCollector(id string) {
	m.mtx.RLock()
	c, ok := m.Collectors[id]
	m.mtx.RUnlock()
	if !ok {
		return
	}

	m.mtx.Lock()
	delete(m.Collectors, id)
	m.mtx.Unlock()

	if err := c.grpcConn.Close(); err != nil {
		log.GetLogger().Warnf("failed to close connection to collector %s at %s: %s", id, c.Address, err.Error())
	}
}

// GetCollector -
func (m *manager) GetCollector(id string) *collectorClient {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	if c, ok := m.Collectors[id]; ok {
		return &c
	}
	return nil
}

// OnInstanceStart handles event when instance is running
func (m *manager) OnInstanceStart(instance *types.InstanceSpecification) {
	log.GetLogger().Infof("running instance %s started cb", instance.InstanceID)
	if isDriverInstance(instance) {
		// do nothing to driver
		log.GetLogger().Debugf("skip driver instance event of %s", instance.InstanceID)
		return
	}
	// then try to fulfill the log entry
	m.fulfillLogEntryInLogDB(instance)
}

// ReportLogItem handles the report request
func (m *manager) ReportLogItem(item *logservice.LogItem) {
	// match and check the component id (runtime id)
	m.LogDB.Put(NewLogEntry(item), putOptionIfExistsNoop)
	if item.RuntimeID != "" {
		// if not empty, this is a runtime, try match the runtime and try to fulfill the log entry
		m.fulfillLogEntryInLogDB(etcdcache.InstanceCache.GetByRuntimeID(item.RuntimeID))
	}
}

func (m *manager) fulfillLogEntryInLogDB(instance *types.InstanceSpecification) {
	if instance == nil {
		log.GetLogger().Debugf("try fulfill a log entry with nil instance ptr")
		return
	}
	result := m.LogDB.Query(logDBQuery{RuntimeID: instance.RuntimeID})
	result.Range(func(entry *LogEntry) {
		if entry.InstanceID == instance.InstanceID {
			log.GetLogger().Debugf("entry of instance(%s) with filename(%s) on collector(%s) is already been set, "+
				"no update need", instance.InstanceID, entry.Filename, entry.CollectorID)
			return
		}
		entry.InstanceID = instance.InstanceID
		entry.JobID = instance.JobID
		// actually performs an in-place modification, but still put it to avoid some unexpected problem
		m.LogDB.Put(entry, putOptionIfExistsReplace)
		go m.processLogStream(instance)
	})
}

func (m *manager) processLogStream(instance *types.InstanceSpecification) {
	// query for the first time
	queryResult := m.LogDB.Query(logDBQuery{InstanceID: instance.InstanceID})
	if queryResult.Len() <= 0 {
		log.GetLogger().Infof("failed to detect instance %s log data in log db, the log stream will not start",
			instance.InstanceID)
		return
	}
	queryResult.Range(func(inst *LogEntry) {
		// this is a pointer, so it is not copied, but it seems that in-place modify is ok
		inst.LogItem.Target = logservice.LogTarget_USER_STD
		streamTopic := streamTopicOfInstanceJobID(instance)
		log.GetLogger().Infof("notify collector to start a stream about job %s on item %s", streamTopic, inst.LogItem)
		log.GetLogger().Infof("collector: %#v", m.GetCollector(inst.CollectorID))
		_, err := m.GetCollector(inst.CollectorID).logClient.StartLogStream(context.TODO(),
			&logservice.StartLogStreamRequest{StreamName: streamTopic, Item: inst.LogItem})
		if err != nil {
			log.GetLogger().Warnf("notify collector %s on stream %s about %s failed: %s", inst.CollectorID,
				streamTopic, inst.LogItem.Filename, err)
			return
		}
	})
}

// OnInstanceExit -
func (m *manager) OnInstanceExit(instance *types.InstanceSpecification) {
	if !isDriverInstance(instance) {
		return
	}
	// only stop all stream when driver exit
	log.GetLogger().Infof("instance %s exited, stop log stream of %s now", instance.InstanceID, instance.JobID)
	for _, c := range m.Collectors {
		_, err := c.logClient.StopLogStream(context.TODO(),
			&logservice.StopLogStreamRequest{StreamName: streamTopicOfInstanceJobID(instance)})
		if err != nil {
			log.GetLogger().Errorf("failed to stop stream, err: %v", err)
			continue
		}
	}
}

func isDriverInstance(instance *types.InstanceSpecification) bool {
	return strings.HasPrefix(instance.InstanceID, "driver") && instance.ParentID == ""
}

// RemoveLogItem handles the remove log request
func (m *manager) RemoveLogItem(item *logservice.LogItem) {
	m.LogDB.Remove(NewLogEntry(item))
}
