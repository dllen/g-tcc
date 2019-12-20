# 迷你 TCC (Try - Confirm - Cancel) 的实现

### Exponential backoff

[指数补偿（Exponential backoff）在网络请求中的应用][https://zhuanlan.zhihu.com/p/37729147]

### TCC

一个分布式的全局事务，整体是 两阶段提交 的模型。全局事务是由若干分支事务组成的，分支事务要满足 两阶段提交 的模型要求，即需要每个分支事务都具备自己的：

1.一阶段 prepare 行为
2.二阶段 commit 或 rollback 行为

![http://seata.io/img/overview-4.png][tcc]

根据两阶段行为模式的不同，我们将分支事务划分为 Automatic (Branch) Transaction Mode 和 Manual (Branch) Transaction Mode.

AT 模式 基于 支持本地 ACID 事务 的 关系型数据库：

    一阶段 prepare 行为：在本地事务中，一并提交业务数据更新和相应回滚日志记录。
    二阶段 commit 行为：马上成功结束，自动 异步批量清理回滚日志。
    二阶段 rollback 行为：通过回滚日志，自动 生成补偿操作，完成数据回滚。

相应的，TCC 模式，不依赖于底层数据资源的事务支持：

    一阶段 prepare 行为：调用 自定义 的 prepare 逻辑。
    二阶段 commit 行为：调用 自定义 的 commit 逻辑。
    二阶段 rollback 行为：调用 自定义 的 rollback 逻辑。
所谓 TCC 模式，是指支持把 自定义 的分支事务纳入到全局事务的管理中。

### Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/dllen/g-tcc"
)

var (
	db = &FakeDB{
		flight: flight{StockSeatCount: uint64(3)},
		hotel:  hotel{StockRoomCount: uint64(1)},
	}

	flightService = tcc.NewService(
		"flight reservation",
		db.tryReserveFlightSeat,
		db.confirmFlightSeatReservation,
		db.cancelFlightSeat,
	)

	hotelService = tcc.NewService(
		"hotel reservation",
		db.tryReserveHotelRoom,
		db.confirmHotelRoomReservation,
		db.cancelHotelRoom,
	)
)

type flight struct {
	StockSeatCount    uint64
	ReservedSeatCount uint64
}

type hotel struct {
	StockRoomCount    uint64
	ReservedRoomCount uint64
}

// FakeDB represents a database for example
type FakeDB struct {
	flight flight
	hotel  hotel
}

func (f *FakeDB) tryReserveFlightSeat() error {
	if f.flight.StockSeatCount == 0 {
		return fmt.Errorf("no seat")
	}
	f.flight.StockSeatCount--
	return nil
}

func (f *FakeDB) confirmFlightSeatReservation() error {
	f.flight.ReservedSeatCount++
	return nil
}

func (f *FakeDB) cancelFlightSeat() error {
	f.flight.StockSeatCount++
	return nil
}

func (f *FakeDB) tryReserveHotelRoom() error {
	if f.hotel.StockRoomCount == 0 {
		return fmt.Errorf("no room")
	}
	f.hotel.StockRoomCount--
	return nil
}

func (f *FakeDB) confirmHotelRoomReservation() error {
	f.hotel.ReservedRoomCount++
	return nil
}

func (f *FakeDB) cancelHotelRoom() error {
	f.hotel.StockRoomCount++
	return nil
}

func main() {
	doFirstReservation(db)
	doSecondReservation(db)
}

func doFirstReservation(db *FakeDB) {
	orchestrator := tcc.NewDirector([]*tcc.Service{flightService, hotelService}, tcc.WithMaxRetries(1))
	err := orchestrator.Direct()
	if err != nil {
		log.Printf("error happened in 1st reservation: %s", err)
	}
}

func doSecondReservation(db *FakeDB) {
	// In second reservation, flight seat is not enough
	orchestrator := tcc.NewDirector([]*tcc.Service{flightService, hotelService}, tcc.WithMaxRetries(1))
	err := orchestrator.Direct()
	if err != nil {
		log.Printf("error happened in 2nd reservation: %s", err)
	}
	tccErr := err.(*tcc.Error)
	log.Printf("tccErr.Error: %v", tccErr.Error())
	log.Printf("tccErr.FailedPhase == ErrTryFailed: %v", tccErr.FailedPhase() == tcc.ErrTryFailed)
	log.Printf("tccErr.ServiceName: %v", tccErr.ServiceName())
}
```

### Documents

[GoDoc](https://godoc.org/github.com/dllen/g-tcc).

### 参考文档

[Seata](http://seata.io/zh-cn/docs/overview/what-is-seata.html)
[Eventual Data Consistency Solution in ServiceComb - part 3](https://servicecomb.apache.org/docs/distributed_saga_3/)
[Transactions for the REST of Us](https://dzone.com/articles/transactions-for-the-rest-of-us)

### License

MIT
