package zklock


import (
	"fmt"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/Loopring/relay-lib/utils"
)

type Task struct {
	Path string
	Payload  string
	Weight   int
	Status   string
	Owner    string
	NewOwner string
}

const (
	normal = iota
	changing
	deleting
)

const balancerShareBasePath = "/loopring_balancer"
const workerPath = "worker"
const taskPath = "task"

type ZkBalancer struct {
	Name string
	WorkerPath string
	TaskPath string
	Tasks []Task
}

func (zb *ZkBalancer) Init(tasks []Task) error {
	if zb.Name == "" {
		return fmt.Errorf("balancer Name is empty")
	}
	if len(zb.Tasks) == 0 {
		return fmt.Errorf("no tasks to balance")
	}
	if !IsInit() {
		return fmt.Errorf("zkClient is not intiliazed")
	}
	var err error
	if zb.WorkerPath, err = zb.createSubPath(workerPath); err != nil {
		return err
	}
	if zb.TaskPath, err = zb.createSubPath(taskPath); err != nil {
		return err
	}
	return nil
}

func (zb *ZkBalancer) Start() {
	zb.startMaster()
	zb.registerWorker()
}

func (zb *ZkBalancer) Stop() {
	ReleaseLock(zb.masterLockName())
	zb.unRegisterWorker()
}

func (zb *ZkBalancer) OnAssign(assignFunc func (tasks []Task) error) {
	assignFunc([]Task{})
}

func (zb *ZkBalancer) OnRelease(needReleaseFunc func (tasks []Task) error) {
	needReleaseFunc([]Task{})
}

func (zb *ZkBalancer) Released(task Task) error {
	return nil
}

func (zb *ZkBalancer) startMaster() {
	go func() {
		TryLock(zb.masterLockName())
		zb.validTasks()
	}()
}

func (zb *ZkBalancer) validTasks() {
	//
}

func (zb *ZkBalancer) registerWorker() error {
	if _, err := ZkClient.Create(zb.workerEphemeral(), nil, zk.FlagEphemeral, zk.WorldACL(zk.PermAll)); err != nil {
		return nil
	} else {
		return nil
	}
}

func (zb *ZkBalancer) unRegisterWorker() {
	if _, stat, err := ZkClient.Get(zb.workerEphemeral()); err == nil {
		ZkClient.Delete(zb.workerEphemeral(), stat.Version)
	} else {

	}
}

func (zb *ZkBalancer) monitorWorkers() {

}

func (zb *ZkBalancer) createSubPath(subPath string) (string, error) {
	subPathFull := fmt.Sprintf("%s/%s/%s", balancerShareBasePath, zb.Name, subPath)
	isExist, _, err := ZkClient.Exists(subPathFull)
	if err != nil {
		return "", fmt.Errorf("check zk path %s exists failed : %s", subPathFull, err.Error())
	}
	if !isExist {
		_, err := ZkClient.Create(subPathFull, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists {
			return "", fmt.Errorf("failed create balancer sub path %s", subPathFull)
		}
	}
	return subPathFull, nil
}

func (zb *ZkBalancer) masterLockName() string {
	return fmt.Sprintf("balancer_%s", zb.Name)
}

func (zb *ZkBalancer) workerEphemeral() string {
	return fmt.Sprintf("%s/%s", zb.WorkerPath, utils.GetLocalIp())
}