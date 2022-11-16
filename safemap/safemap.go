package safemap

import (
	"sync"
	"time"
)

type (
	SafeGcMap interface {
		Get(interface{}) (interface{}, bool)
		Set(interface{}, interface{})
		Del(interface{})
		Size() int
	}

	// AutoGcMap 因为 golang 设计 map 为了复用 key, 删除不会真正的删除, 只是软删除
	AutoGcMap struct {
		lock         sync.RWMutex
		m            map[interface{}]interface{} // 用来存储数据的
		mBck         map[interface{}]interface{} // 触发阈值, m 与 mBck 调换
		set          map[interface{}]struct{}
		mThreshold   int  // 阈值
		firstMigrate bool // 是否第一次迁移
		migrating    bool // 是否正在迁移中
	}
)

func (m *AutoGcMap) Size() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if m.mBck != nil {
		return len(m.m) + len(m.mBck)
	}
	return len(m.m)
}

func (m *AutoGcMap) Get(i interface{}) (v interface{}, exists bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if m.mBck != nil {
		v, exists = m.mBck[i]
		if exists {
			return
		}
	}
	v, exists = m.m[i]
	return
}

func (m *AutoGcMap) Set(i interface{}, v interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()
	// 在每一次操作的时候迁移两个数据, 避免一次性全部迁移导致比较大的抖动
	m.dataMigrate()
	m.m[i] = v
	m.set[i] = struct{}{}
	m.swapMap()
}

func (m *AutoGcMap) Del(i interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.dataMigrate()
	if m.mBck != nil {
		delete(m.mBck, i)
	}
	delete(m.m, i)
	m.swapMap()
}

// swapMap 交换两个map, 并重置set
func (m *AutoGcMap) swapMap() {
	mSetLength := len(m.set)
	mMLength := len(m.m)
	// 假如说map一直在往里面存数据，但是并没有做删除操作，
	// 那么就算达到阈值了，重新分配也不会释放内存，反而在转换的时候消耗了CPU
	if mSetLength > m.mThreshold && mMLength <= mSetLength/2 {
		m.set = make(map[interface{}]struct{}, mMLength)
		m.mBck, m.m = m.m, make(map[interface{}]interface{}, m.mThreshold)
		if m.firstMigrate {
			m.firstMigrate = false
			m.migrating = true
			go m.backendMigrate()
			return
		}
		if !m.migrating {
			m.migrating = true
			// 启动一个 goroutine 在后台进行数据迁移
			go m.backendMigrate()
		}
	}
}

// backendMigrate 后台迁移数据
func (m *AutoGcMap) backendMigrate() {
	for {
		m.lock.Lock()
		m.dataMigrate()
		if m.mBck == nil {
			m.migrating = false
			m.lock.Unlock()
			break
		}
		m.lock.Unlock()
		time.Sleep(time.Millisecond * 100)
	}
}

// transferData 数据迁移, 每次迁移两个, 均摊到每次操作中
func (m *AutoGcMap) dataMigrate() {
	if m.mBck != nil {
		var index int
		for k, v := range m.mBck {
			index++
			if index > 2 {
				break
			}
			m.m[k] = v
			m.set[k] = struct{}{}
			delete(m.mBck, k)
		}
		if len(m.mBck) == 0 {
			m.mBck = nil
		}
	}
}

func NewAutoGcMap(length, mThreshold int) *AutoGcMap {
	return &AutoGcMap{
		m:            make(map[interface{}]interface{}, length),
		mThreshold:   mThreshold,
		set:          make(map[interface{}]struct{}, mThreshold),
		firstMigrate: true,
	}
}
