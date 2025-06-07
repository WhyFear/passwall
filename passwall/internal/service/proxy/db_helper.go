package proxy

import (
	"sync"
)

// 全局数据库互斥锁，用于避免SQLite的"database is locked"错误
var dbMutex sync.Mutex

// SafeDBOperation 使用互斥锁保护的数据库操作
// 接收一个函数作为参数，在互斥锁保护下执行该函数
// 可以在所有需要数据库操作的地方使用，避免并发写入导致的锁定问题
func SafeDBOperation(operation func() error) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	return operation()
}
