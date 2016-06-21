package mq

import (
	_ "gitserver/iop/DockerStack/common"
	"sync"
)

func GetConnection(connection_type string) *BaseMq {

	switch connection_type {
	case "manager":
		return getManagerMq()
	case "monitor":
		return getMonitorMq()
	default:
		panic("无效运算符号")
		return nil
	}
}

var managerMq *BaseMq
var managerMqonce sync.Once
var monitorMq *BaseMq
var monitorMqonce sync.Once

func getManagerMq() *BaseMq {
	managerMqonce.Do(func() {
		managerMq = &BaseMq{
			MqConnection: &MqConnection{MqUri: "amqp://guest:guest@127.0.0.1:5672/"},
		}
		managerMq.Init()
	})
	return managerMq
}

func getMonitorMq() *BaseMq {
	monitorMqonce.Do(func() {
		monitorMq = &BaseMq{
			MqConnection: &MqConnection{MqUri: "amqp://guest:guest@127.0.0.1:5672/"},
		}
		monitorMq.Init()
	})
	return monitorMq
}
